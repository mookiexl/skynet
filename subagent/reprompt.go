package subagent

import (
	"context"
	"fmt"
	"time"

	"skynet/tools"
)

// RepromptDefinition is the tool schema for subagent_reprompt.
var RepromptDefinition = map[string]any{
	"type":        "function",
	"name":        "subagent_reprompt",
	"description": "Continues an existing subagent run with a new prompt.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash":   map[string]any{"type": "string", "description": "Hash ID of the subagent run."},
			"prompt": map[string]any{"type": "string", "description": "New prompt to continue the conversation."},
		},
		"required": []string{"hash", "prompt"},
	},
}

// Reprompt continues an existing subagent run with a new prompt.
func (m *Manager) Reprompt(args map[string]any) (any, error) {
	hash, ok := tools.ArgString(args, "hash")
	if !ok || hash == "" {
		return tools.ErrorResult("hash is required"), nil
	}
	prompt, ok := tools.ArgString(args, "prompt")
	if !ok || prompt == "" {
		return tools.ErrorResult("prompt is required"), nil
	}
	m.mu.Lock()
	state, exists := m.runs[hash]
	if !exists {
		m.mu.Unlock()
		return tools.ErrorResult("subagent run not found"), nil
	}
	state.prompt = prompt
	// A new question supersedes the unexecuted tool calls of the
	// terminating step, so they are dropped.
	state.pending = nil
	msgs := append([]Message(nil), state.messages...)
	msgs = append(msgs, Message{Role: "user", Content: prompt}) //nolint:exhaustruct_v5
	m.mu.Unlock()

	m.hostLog(fmt.Sprintf("Subagent reprompt hash=%s", hash))
	return m.execute(state, "Subagent reprompt", hash, func(cfg Config) (string, []Message, []PendingCall, int, time.Duration, error) {
		return RunWithHistory(context.Background(), cfg, msgs)
	})
}
