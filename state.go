package main

import (
	"sync"

	"skynet/subagent"
	"skynet/tools"
)

// Config holds the configurable runtime limits. It is both the runtime
// state and the .skynet.yml payload (Settings embeds it), so there is a
// single field list with a single source of defaults. Durations are stored
// as plain ints in seconds so the yaml file stays trivial; the tools and
// subagent packages convert to their units.
type Config struct {
	// General
	MaxTextLimit          int `yaml:"max_text_limit"`
	ShellTimeLimitSeconds int `yaml:"shell_time_limit_seconds"`
	MaxRollbackSizeMB     int `yaml:"max_rollback_size_mb"`
	MaxLoopHistory        int `yaml:"max_loop_history"`
	MaxSleepSeconds       int `yaml:"max_sleep_seconds"`
	// Ripgrep
	RipgrepMaxResults int `yaml:"ripgrep_max_results"`
	// Web search
	ExaMaxCharacters int `yaml:"exa_max_characters"`
	// Subagent
	SubagentRequestTimeoutSeconds int `yaml:"subagent_request_timeout_seconds"`
	SubagentDeadlineSeconds       int `yaml:"subagent_deadline_seconds"`
	SubagentMaxIterations         int `yaml:"subagent_max_iterations"`
	SubagentThinkingBudgetTokens  int `yaml:"subagent_thinking_budget_tokens"`
	// Multimodal
	MaxImageSizeMB    int `yaml:"max_image_size_mb"`
	MaxImageDimension int `yaml:"max_image_dimension"`
}

// DefaultConfig returns the default runtime limits.
func DefaultConfig() Config {
	return Config{
		MaxTextLimit:                  tools.DefaultMaxTextLimit,
		ShellTimeLimitSeconds:         int(tools.DefaultShellTimeLimit.Seconds()),
		MaxRollbackSizeMB:             tools.DefaultMaxRollbackSizeMB,
		MaxLoopHistory:                tools.DefaultMaxLoopHistory,
		MaxSleepSeconds:               tools.DefaultMaxSleepSeconds,
		RipgrepMaxResults:             tools.DefaultRipgrepMaxResults,
		ExaMaxCharacters:              tools.DefaultExaMaxCharacters,
		SubagentRequestTimeoutSeconds: int(subagent.DefaultRequestTimeout.Seconds()),
		SubagentDeadlineSeconds:       int(subagent.DefaultRunDeadline.Seconds()),
		SubagentMaxIterations:         subagent.DefaultMaxIterations,
		SubagentThinkingBudgetTokens:  subagent.DefaultReasoningBudgetTokens,
		MaxImageSizeMB:                tools.DefaultMaxImageSizeMB,
		MaxImageDimension:             tools.DefaultMaxImageDimension,
	}
}

// ServerState is the single source of truth for server configuration.
// Shared between the TUI model and HTTP handlers.
type ServerState struct {
	mu                   sync.RWMutex
	EnabledTools         map[string]bool
	SubagentEnabledTools map[string]bool
	Port                 int
	RemoteAccess         bool
	SubagentLLMAPI       string
	Config               Config
}

// IsToolEnabled checks if a tool is enabled (thread-safe)
func (s *ServerState) IsToolEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.EnabledTools[name]
}

// SetToolEnabled toggles a tool's enabled state (thread-safe)
func (s *ServerState) SetToolEnabled(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EnabledTools[name] = enabled
}

// IsSubagentToolEnabled checks if a tool is enabled for subagents (thread-safe)
func (s *ServerState) IsSubagentToolEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SubagentEnabledTools[name]
}

// SetSubagentToolEnabled toggles a tool's subagent enabled state (thread-safe)
func (s *ServerState) SetSubagentToolEnabled(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SubagentEnabledTools[name] = enabled
}

// GetPort returns the current port (thread-safe)
func (s *ServerState) GetPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Port
}

// SetPort sets the port (thread-safe)
func (s *ServerState) SetPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Port = port
}

// ToggleRemoteAccess toggles remote access (thread-safe)
func (s *ServerState) ToggleRemoteAccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RemoteAccess = !s.RemoteAccess
}

// IsRemoteAccessEnabled checks if remote access is enabled (thread-safe)
func (s *ServerState) IsRemoteAccessEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RemoteAccess
}

// GetSubagentLLMAPI returns the subagent LLM API endpoint (thread-safe)
func (s *ServerState) GetSubagentLLMAPI() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SubagentLLMAPI
}

// SetSubagentLLMAPI sets the subagent LLM API endpoint (thread-safe)
func (s *ServerState) SetSubagentLLMAPI(api string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SubagentLLMAPI = api
}

// GetConfig returns a copy of the configurable runtime limits (thread-safe)
func (s *ServerState) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Config
}

// SetConfig replaces the configurable runtime limits (thread-safe)
func (s *ServerState) SetConfig(c Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = c
}

// Global state - single source of truth
var State *ServerState
