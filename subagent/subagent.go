// Package subagent implements the subagent execution loop.
//
// A subagent is an autonomous agent run by the main LLM via the "subagent"
// tool. It talks to an OpenAI-compatible LLM server (llama.cpp server) and
// iterates: think (streamed content) -> call tools (SUB-enabled) -> read
// results -> ... until it produces a final answer.
//
// All intermediate activity (thinking, tool calls, tool results) is streamed
// through the provided Log function into a caller-supplied log buffer that
// keeps only the last 100 lines (the GUI "Subagent" tab).
package subagent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"skynet/tools"
)

// errDone signals that the subagent produced its final answer, i.e. a step
// with no tool calls. Run must stop and return the accumulated content.
var errDone = errors.New("subagent finished with a final answer")

// InterruptedError signals that the run was stopped before the subagent
// produced a final answer: user termination, total-run timeout, max
// iterations, or loop detection. The host still returns the run hash along
// with the last accumulated content so the run can be resumed with
// subagent_resume.
type InterruptedError struct {
	Reason string
}

func (e *InterruptedError) Error() string {
	return "subagent " + e.Reason
}

const (
	// DefaultMaxIterations is the default maximum number of LLM round-trips in
	// one subagent run.
	DefaultMaxIterations = 100

	// DefaultRequestTimeout is the default per-LLM-request timeout.
	DefaultRequestTimeout = 10 * time.Minute

	// LogResultTruncate is the max characters of a tool result shown in the log.
	LogResultTruncate = 2000

	// DefaultRunDeadline is the default total wall-clock budget for one
	// subagent run.
	DefaultRunDeadline = 60 * time.Minute

	// DefaultReasoningBudgetTokens is the default token budget for the model's
	// reasoning (thinking) on each subagent LLM request when thinking is
	// enabled.
	DefaultReasoningBudgetTokens = 4096
)

// Limits are the configurable run budgets for a subagent run. Defaults come
// from DefaultLimits; the host sets them from the loaded settings.
type Limits struct {
	// MaxIterations is the maximum number of LLM round-trips in one run.
	MaxIterations int
	// RequestTimeout is the per-LLM-request timeout.
	RequestTimeout time.Duration
	// RunDeadline is the total wall-clock budget for one run.
	RunDeadline time.Duration
	// ReasoningBudgetTokens is the reasoning token budget per request when
	// thinking is enabled.
	ReasoningBudgetTokens int
}

// DefaultLimits returns the default subagent run budgets.
func DefaultLimits() Limits {
	return Limits{
		MaxIterations:         DefaultMaxIterations,
		RequestTimeout:        DefaultRequestTimeout,
		RunDeadline:           DefaultRunDeadline,
		ReasoningBudgetTokens: DefaultReasoningBudgetTokens,
	}
}

// reasoningBudgetMessage is sent to the model when its reasoning budget
// is exceeded, mirroring the main app's implementation.
const reasoningBudgetMessage = "... [[REASONING LIMIT REACHED: You must now respond and/or take action based on the reasoning you produced so far.]] Responding now."

// Active-run state: at most one subagent run is tracked at a time.
var (
	runMu           sync.Mutex
	runCancelHolder *context.CancelFunc
	runActive       bool
)

// SubTool is a tool the subagent is allowed to call.
type SubTool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Execute     func(args map[string]any) (any, error)
}

// Config carries everything Run needs from the host server.
type Config struct {
	// API is the OpenAI-compatible base URL (e.g. http://localhost:8080/v1).
	API string
	// AvailableTools are the tools the subagent may use (SUB-enabled only).
	AvailableTools []SubTool
	// Log streams a line of subagent activity to the monitoring buffer.
	Log func(string)
	// Thinking enables extended (reasoning) thinking on the model side.
	// When true, the model's reasoning budget is capped at
	// Limits.ReasoningBudgetTokens for every subagent LLM request.
	Thinking bool
	// Limits are the configurable run budgets for this run.
	Limits Limits
	// HaltOnNarration controls when the run terminates. When true, any step
	// that produces answer content ends the run, even if tool calls were
	// also requested. When false, a step with content AND tool calls
	// continues (the calls are executed); only a content step without tool
	// calls terminates the run. The host must always set this explicitly —
	// it is a required parameter of subagent_start and has no default.
	HaltOnNarration bool
}

