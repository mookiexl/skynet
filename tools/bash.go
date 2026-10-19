//go:build unix

// Package tools provides tool handlers for file, memory, and expression operations.
package tools

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

func init() {
	RegisterTool("bash", BashDefinition, ExecuteBash)
}

// BashDefinition defines the tool schema.
var BashDefinition = map[string]any{
	"description": "Execute a bash command and return the output. If the command exceeds the time limit, it continues in the background. Rollback unavailable - only use for tasks not possible with built-in tools.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The bash command to execute",
			},
		},
		"required": []string{"command"},
	},
}

// ExecuteBash executes a bash command and returns the output.
// If the command exceeds ShellTimeLimit, it continues in the background.
func ExecuteBash(args map[string]any) (any, error) {
	command, ok := ArgString(args, "command")
	if !ok || command == "" {
		return ErrorResult("no command provided"), nil
	}

	stdout := &boundedBuffer{} //nolint: exhaustruct_v5 // zero-value fields are self-managed by the buffer
	stderr := &boundedBuffer{} //nolint: exhaustruct_v5 // zero-value fields are self-managed by the buffer

	cmd := exec.Command("bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{ //nolint: exhaustruct_v5 // stdlib type, only Setpgid needed
		Setpgid: true,
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return map[string]any{
			"error":     fmt.Sprintf("Failed to start command: %v", err),
			"exit_code": -1,
		}, nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}

	return runShellWithBackgroundTransition(cmd, pgid, stdout, stderr, startTime), nil
}

// killProcessGroupByPGID kills a process group by its known PGID.
func killProcessGroupByPGID(pgid int) {
	// Send SIGTERM first
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(KillProcessGroupGracePeriodMs * time.Millisecond)
	// Then SIGKILL
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
