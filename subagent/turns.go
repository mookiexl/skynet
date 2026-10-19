package subagent

import (
	"fmt"
	"strings"
)

// Limits for the subagent inspection tools.
const (
	// inspectTurnListCap is the max number of turns listed by
	// subagent_inspect; older turns remain reachable by index.
	inspectTurnListCap = 50

	// inspectPreviewLen is the max length of a turn preview.
	inspectPreviewLen = 120
)

// Turn is one entry of a run's turn list, derived from the persisted
// conversation (kinds: think / content / tool / tool return).
type Turn struct {
	index   int
	kind    string
	content string
}

// deriveTurns builds the turn list from a persisted conversation. The
// index is stable across resume/reprompt because the conversation only
// grows by appending.
func deriveTurns(messages []Message) []Turn {
	var turns []Turn
	add := func(kind, content string) {
		turns = append(turns, Turn{index: len(turns), kind: kind, content: content})
	}
	for i := range messages {
		msg := &messages[i]
		switch msg.Role {
		case "assistant":
			if thinking := strings.TrimSpace(msg.ReasoningContent); thinking != "" {
				add("think", msg.ReasoningContent)
			}
			if content := messageText(msg.Content); strings.TrimSpace(content) != "" {
				add("content", content)
			}
			for _, tc := range msg.ToolCalls {
				add("tool", fmt.Sprintf("%s %s", tc.Function.Name, tc.Function.Arguments))
			}
		case "tool":
			add("tool return", messageText(msg.Content))
		}
	}
	return turns
}

// messageText extracts readable text from a message Content (a string or
// an array of content parts).
func messageText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, part := range v {
			if m, ok := part.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	return fmt.Sprint(content)
}

// inspectPreview truncates a turn's content to the preview length.
func inspectPreview(s string) string {
	if len(s) <= inspectPreviewLen {
		return s
	}
	return s[:inspectPreviewLen] + fmt.Sprintf("... [truncated, %d chars total]", len(s))
}

// getRunTurns looks up a run by hash and returns its turn list, or nil
// when the hash is unknown.
func (m *Manager) getRunTurns(hash string) []Turn {
	m.mu.RLock()
	state, exists := m.runs[hash]
	var msgs []Message
	if exists {
		msgs = append([]Message(nil), state.messages...)
	}
	m.mu.RUnlock()
	if !exists {
		return nil
	}
	return deriveTurns(msgs)
}