// chat wire types (OpenAI-compatible)

type functionWire struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolCallWire struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionWire `json:"function"`
}

// Message is a single conversation message.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
	// ReasoningContent holds the model's thinking for assistant turns. It is
	// persisted in the conversation and re-sent on resume because modern
	// thinking LLMs expect previous thinking turns to be preserved.
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []toolCallWire `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
}

type toolDefWire struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatTemplateKwargs struct {
	EnableThinking bool `json:"enable_thinking"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type usageWire struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatRequest struct {
	Messages             []Message           `json:"messages"`
	Tools                []toolDefWire       `json:"tools,omitempty"`
	ToolChoice           string              `json:"tool_choice"`
	Stream               bool                `json:"stream"`
	StreamOptions        *streamOptions      `json:"stream_options,omitempty"`
	ThinkingBudgetTokens *int                `json:"thinking_budget_tokens,omitempty"`
	ReasoningBudgetMsg   string              `json:"reasoning_budget_message,omitempty"`
	ChatTemplateKwargs   *chatTemplateKwargs `json:"chat_template_kwargs,omitempty"`
}

type deltaToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatDelta struct {
	Role             string          `json:"role"`
	Content          any             `json:"content"`
	ReasoningContent string          `json:"reasoning_content"`
	ToolCalls        []deltaToolCall `json:"tool_calls"`
}

// chatChunk is one SSE data payload of the stream. The final chunk
// (sent when stream_options.include_usage is set) carries no choices but
// a usage object with the request's prompt and completion token counts.
type chatChunk struct {
	Choices []struct {
		Delta        chatDelta `json:"delta"`
		FinishReason *string   `json:"finish_reason"`
	} `json:"choices"`
	Usage *usageWire `json:"usage,omitempty"`
}

// completedToolCall is a fully accumulated streamed tool call.
type completedToolCall struct {
	id        string
	name      string
	arguments string
}

// PendingCall is a tool call that the model requested in the step that
// terminated the run but that was not executed, because that step also
// contained answer content. Callers persist it so the run can resume
// exactly where it stopped.
type PendingCall struct {
	ID        string
	Name      string
	Arguments string
}

// RunWithHistory executes the subagent loop over an existing conversation
// (whose first message should be the system prompt). It returns the final
// response text, the full conversation, any unexecuted tool calls requested
// in the terminating step, the last LLM request's usage (total context
// tokens; 0 when unreported), and the run's wall-clock duration. Callers
// should persist both the returned messages and pending calls so the run
// can resume exactly where it stopped.
func RunWithHistory(ctx context.Context, cfg Config, messages []Message) (string, []Message, []PendingCall, int, time.Duration, error) {
	if cfg.API == "" {
		return "", nil, nil, 0, 0, fmt.Errorf("subagent LLM API is not configured")
	}
	if len(messages) == 0 {
		return "", nil, nil, 0, 0, fmt.Errorf("conversation is empty")
	}

	logf := cfg.Log
	if logf == nil {
		logf = func(string) {}
	}
	logf(fmt.Sprintf("Subagent started (api=%s, tools=%d)", cfg.API, len(cfg.AvailableTools)))

	return runLoop(ctx, cfg, messages, nil, logf)
}

// Resume continues a run exactly where it stopped: if the run ended with
// unexecuted pending tool calls, they are executed first and their results
// appended to the conversation, then the loop continues. It returns the
// final response text, the full conversation, and any new unexecuted tool
// calls from the terminating step, the last LLM request's usage (total
// context tokens; 0 when unreported), and the run's wall-clock duration.
func Resume(ctx context.Context, cfg Config, messages []Message, pending []PendingCall) (string, []Message, []PendingCall, int, time.Duration, error) {
	if cfg.API == "" {
		return "", nil, nil, 0, 0, fmt.Errorf("subagent LLM API is not configured")
	}
	if len(messages) == 0 {
		return "", nil, nil, 0, 0, fmt.Errorf("conversation is empty")
	}

	logf := cfg.Log
	if logf == nil {
		logf = func(string) {}
	}
	logf(fmt.Sprintf("Subagent resumed (api=%s, tools=%d, pending=%d)", cfg.API, len(cfg.AvailableTools), len(pending)))

	return runLoop(ctx, cfg, messages, fromPending(pending), logf)
}

