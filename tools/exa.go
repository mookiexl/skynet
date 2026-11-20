package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"time"
)

func init() {
	RegisterTool("web_search_exa", WebSearchExaDefinition, ExecuteWebSearchExa)
	RegisterTool("web_fetch_exa", WebFetchExaDefinition, ExecuteWebFetchExa)
}

// WebSearchExaDefinition defines the web_search_exa tool schema.
var WebSearchExaDefinition = map[string]any{
	"description": "Search the web for any topic and get clean, ready-to-use content.\n\nBest for: Finding current information, news, facts, people, companies, or answering questions about any topic.\nReturns: Clean text content from top search results.\n\nQuery tips:\ndescribe the ideal page, not just keywords. Use category:people / category:company to focus results.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language search query. Should be a semantically rich description of the ideal page, not just keywords.",
			},
			"numResults": map[string]any{
				"type":        "number",
				"description": "Number of search results to return (default: 10).",
			},
		},
		"required": []string{"query"},
	},
}

// WebFetchExaDefinition defines the web_fetch_exa tool schema.
var WebFetchExaDefinition = map[string]any{
	"description": "Read a webpage's full content as clean markdown. Use after web_search_exa when highlights are insufficient or to read any URL.\n\nBest for: Extracting full content from known URLs. Batch multiple URLs in one call.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"urls": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "URLs to read. Batch multiple URLs in one call.",
			},
			"maxCharacters": map[string]any{
				"type":        "number",
				"description": "Maximum characters to extract per page (default: 3000)",
			},
		},
		"required": []string{"urls"},
	},
}

// exaMCPEndpoint is the Exa MCP server endpoint.
const exaMCPEndpoint = "https://mcp.exa.ai/mcp"

// exaMCPRPCRequest is the JSON-RPC request format for MCP.
type exaMCPRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// exaMCPRPCResponse is the parsed result from the MCP server.
type exaMCPRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// callExaMCP sends a JSON-RPC request to the Exa MCP server and returns the parsed response.
func callExaMCP(ctx context.Context, req exaMCPRPCRequest) (*exaMCPRPCResponse, error) {
	apiKey := os.Getenv("EXA_API_KEY")

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, exaMCPEndpoint, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("x-api-key", apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Exa MCP: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exa MCP returned status %d", resp.StatusCode)
	}

	// Parse SSE format: lines start with "data: ".
	// A single data line can carry the whole page as JSON, so raise the
	// scanner's default 64KB token limit (same pattern as subagent's
	// readChatStream).
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 5 && line[:5] == "data:" {
			data := line[5:]
			var parsed exaMCPRPCResponse
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}
			if parsed.Error.Code != 0 {
				return nil, fmt.Errorf("exa MCP error %d: %s", parsed.Error.Code, parsed.Error.Message)
			}
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("no data received from Exa MCP")
}

// parseExaResult parses the raw Exa result JSON and extracts the first content text.
func parseExaResult(raw json.RawMessage) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Exa response: %w", err)
	}

	// Extract text from MCP content structure
	if contentArr, ok := result["content"].([]any); ok && len(contentArr) > 0 {
		if first, ok := contentArr[0].(map[string]any); ok {
			if text, ok := first["text"].(string); ok {
				result["content"] = html.UnescapeString(text)
			}
		}
	}

	return result, nil
}

// ExecuteWebSearchExa searches the web via Exa.
//
//nolint:gocognit,cyclop
func ExecuteWebSearchExa(args map[string]any) (any, error) {
	query, ok := ArgString(args, "query")
	if !ok || query == "" {
		return ErrorResult("query is required"), nil
	}

	numResults := 10
	if numFloat, ok := ArgFloat(args, "numResults"); ok {
		numResults = int(numFloat)
		numResults = max(1, numResults)
		numResults = min(20, numResults)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := exaMCPRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "web_search_exa",
			"arguments": map[string]any{
				"query":      query,
				"numResults": numResults,
			},
		},
	}

	resp, err := callExaMCP(ctx, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	result, err := parseExaResult(resp.Result)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return result, nil
}

// ExecuteWebFetchExa fetches webpage content via Exa.
//
//nolint:gocognit,cyclop
func ExecuteWebFetchExa(args map[string]any) (any, error) {
	urlsAny, ok := args["urls"]
	if !ok {
		return ErrorResult("urls is required"), nil
	}

	urls, ok := urlsAny.([]any)
	if !ok || len(urls) == 0 {
		return ErrorResult("urls must be a non-empty array"), nil
	}

	// Convert []any to []string
	urlStrings := make([]string, len(urls))
	for i, u := range urls {
		if s, ok := u.(string); ok {
			urlStrings[i] = s
		}
	}

	maxChars := 3000
	if maxFloat, ok := ArgFloat(args, "maxCharacters"); ok {
		maxChars = int(maxFloat)
	}
	// Clamp to the configured range (numResults is already clamped 1..20).
	maxChars = max(1, maxChars)
	maxChars = min(int(ExaMaxCharacters.Load()), maxChars)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := exaMCPRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "web_fetch_exa",
			"arguments": map[string]any{
				"urls":          urlStrings,
				"maxCharacters": maxChars,
			},
		},
	}

	resp, err := callExaMCP(ctx, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	result, err := parseExaResult(resp.Result)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return result, nil
}
