package main

import (
	"fmt"
	"skynet/tools"
	"strings"
	"sync"
	"time"
)

// Global log buffer for TUI
var GlobalLog *LogBuffer

// Global subagent log buffer for subagent monitoring tab
var SubagentLog *LogBuffer

func init() {
	GlobalLog = NewLogBuffer(tools.LogBufferSize)
	SubagentLog = NewLogBuffer(tools.SubagentLogLines)
}

// LogBuffer is a thread-safe buffer for TUI logs
type LogBuffer struct {
	mu    sync.Mutex
	Lines []string
	Limit int
}

// NewLogBuffer creates a new LogBuffer
func NewLogBuffer(limit int) *LogBuffer {
	return &LogBuffer{
		mu:    sync.Mutex{},
		Lines: make([]string, 0, limit),
		Limit: limit,
	}
}

// Snapshot returns a copy of all lines (thread-safe)
func (lb *LogBuffer) Snapshot() []string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	out := make([]string, len(lb.Lines))
	copy(out, lb.Lines)
	return out
}

// AddLine adds a line to the log buffer
func (lb *LogBuffer) AddLine(msg string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	formattedMsg := fmt.Sprintf("[%s] %s", timestamp, msg)

	lb.Lines = append(lb.Lines, formattedMsg)
	// Keep only the last 'Limit' lines
	if len(lb.Lines) > lb.Limit {
		lb.Lines = lb.Lines[len(lb.Lines)-lb.Limit:]
	}
}

// AddSubagentLine appends a line to the subagent monitoring buffer (thread-safe)
func AddSubagentLine(msg string) {
	SubagentLog.AddLine(msg)
}

// Tail returns the last n lines joined with newlines, each line followed by "\n".
func (lb *LogBuffer) Tail(n int) string {
	lines := lb.Snapshot()
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	var b strings.Builder
	for _, line := range lines[start:] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// GetSubagentLogText returns the current subagent log buffer as a string
func GetSubagentLogText() string {
	return SubagentLog.Tail(tools.TUILogDisplayLines)
}
