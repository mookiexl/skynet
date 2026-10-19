package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TokenizeTimeout bounds a single call to the llama.cpp tokenize endpoint.
// The conversation text can be very long, so the deadline is generous.
const TokenizeTimeout = 3 * time.Minute

// ContextTimeout bounds the /v1/models lookup used to read the server's
// context size.
const ContextTimeout = 30 * time.Second

// serverRoot derives the llama.cpp server's root URL from the OpenAI
// base URL: the subagents talk to <root>/v1, but the tokenize endpoint
// lives at <root>/tokenize.
func serverRoot(api string) string {
	if root, _, ok := strings.CutLast(api, "/v1"); ok {
		return root
	}
	return api
}

// conversationText renders the subagent's conversation as plain text so
// the server can count its tokens via the tokenize endpoint. All parts
// that occupy the model's context are included: thinking, content, and
// tool call names and arguments.
func conversationText(messages []Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(strings.ToUpper(m.Role))
		b.WriteString(": ")
		if m.ReasoningContent != "" {
			b.WriteString(m.ReasoningContent)
			b.WriteString("\n")
		}
		switch c := m.Content.(type) {
		case string:
			b.WriteString(c)
		case nil:
			// role markers only (e.g. the "tool" role is identified by its
			// tool_call_id, which we do not surface here)
		default:
			data, err := json.Marshal(c)
			if err == nil {
				b.WriteString(string(data))
			}
		}
		b.WriteString("\n")
		for _, tc := range m.ToolCalls {
			b.WriteString("tool call ")
			b.WriteString(tc.Function.Name)
			b.WriteString("(")
			b.WriteString(tc.Function.Arguments)
			b.WriteString(")\n")
		}
	}
	return b.String()
}

// contextUsedTokens counts the tokens of the given text with the
// llama.cpp server's tokenize endpoint (POST /tokenize, {"content": ...}).
func contextUsedTokens(root, text string) (int, error) {
	body, err := json.Marshal(map[string]string{"content": text})
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), TokenizeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/tokenize", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("tokenize endpoint returned HTTP %d", resp.StatusCode)
	}

	var doc struct {
		Tokens []int `json:"tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return 0, err
	}
	return len(doc.Tokens), nil
}

// contextSize returns the server's context size (n_ctx) for the endpoint.
// It is fetched fresh on every call: the endpoint may front a router or
// rotate backends, so a value cached from an earlier run can belong to a
// different model.
func contextSize(root string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/v1/models", nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("models endpoint returned HTTP %d", resp.StatusCode)
	}

	var doc struct {
		Data []struct {
			Meta struct {
				NCtx int `json:"n_ctx"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return 0, err
	}
	if len(doc.Data) == 0 {
		return 0, errors.New("no models reported by the LLM server")
	}
	v := doc.Data[0].Meta.NCtx
	if v <= 0 {
		return 0, errors.New("LLM server did not report n_ctx")
	}
	return v, nil
}

// formatContextStats renders the stats line appended to subagent responses.
func formatContextStats(used, total int) string {
	remaining := max(total-used, 0)
	return fmt.Sprintf("Context: %d of %d tokens used (%d remaining)", used, total, remaining)
}

// formatDuration renders the run's wall-clock duration line.
func formatDuration(d time.Duration) string {
	return "Duration: " + d.Round(time.Second).String()
}

// appendContextStats adds the context and duration stats lines to a
// response string. usedFromLLM is the last LLM request's usage (total
// context tokens, prompt + completion) as reported in the response's
// usage field; when it is 0 the server did not report it and the
// conversation is counted via the tokenize endpoint instead.
// It returns the response unchanged when the stats could not be
// computed, so a flaky lookup can never fail a subagent run; the failure
// is reported through logf so the degradation is visible.
func appendContextStats(response, api string, messages []Message, usedFromLLM int, duration time.Duration, logf func(string)) string {
	used := usedFromLLM
	if used == 0 {
		u, err := contextUsedTokens(serverRoot(api), conversationText(messages))
		if err != nil {
			logf("Context stats unavailable: " + err.Error())
		} else {
			used = u
		}
	}
	if used > 0 {
		total, err := contextSize(serverRoot(api))
		if err != nil {
			logf("Context stats unavailable: " + err.Error())
		} else {
			response += "\n\n" + formatContextStats(used, total)
		}
	}
	if duration > 0 {
		response += "\n\n" + formatDuration(duration)
	}
	return response
}
