//go:build unix

package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func init() {
	RegisterTool("exec_screenshot", ExecScreenshotDefinition, ExecuteExecScreenshot)
}

// ExecScreenshotDefinition defines the tool schema.
var ExecScreenshotDefinition = map[string]any{
	"description": "Execute an executable from the working directory and take a screenshot of the spawned window after a delay.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"executable": map[string]any{
				"type":        "string",
				"description": "Path to the executable relative to the working directory",
			},
			"delay": map[string]any{
				"type":        "number",
				"description": "Delay before screenshot in seconds (default: 5)",
			},
		},
		"required": []string{"executable"},
	},
}

type execScreenshotArgs struct {
	executable   string
	delaySeconds int
}

func parseExecScreenshotArgs(args map[string]any) (execScreenshotArgs, error) {
	executable, ok := ArgString(args, "executable")
	if !ok || executable == "" {
		return execScreenshotArgs{}, fmt.Errorf("invalid executable path")
	}

	delaySeconds := 5
	if delayFloat, ok := ArgFloat(args, "delay"); ok {
		delaySeconds = int(delayFloat)
		delaySeconds = max(1, delaySeconds)
	}

	return execScreenshotArgs{
		executable:   executable,
		delaySeconds: delaySeconds,
	}, nil
}

// ExecuteExecScreenshot executes an executable from the working directory, waits for a delay, takes a screenshot, and returns it.
func ExecuteExecScreenshot(args map[string]any) (any, error) {
	execArgs, err := parseExecScreenshotArgs(args)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Validate executable path against working directory
	absExecPath, err := ValidatePath(execArgs.executable)
	if err != nil {
		return map[string]any{
			"error":     fmt.Sprintf("Invalid executable path: %v", err),
			"exit_code": -1,
		}, nil
	}

	// Check file exists
	if _, err := os.Stat(absExecPath); err != nil {
		return map[string]any{
			"error":     fmt.Sprintf("Executable not found: %v", err),
			"exit_code": -1,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execArgs.delaySeconds+1)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, absExecPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{ //nolint:exhaustruct_v5
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return map[string]any{
			"error":     fmt.Sprintf("Failed to start executable: %v", err),
			"exit_code": -1,
		}, nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}

	// Wait for delay before screenshot
	delay := time.Duration(execArgs.delaySeconds) * time.Second
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		killProcessGroupByPGID(pgid)
		_ = cmd.Wait()
		return map[string]any{
			"error":     fmt.Sprintf("Execution timed out after %d seconds", execArgs.delaySeconds+1),
			"exit_code": -1,
		}, nil
	}

	// Take screenshot
	screenshotData, screenshotErr := takeScreenshot()
	if screenshotErr != nil {
		killProcessGroupByPGID(pgid)
		_ = cmd.Wait()
		return map[string]any{
			"error":     fmt.Sprintf("Screenshot failed: %v", screenshotErr),
			"exit_code": -1,
		}, nil
	}

	// Kill process group after screenshot
	killProcessGroupByPGID(pgid)

	// Wait briefly for process to exit
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()

	cmdWait := make(chan error, 1)
	go func() {
		cmdWait <- cmd.Wait()
	}()

	select {
	case <-cmdWait:
	case <-waitCtx.Done():
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		<-cmdWait
	}

	return buildScreenshotResult(screenshotData, absExecPath, execArgs.delaySeconds), nil
}
