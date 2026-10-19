package tools

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

func init() {
	RegisterTool("powershell", PowerShellDefinition, ExecutePowerShell)
}

// PowerShellDefinition defines the tool schema.
var PowerShellDefinition = map[string]any{
	"description": "Execute a PowerShell command and return the output. If the command exceeds the time limit, it continues in the background. Rollback unavailable - only use for tasks not possible with built-in tools.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The PowerShell command to execute",
			},
		},
		"required": []string{"command"},
	},
}

// powershellExecutable returns the PowerShell executable name for the current OS.
// Windows ships "powershell" (Windows PowerShell); Linux/macOS use "pwsh" (PowerShell Core).
func powershellExecutable() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "pwsh"
}

// ExecutePowerShell executes a PowerShell command and returns the output.
// If the command exceeds ShellTimeLimit, it continues in the background.
func ExecutePowerShell(args map[string]any) (any, error) {
	command, ok := ArgString(args, "command")
	if !ok || command == "" {
		return ErrorResult("no command provided"), nil
	}

	executable := powershellExecutable()

	// Check that PowerShell is installed before attempting to run it.
	if _, lookErr := exec.LookPath(executable); lookErr != nil {
		return map[string]any{
			"error":     fmt.Sprintf("PowerShell (%s) is not installed or not in PATH: %v", executable, lookErr),
			"exit_code": -1,
		}, nil
	}

	stdout := &boundedBuffer{} //nolint: exhaustruct_v5 // zero-value fields are self-managed by the buffer
	stderr := &boundedBuffer{} //nolint: exhaustruct_v5 // zero-value fields are self-managed by the buffer

	cmd := exec.Command(executable, "-Command", command)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return map[string]any{
			"error":     fmt.Sprintf("Failed to start command: %v", err),
			"exit_code": -1,
		}, nil
	}

	return runShellWithBackgroundTransition(cmd, cmd.Process.Pid, stdout, stderr, startTime), nil
}
