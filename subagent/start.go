package subagent

import (
	"context"
	"fmt"
	"time"

	"skynet/tools"
)

// StartDefinition is the tool schema for subagent_start.
var StartDefinition = map[string]any{
	"type":        "function",
	"name":        "subagent_start",
	"description": "Spawns a new autonomous subagent run. Runs until the subagent produces response and returns response with hash to continue. Subagent only receives tools listed in the 'tools' parameter. Start fails if a listed tool does not exist or is blocked for subagents by the security policy.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"system_prompt":     map[string]any{"type": "string", "description": "System prompt defining the subagent's role, constraints and style. Use same format as your own system prompt."},
			"prompt":            map[string]any{"type": "string", "description": "Task given to the subagent, and information required to accomplish it."},
			"tools":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tools the subagent may use, if any. Only allow destructive or network-facing tools strictly required for task."},
			"thinking":          map[string]any{"type": "boolean", "description": "Only enable for demanding tasks."},
			"halt_on_narration": map[string]any{"type": "boolean", "description": "Subagent reports progress. Only enable if the task requires supervision."},
		},
		"required": []string{"system_prompt", "prompt", "thinking", "halt_on_narration"},
	},
}

// Start spawns a new subagent run.
func (m *Manager) Start(args map[string]any) (any, error) {
	systemPrompt, ok := tools.ArgString(args, "system_prompt")
	if !ok || systemPrompt == "" {
		return tools.ErrorResult("system_prompt is required"), nil
	}
	prompt, ok := tools.ArgString(args, "prompt")
	if !ok || prompt == "" {
		return tools.ErrorResult("prompt is required"), nil
	}
	thinking, _ := tools.ArgBool(args, "thinking")
	haltOnNarration, _ := tools.ArgBool(args, "halt_on_narration")

	// Validate the allowlist before registering so a failed start leaves
	// no state behind.
	toolNames, err := m.validateTools(args)
	if err != nil {
		m.hostLog(err.Error())
		return tools.ErrorResult(err.Error()), nil
	}

	hash := m.generateHash(systemPrompt, prompt)
	state := &runState{
		hash:            hash,
		systemPrompt:    systemPrompt,
		prompt:          prompt,
		thinking:        thinking,
		haltOnNarration: haltOnNarration,
		toolNames:       toolNames,
		messages:        nil,
		pending:         nil,
	}
	m.mu.Lock()
	m.runs[hash] = state
	m.mu.Unlock()

	m.hostLog(fmt.Sprintf("Subagent start invoked hash=%s", hash))
	return m.execute(state, "Subagent start", hash, func(cfg Config) (string, []Message, []PendingCall, int, time.Duration, error) {
		return RunWithHistory(context.Background(), cfg, InitialMessages(systemPrompt, prompt))
	})
}
