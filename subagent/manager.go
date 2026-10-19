package subagent

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"skynet/tools"
)

// Host carries the host server's dependencies into the run manager: the
// subagent tool security policy, the subagent LLM API endpoint, and the
// two log buffers.
type Host struct {
	IsEnabled   func(name string) bool
	LLMAPI      func() string
	HostLog     func(string)
	SubagentLog func(string)
}

// runState is the persisted state of one subagent run, keyed by hash.
type runState struct {
	hash            string
	systemPrompt    string
	prompt          string
	thinking        bool
	haltOnNarration bool
	toolNames       []string
	messages        []Message
	pending         []PendingCall
}

// Manager owns the subagent run registry and implements the subagent
// tools (start / resume / reprompt / inspect / inspect turn).
type Manager struct {
	mu        sync.RWMutex
	runs      map[string]*runState
	hashSeq   atomic.Uint64
	isEnabled func(name string) bool
	api       func() string
	hostLog   func(string)
	subLog    func(string)
}

// NewManager creates a run manager from the host's dependencies.
func NewManager(host Host) *Manager {
	return &Manager{ //nolint:exhaustruct_v5
		runs:      make(map[string]*runState),
		isEnabled: host.IsEnabled,
		api:       host.LLMAPI,
		hostLog:   host.HostLog,
		subLog:    host.SubagentLog,
	}
}

// runFn is one execution of the subagent loop: it receives the run's
// Config and returns the final response, the full conversation, any
// unexecuted tool calls from the terminating step, the last LLM request's
// usage (total context tokens; 0 when unreported), and the run's
// wall-clock duration.
type runFn func(cfg Config) (string, []Message, []PendingCall, int, time.Duration, error)

// execute runs the subagent loop and handles its outcome: the updated
// conversation is persisted, an InterruptedError becomes a resumable
// response (hash + last content), and any other error is returned as a
// standard error result.
func (m *Manager) execute(state *runState, verb, hash string, fn runFn) (any, error) {
	cfg := m.buildConfig(m.collectGrantedTools(m.stateToolNames(state)), state.thinking, state.haltOnNarration)
	final, updated, pending, lastUsage, duration, err := fn(cfg)
	final, truncated := tools.TruncateOutput(final)
	if updated != nil {
		// Persist the run's conversation only when it produced one: early
		// failures return nil and must not wipe stored state, and the write
		// happens under the same lock inspect reads under.
		m.mu.Lock()
		state.messages = updated
		state.pending = pending
		m.mu.Unlock()
	}
	if err != nil {
		if interrupted, ok := errors.AsType[*InterruptedError](err); ok {
			m.hostLog(interrupted.Error())
			result := interruptedResponse(hash, final, interrupted, truncated)
			if s, ok := result["response"].(string); ok && updated != nil {
				result["response"] = appendContextStats(s, m.api(), updated, lastUsage, duration, m.hostLog)
			}
			return result, nil
		}
		m.hostLog(verb + " failed: " + err.Error())
		return tools.ErrorResult(err.Error()), nil
	}
	m.hostLog(verb + " completed")
	result := map[string]any{"hash": hash, "response": final}
	if truncated {
		result["truncation_warning"] = fmt.Sprintf("Subagent response truncated to %d characters.", tools.MaxTextLimit)
	}
	if updated != nil {
		result["response"] = appendContextStats(final, m.api(), updated, lastUsage, duration, m.hostLog)
	}
	return result, nil
}

// generateHash builds a unique 16-char hash for a new run; the counter and
// wall clock keep identical (systemPrompt, prompt) pairs distinct.
func (m *Manager) generateHash(systemPrompt, prompt string) string {
	seq := m.hashSeq.Add(1)
	h := md5.Sum([]byte(fmt.Sprintf("%d|%s|%s|%s", seq, systemPrompt, prompt, time.Now().String())))
	return hex.EncodeToString(h[:])[:16]
}

// listEnabledTools returns all registered, subagent-enabled tool names,
// sorted for deterministic output in error messages and logs.
func (m *Manager) listEnabledTools() []string {
	var names []string
	for name := range tools.Registry {
		if m.isEnabled(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// validateTools checks the 'tools' allowlist of subagent_start and returns
// the deduplicated tool names. The list is optional: omitted or empty
// grants the subagent no tools. It fails only when the list is malformed
// or contains unknown or policy-blocked tools.
func (m *Manager) validateTools(args map[string]any) ([]string, error) {
	names := make([]string, 0)
	if _, exists := args["tools"]; !exists {
		return names, nil
	}
	list, ok := tools.ArgStringSlice(args, "tools")
	if !ok {
		return nil, fmt.Errorf("subagent_start failed: 'tools' must be an array of tool names")
	}
	seen := make(map[string]bool)
	var unknown, blocked []string
	for _, name := range list {
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
		if _, ok := tools.Registry[name]; !ok {
			unknown = append(unknown, name)
		} else if !m.isEnabled(name) {
			blocked = append(blocked, name)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("subagent_start failed: unknown tool(s) not in the tool registry: %s. Tools enabled for subagents: %s",
			strings.Join(unknown, ", "), strings.Join(m.listEnabledTools(), ", "))
	}
	if len(blocked) > 0 {
		return nil, fmt.Errorf("subagent_start failed: tool(s) blocked for subagents by security policy: %s. Enable them via the SUB checkbox in the GUI or subagent_enabled_tools in .skynet.yml",
			strings.Join(blocked, ", "))
	}
	return names, nil
}

// collectGrantedTools builds the SubTool set from a validated allowlist,
// sorted by name; policy-disabled tools are never exposed.
func (m *Manager) collectGrantedTools(names []string) []SubTool {
	var available []SubTool
	for _, name := range names {
		tool, ok := tools.Registry[name]
		if !ok || !m.isEnabled(name) {
			continue
		}
		desc, _ := tool.Definition["description"].(string)
		params, ok := tool.Definition["input_schema"].(map[string]any)
		if !ok {
			continue
		}
		available = append(available, SubTool{
			Name:        name,
			Description: desc,
			Parameters:  params,
			Execute:     tool.Execute,
		})
	}
	sort.Slice(available, func(i, j int) bool { return available[i].Name < available[j].Name })
	return available
}

// stateToolNames returns the run's allowlist (possibly empty); only runs
// created before the per-run allowlist existed (nil) fall back to all
// policy-enabled tools.
func (m *Manager) stateToolNames(state *runState) []string {
	if state.toolNames != nil {
		return state.toolNames
	}
	return m.listEnabledTools()
}

// buildConfig assembles the run Config from the host's current settings.
func (m *Manager) buildConfig(available []SubTool, thinking, haltOnNarration bool) Config {
	return Config{
		API:             m.api(),
		AvailableTools:  available,
		Log:             m.subLog,
		Thinking:        thinking,
		HaltOnNarration: haltOnNarration,
	}
}

// interruptedResponse builds the resumable response for an interrupted run:
// hash, last content, and the interruption reason.
func interruptedResponse(hash, final string, interrupted *InterruptedError, truncated bool) map[string]any {
	response := interrupted.Error()
	if final != "" {
		response = final + "\n\n" + response
	}
	result := map[string]any{"hash": hash, "response": response}
	if truncated {
		result["truncation_warning"] = fmt.Sprintf("Subagent response truncated to %d characters.", tools.MaxTextLimit)
	}
	return result
}