// beginRun claims the single active subagent slot. ok is false when a run
// is already active, in which case start/resume/reprompt must fail cleanly
// rather than two runs racing on the slot.
func beginRun(ctx context.Context) (context.Context, func(), bool) {
	runMu.Lock()
	defer runMu.Unlock()
	if runActive {
		return context.Background(), nil, false
	}
	runCtx, cancel := context.WithCancel(ctx)
	holder := &cancel
	runActive = true
	runCancelHolder = holder
	return runCtx, func() {
		cancel()
		runMu.Lock()
		if runCancelHolder == holder {
			runCancelHolder = nil
			runActive = false
		}
		runMu.Unlock()
	}, true
}

// finishRun maps a terminating step error to the run's return values: an
// InterruptedError yields the last accumulated content (resumable via hash),
// a cancelled context is treated as user termination, and anything else is a
// hard error.
func finishRun(runCtx context.Context, lastContent string, messages []Message, err error) (string, []Message, error) {
	if interrupted, ok := errors.AsType[*InterruptedError](err); ok {
		return lastContent, messages, interrupted
	}
	if runCtx.Err() != nil {
		return lastContent, messages, &InterruptedError{Reason: "terminated by user"}
	}
	return lastContent, messages, err
}

// runLoop tracks the active run, executes any pending tool calls, and runs
// the iteration loop until the subagent answers directly, the budget is
// exhausted, or the run is cancelled.
func runLoop(ctx context.Context, cfg Config, messages []Message, pending []completedToolCall, logf func(string)) (string, []Message, []PendingCall, int, time.Duration, error) {
	runCtx, release, ok := beginRun(ctx)
	if !ok {
		return "", nil, nil, 0, 0, fmt.Errorf("a subagent run is already active")
	}
	defer release()

	var lastContent string
	var lastUsage int
	var loopHistory = make(map[string]int)
	start := time.Now()

	if len(pending) > 0 {
		logf(fmt.Sprintf("Executing %d pending tool call(s) from previous step", len(pending)))
		if err := runToolCalls(runCtx, cfg, &messages, pending, &loopHistory, logf); err != nil {
			f, m, e := finishRun(runCtx, lastContent, messages, err)
			return f, m, nil, lastUsage, time.Since(start), e
		}
	}

	for iter := 1; iter <= cfg.Limits.MaxIterations; iter++ {
		remaining := cfg.Limits.RunDeadline - time.Since(start)
		content, toolCalls, usage, err := runIteration(runCtx, cfg, &messages, &loopHistory, logf, iter, remaining)
		if err != nil {
			if errors.Is(err, errDone) {
				return content, messages, toPending(toolCalls), usage, time.Since(start), nil
			}
			if content != "" {
				lastContent = content
			}
			f, m, e := finishRun(runCtx, lastContent, messages, err)
			return f, m, nil, lastUsage, time.Since(start), e
		}
		lastContent = content
		lastUsage = usage
	}
	f, m, e := finishRun(runCtx, lastContent, messages, &InterruptedError{Reason: fmt.Sprintf("reached max iterations (%d) without a final answer", cfg.Limits.MaxIterations)})
	return f, m, nil, lastUsage, time.Since(start), e
}

