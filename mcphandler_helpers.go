package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"skynet/tools"
)

// MCPResponse represents a standard MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *MCPErr        `json:"error,omitempty"`
	ID      any            `json:"id,omitempty"`
}

// MCPErr represents an MCP JSON-RPC error
type MCPErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sendSuccess sends a success response with result data
func sendSuccess(w http.ResponseWriter, id any, result map[string]any) {
	resp := MCPResponse{
		JSONRPC: tools.JSONRPCVersion,
		Result:  result,
		Error:   nil,
		ID:      id,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		GlobalLog.AddLine(fmt.Sprintf("sendSuccess encode error: %v", err))
	}
}

// sendError sends an error response
func sendError(w http.ResponseWriter, id any, message string) {
	resp := MCPResponse{
		JSONRPC: tools.JSONRPCVersion,
		Error: &MCPErr{
			Code:    tools.JSONRPCErrInvalidParams,
			Message: message,
		},
		Result: nil,
		ID:     id,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		GlobalLog.AddLine(fmt.Sprintf("sendError encode error: %v", err))
	}
}

// sendNotification sends a notification response (no ID)
func sendNotification(w http.ResponseWriter) {
	resp := MCPResponse{
		JSONRPC: tools.JSONRPCVersion,
		Result:  map[string]any{},
		Error:   nil,
		ID:      nil,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		GlobalLog.AddLine(fmt.Sprintf("sendNotification encode error: %v", err))
	}
}
