package main

import (
	"sync"
)

// ServerState is the single source of truth for server configuration.
// Shared between the TUI model and HTTP handlers.
type ServerState struct {
	mu                   sync.RWMutex
	EnabledTools         map[string]bool
	SubagentEnabledTools map[string]bool
	Port                 int
	RemoteAccess         bool
	SubagentLLMAPI       string
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

// Global state - single source of truth
var State *ServerState