// runIteration performs one LLM round trip. If the step contains answer
// content, it terminates the run: errDone is returned and the tool calls
// from that step are passed back (unexecuted) as pending calls. Otherwise
// the requested tool calls are executed and their results appended.
func runIteration(runCtx context.Context, cfg Config, messages *[]Message, loopHistory *map[string]int, logf func(string), iter int, remaining time.Duration) (string, []completedToolCall, int, error) {
	if err := runCtx.Err(); err != nil {
		return "", nil, 0, &InterruptedError{Reason: "terminated by user"}
	}
	if remaining <= 0 {
		return "", nil, 0, &InterruptedError{Reason: fmt.Sprintf("exceeded total run deadline (%s)", cfg.Limits.RunDeadline)}
	}
	// Cap the per-request timeout to the per-LLM-request budget so a single
	// slow request cannot blow past RequestTimeout by waiting on the full
	// RunDeadline window.
	reqTimeout := min(remaining, cfg.Limits.RequestTimeout)
	content, thinking, toolCalls, usageTotal, err := llmStreamCall(runCtx, cfg, *messages, logf, reqTimeout)
	if err != nil {
		return "", nil, 0, fmt.Errorf("subagent LLM request failed: %w", err)
	}
	// A content step terminates the run, and the assistant message is
	// appended with its (possibly unexecuted) tool calls so a resume can
	// execute exactly those pending calls. With HaltOnNarration enabled,
	// content terminates even when tool calls were also requested; with it
	// disabled, a content step continues as long as it also requested tools.
	// The model's thinking is persisted in the message so resumed
	// conversations preserve previous thinking turns.
	if strings.TrimSpace(content) != "" && (len(toolCalls) == 0 || cfg.HaltOnNarration) {
		logf(fmt.Sprintf("Subagent finished after %d iteration(s) with a direct answer", iter))
		appendAssistantMessage(messages, content, thinking, toolCalls)
		return content, toolCalls, usageTotal, errDone
	}

	appendAssistantMessage(messages, content, thinking, toolCalls)

	if err := runToolCalls(runCtx, cfg, messages, toolCalls, loopHistory, logf); err != nil {
		return content, nil, usageTotal, err
	}
	return content, nil, usageTotal, nil
}

// toPending converts internal completed tool calls into the exported type.
func toPending(calls []completedToolCall) []PendingCall {
	pending := make([]PendingCall, len(calls))
	for i, c := range calls {
		pending[i] = PendingCall{ID: c.id, Name: c.name, Arguments: c.arguments}
	}
	return pending
}

// fromPending converts exported pending calls into the internal type.
func fromPending(pending []PendingCall) []completedToolCall {
	calls := make([]completedToolCall, len(pending))
	for i, p := range pending {
		calls[i] = completedToolCall{id: p.ID, name: p.Name, arguments: p.Arguments}
	}
	return calls
}

// InitialMessages builds the starting conversation.
func InitialMessages(systemPrompt, prompt string) []Message {
	return []Message{
		{Role: "system", Content: systemPrompt}, //nolint:exhaustruct_v5
		{Role: "user", Content: prompt},         //nolint:exhaustruct_v5
	}
}

// appendAssistantMessage appends the assistant turn (thinking + content +
// tool calls).
func appendAssistantMessage(messages *[]Message, content, reasoning string, calls []completedToolCall) {
	wireCalls := make([]toolCallWire, len(calls))
	for i, tc := range calls {
		wireCalls[i] = toolCallWire{
			ID:       tc.id,
			Type:     "function",
			Function: functionWire{Name: tc.name, Arguments: tc.arguments},
		}
	}
	*messages = append(*messages, Message{Role: "assistant", Content: content, ReasoningContent: reasoning, ToolCalls: wireCalls}) //nolint:exhaustruct_v5
}

// loopWarning is sent to the subagent on the first repetition of an
// identical tool call with an identical result, mirroring the main agent's
// loop detection.
const loopWarning = "Error: redundant tool call - identical call with identical result. You have already done this."

// runToolCalls executes each requested tool call and appends the results to
// the conversation. Loop detection works like the main agent's: the first
// repetition of an identical call with an identical result replaces the
// result with a warning so the subagent can break out of the loop; only if
// the subagent keeps repeating is the run terminated. It also returns an
// error if the run was cancelled between tool calls.
func runToolCalls(runCtx context.Context, cfg Config, messages *[]Message, calls []completedToolCall, loopHistory *map[string]int, logf func(string)) error {
	for _, tc := range calls {
		// Respond to Terminate promptly between tool calls rather than
		// executing additional tool calls on a cancelled run.
		if err := runCtx.Err(); err != nil {
			return &InterruptedError{Reason: "terminated by user"}
		}
		logf(fmt.Sprintf("Tool call: %s %s", tc.name, tc.arguments))

		resultText, err := executeSubagentTool(cfg, tc)
		if err != nil {
			logf(fmt.Sprintf("Tool error (%s): %v", tc.name, err))
		} else {
			logf(fmt.Sprintf("Tool result (%s): %s", tc.name, truncateForLog(resultText, LogResultTruncate)))
			history := *loopHistory
			key := loopKey(tc, resultText)
			repeated := history[key]
			switch repeated {
			case 0:
				// First time this identical call+result occurs.
				history[key] = 1
			case 1:
				// First repetition: warn the subagent, keep the run going.
				logf(fmt.Sprintf("Loop detected in %s - warning sent to subagent", tc.name))
				history[key] = 2
				resultText = loopWarning
			default:
				// Still repeating after the warning: terminate the run.
				logf(fmt.Sprintf("Loop detected in %s - stopping", tc.name))
				return &InterruptedError{Reason: fmt.Sprintf("stuck in a loop calling %s with identical results", tc.name)}
			}
		}
		*messages = append(*messages, Message{ //nolint:exhaustruct_v5
			Role:       "tool",
			ToolCallID: tc.id,
			Content:    resultText,
		})
	}
	return nil
}

