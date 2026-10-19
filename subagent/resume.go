package subagent

import (
	"context"
	"fmt"
	"time"

	"skynet/tools"
)

// ResumeDefinition is the tool schema for subagent_resume.
var ResumeDefinition = map[string]any{
	"type":        "function",
	"name":        "subagent_resume",
	"description": "Continues execution of an existing subagent run.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{"type": "string", "description": "Hash ID of the subagent run."},
		},
		"required": []string{"hash"},
	},
}

// Resume continues an existing subagent run.
func (m *Manager) Resume(args map[string]any) (any, error) {
	hash, ok := tools.ArgString(args, "hash")
	if !ok || hash == "" {
		return tools.ErrorResult("hash is required"), nil
	}
	m.mu.RLock()
	state, exists := m.runs[hash]
	var msgs []Message
	var pending []PendingCall
	if exists {
		msgs = append([]Message(nil), state.messages...)
		pending = append([]PendingCall(nil), state.pending...)
	}
	m.mu.RUnlock()
	if !exists {
		return tools.ErrorResult("subagent run not found"), nil
	}
	m.hostLog(fmt.Sprintf("Subagent resume hash=%s (pending=%d)", hash, len(pending)))
	return m.execute(state, "Subagent resume", hash, func(cfg Config) (string, []Message, []PendingCall, int, time.Duration, error) {
		return Resume(context.Background(), cfg, msgs, pending)
	})
}
