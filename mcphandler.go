package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"skynet/tools"
	"strings"
	"sync"
)

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
	ID     any            `json:"id"`
}

// LoopEntry records one observed (call signature, result) pair for loop
// detection.
type LoopEntry struct {
	signature string
	result    string
}

var (
	loopMutex   sync.Mutex
	loopHistory []LoopEntry
	maxLoopHist = tools.MaxLoopHistory
)

// getCallSignature builds a canonical string identifying a tool call
// (name plus its marshaled arguments).
func getCallSignature(name string, args map[string]any) string {
	data, _ := json.Marshal(args)
	return fmt.Sprintf("%s:%s", name, string(data))
}

// checkAndAddToLoopHistory reports whether this exact call has already
// produced the same result, and appends the entry to the history — all
// under one lock. Returns true when a loop was detected.
func checkAndAddToLoopHistory(name string, args map[string]any, result string) bool {
	loopMutex.Lock()
	defer loopMutex.Unlock()

	sig := getCallSignature(name, args)
	for _, entry := range loopHistory {
		if entry.signature == sig && entry.result == result {
			return true
		}
	}
	loopHistory = append(loopHistory, LoopEntry{signature: sig, result: result})
	if len(loopHistory) > maxLoopHist {
		loopHistory = loopHistory[1:]
	}
	return false
}

// ClearLoopHistory clears the loop detection history
func ClearLoopHistory() {
	loopMutex.Lock()
	defer loopMutex.Unlock()
	loopHistory = []LoopEntry{}
}

// corsMiddleware adds CORS headers and, when remote access is disabled,
// rejects any request that does not come from localhost.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the client IP
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		// Check if remote access is allowed
		if !State.IsRemoteAccessEnabled() {
			// Only allow localhost connections
			if ip != "127.0.0.1" && ip != "::1" && ip != "localhost" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("Access denied: Remote access is disabled. Only localhost connections allowed."))
				return
			}
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, mcp-protocol-version")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func handlePromptsList(w http.ResponseWriter, req MCPRequest) {
	prompts := ScanPrompts()
	var promptsList []map[string]any
	for _, prompt := range prompts {
		promptsList = append(promptsList, map[string]any{
			"name":        prompt.Name,
			"description": prompt.Description,
			"content":     prompt.Content,
		})
	}
	sendSuccess(w, req.ID, map[string]any{
		"prompts": promptsList,
	})
}

func handlePromptsGet(w http.ResponseWriter, req MCPRequest) {
	params := req.Params
	promptName, ok := params["name"].(string)
	if !ok {
		sendError(w, req.ID, "Prompt name missing")
		return
	}

	prompt, err := GetPrompt(promptName)
	if err != nil {
		sendError(w, req.ID, err.Error())
		return
	}

	sendSuccess(w, req.ID, map[string]any{
		"name":        prompt.Name,
		"description": "Prompt: " + prompt.Name,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": map[string]any{
					"type": "text",
					"text": prompt.Content,
				},
			},
		},
	})
}

func handleInitialize(w http.ResponseWriter, req MCPRequest) {
	protocolVersion := tools.JSONRPCVersion
	if req.Params != nil {
		if version, ok := req.Params["protocolVersion"].(string); ok {
			protocolVersion = version
		}
	}

	sendSuccess(w, req.ID, map[string]any{
		"protocolVersion": protocolVersion,
		"serverInfo": map[string]string{
			"name":    "skynet-server",
			"version": "1.0.0",
		},
		"capabilities": map[string]any{
			"tools":   map[string]any{},
			"prompts": map[string]any{},
		},
	})
}

func handleToolsList(w http.ResponseWriter, req MCPRequest) {
	GlobalLog.AddLine("Incoming request: tools/list")
	var toolsList []map[string]any
	for name, tool := range tools.Registry {
		if !State.IsToolEnabled(name) {
			continue
		}
		toolsList = append(toolsList, map[string]any{
			"name":        name,
			"description": tool.Definition["description"],
			"inputSchema": tool.Definition["input_schema"],
		})
	}
	sendSuccess(w, req.ID, map[string]any{
		"tools": toolsList,
	})
}

func getToolParams(params map[string]any) (string, map[string]any) {
	toolName, _ := params["name"].(string)
	if toolName == "" {
		toolName, _ = params["toolName"].(string)
	}
	arguments, _ := params["arguments"].(map[string]any)
	if arguments == nil {
		arguments = make(map[string]any)
	}
	return toolName, arguments
}

func validateTool(name string) error {
	_, exists := tools.Registry[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}
	if !State.IsToolEnabled(name) {
		return fmt.Errorf("tool %s is disabled", name)
	}
	return nil
}

func handleToolsCall(w http.ResponseWriter, req MCPRequest) {
	toolName, arguments := getToolParams(req.Params)
	if toolName == "" {
		sendError(w, req.ID, "Tool name missing")
		return
	}

	if err := validateTool(toolName); err != nil {
		sendError(w, req.ID, err.Error())
		return
	}

	GlobalLog.AddLine(fmt.Sprintf("Incoming request: tools/call -> %s", toolName))
	tool := tools.Registry[toolName]
	result, err := tool.Execute(arguments)
	if err != nil {
		GlobalLog.AddLine(fmt.Sprintf("Error executing %s: %v", toolName, err))
		sendError(w, req.ID, err.Error())
		return
	}

	var responseText string
	if resultMap, ok := result.(map[string]any); ok {
		var lines []string
		for k, v := range resultMap {
			lines = append(lines, fmt.Sprintf("%s: %v", k, v))
		}
		responseText = strings.Join(lines, "\n")
	} else {
		responseText = fmt.Sprintf("%v", result)
	}

	resJSON, _ := json.Marshal(result)
	loopDetectionKey := string(resJSON)

	if checkAndAddToLoopHistory(toolName, arguments, loopDetectionKey) {
		GlobalLog.AddLine(fmt.Sprintf("Loop detected for %s", toolName))
		sendSuccess(w, req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Error: redundant tool call - identical call with identical result. You have already done this."},
			},
		})
		return
	}

	GlobalLog.AddLine(fmt.Sprintf("Success: %s executed successfully", toolName))

	if resultMap, ok := result.(map[string]any); ok {
		if isImage, ok := resultMap["is_image"].(bool); ok && isImage {
			sendSuccess(w, req.ID, map[string]any{
				"content": []map[string]any{
					{
						"type":     "image",
						"data":     resultMap["data"],
						"mimeType": resultMap["media_type"],
					},
				},
			})
			return
		}
	}

	sendSuccess(w, req.ID, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": responseText},
		},
	})
}

func handleNotificationsInitialized(w http.ResponseWriter, _ MCPRequest) {
	sendNotification(w)
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("mcp-protocol-version", tools.JSONRPCVersion)

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "prompts/list":
		handlePromptsList(w, req)
	case "prompts/get":
		handlePromptsGet(w, req)
	case "initialize":
		handleInitialize(w, req)
	case "tools/list":
		handleToolsList(w, req)
	case "tools/call":
		handleToolsCall(w, req)
	case "notifications/initialized":
		handleNotificationsInitialized(w, req)
	default:
		sendError(w, req.ID, "Method not found: "+req.Method)
	}
}