// loopKey builds the map key identifying an identical tool call with an
// identical result, matching the main agent's call-signature + result
// comparison.
func loopKey(tc completedToolCall, result string) string {
	return tc.name + "\x00" + tc.arguments + "\x00" + result
}

// llmStreamCall performs one streaming chat-completion round trip and
// returns the accumulated assistant content, its thinking, the completed
// tool calls, and the request's usage (total context tokens; 0 when the
// server did not report it).
func llmStreamCall(parentCtx context.Context, cfg Config, messages []Message, logf func(string), timeout time.Duration) (string, string, []completedToolCall, int, error) {
	data, err := json.Marshal(buildChatRequest(cfg, messages))
	if err != nil {
		return "", "", nil, 0, err
	}

	// The context must live for the whole body read, not just the request:
	// Go aborts resp.Body reads when the request context is done.
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	resp, err := postChatRequest(ctx, cfg.API, data)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	content, thinking, pending, usageTotal, err := readChatStream(resp.Body, logf)
	if err != nil {
		return "", "", nil, 0, err
	}

	calls := orderedCalls(pending)
	if len(calls) > 0 {
		logf(fmt.Sprintf("Assistant requested %d tool call(s)", len(calls)))
	}
	return content, thinking, calls, usageTotal, nil
}

// buildChatRequest assembles the OpenAI-compatible request body.
func buildChatRequest(cfg Config, messages []Message) chatRequest {
	reqBody := chatRequest{ //nolint:exhaustruct_v5
		Messages:           messages,
		ToolChoice:         "auto",
		Stream:             true,
		StreamOptions:      &streamOptions{IncludeUsage: true},
		ChatTemplateKwargs: &chatTemplateKwargs{EnableThinking: cfg.Thinking},
	}
	// Cap the model's reasoning budget when thinking is enabled so a
	// demanding subagent task cannot burn unbounded thinking tokens.
	if cfg.Thinking {
		budget := cfg.Limits.ReasoningBudgetTokens
		reqBody.ThinkingBudgetTokens = &budget
		reqBody.ReasoningBudgetMsg = reasoningBudgetMessage
	}
	for _, t := range cfg.AvailableTools {
		reqBody.Tools = append(reqBody.Tools, toolDefWire{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return reqBody
}

// postChatRequest posts the request and checks the HTTP status. The caller
// owns ctx and must keep it alive until resp.Body is fully read.
func postChatRequest(ctx context.Context, api string, data []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, api+"/chat/completions", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("LLM server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

// readChatStream consumes the SSE stream, streaming thinking and response
// lines through logf, and accumulates the full content, full thinking,
// pending tool calls, and the request's usage (total token count of the
// prompt plus completion; 0 when the server did not report it).
func readChatStream(body io.Reader, logf func(string)) (string, string, map[int]*completedToolCall, int, error) {
	content := strings.Builder{}
	contentBuf := strings.Builder{} // partial line held across deltas
	thinking := strings.Builder{}
	thinkingBuf := strings.Builder{}
	pending := make(map[int]*completedToolCall)
	usageTotal := 0

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk chatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			// Skip malformed chunk rather than killing the whole run.
			continue
		}
		if chunk.Usage != nil {
			usageTotal = chunk.Usage.TotalTokens
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		processDelta(chunk.Choices[0].Delta, &content, &contentBuf, &thinking, &thinkingBuf, pending, logf)
	}
	if err := scanner.Err(); err != nil {
		return "", "", nil, 0, err
	}
	if err := finalizeStream(&content, &contentBuf, &thinking, &thinkingBuf, pending, logf); err != nil {
		return "", "", nil, 0, err
	}
	return content.String(), thinking.String(), pending, usageTotal, nil
}

// appendSegment appends streamed text and logs each completed line with the
// given prefix, keeping the partial tail in buf.
func appendSegment(full, buf *strings.Builder, prefix, text string, logf func(string)) {
	buf.WriteString(text)
	s := buf.String()
	if idx := strings.LastIndexByte(s, '\n'); idx >= 0 {
		line := strings.TrimRight(s[:idx], "\n")
		full.WriteString(s[:idx+1])
		rest := ""
		if idx+1 < len(s) {
			rest = s[idx+1:]
		}
		buf.Reset()
		buf.WriteString(rest)
		if line != "" {
			logf(prefix + line)
		}
	}
}

// flushTail logs the final partial line of a segment.
func flushTail(full, buf *strings.Builder, prefix string, logf func(string)) {
	if buf.Len() == 0 {
		return
	}
	full.WriteString(buf.String())
	if line := strings.TrimSpace(buf.String()); line != "" {
		logf(prefix + line)
	}
}

// finalizeStream flushes any remaining lines and validates the result.
func finalizeStream(content, contentBuf, thinking, thinkingBuf *strings.Builder, pending map[int]*completedToolCall, logf func(string)) error {
	flushTail(content, contentBuf, "Assistant: ", logf)
	flushTail(thinking, thinkingBuf, "Thinking: ", logf)
	if content.Len() == 0 && len(pending) == 0 {
		return errors.New("LLM returned an empty response")
	}
	return nil
}

// processDelta handles one stream delta: appends thinking and response text
// and updates the pending tool calls.
func processDelta(delta chatDelta, content, contentBuf, thinking, thinkingBuf *strings.Builder, pending map[int]*completedToolCall, logf func(string)) {
	if s := delta.ReasoningContent; s != "" {
		appendSegment(thinking, thinkingBuf, "Thinking: ", s, logf)
	}
	if s, ok := delta.Content.(string); ok {
		appendSegment(content, contentBuf, "Assistant: ", s, logf)
	}

	for _, tc := range delta.ToolCalls {
		p, ok := pending[tc.Index]
		if !ok {
			p = &completedToolCall{} //nolint:exhaustruct_v5
			pending[tc.Index] = p
		}
		if tc.ID != "" {
			p.id = tc.ID
		}
		if tc.Function.Name != "" {
			p.name = tc.Function.Name
		}
		p.arguments += tc.Function.Arguments
	}
}

// orderedCalls sorts pending tool calls by their original delta index.
func orderedCalls(pending map[int]*completedToolCall) []completedToolCall {
	keys := make([]int, 0, len(pending))
	for i := range pending {
		keys = append(keys, i)
	}
	sort.Ints(keys)
	var calls []completedToolCall
	for _, k := range keys {
		calls = append(calls, *pending[k])
	}
	return calls
}

// executeSubagentTool runs a SUB-enabled tool and returns its text result.
func executeSubagentTool(cfg Config, tc completedToolCall) (string, error) {
	name, argsJSON := tc.name, tc.arguments
	var tool *SubTool
	for i := range cfg.AvailableTools {
		if cfg.AvailableTools[i].Name == name {
			tool = &cfg.AvailableTools[i]
			break
		}
	}
	if tool == nil {
		return "Error: tool " + name + " is not available to subagents", fmt.Errorf("tool %s is not available to subagents", name)
	}

	args := make(map[string]any)
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error(), fmt.Errorf("invalid tool arguments for %s: %w", name, err)
		}
	}

	result, err := tool.Execute(args)
	if err != nil {
		return "Error: " + err.Error(), err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "Error: " + err.Error(), err
	}
	return string(data), nil
}

// truncateForLog truncates long tool results for the monitoring log,
// never splitting a multi-byte UTF-8 rune.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return tools.TruncateRuneSafe(s, maxLen) + fmt.Sprintf("... [truncated, %d chars total]", len(s))
}

// Terminate cancels the active subagent run, if any.
func Terminate() error {
	runMu.Lock()
	defer runMu.Unlock()
	if !runActive {
		return errors.New("no active subagent")
	}
	(*runCancelHolder)()
	runActive = false
	runCancelHolder = nil
	return nil
}
