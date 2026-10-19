//go:build windows

// Package tools provides tool handlers for file, memory, and expression operations.
package tools

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
)

// terminateBackgroundProcess kills the tracked process and all its child
// processes. taskkill /T terminates the entire process tree rooted at the
// given PID; Process.Kill() is a fallback that only terminates the direct
// process if taskkill fails.
func terminateBackgroundProcess(proc *bgProcess) {
	cmd := exec.Command("taskkill", "/F", "/T", fmt.Sprintf("/PID:%d", proc.pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{ //nolint: exhaustruct_v5 // stdlib type, only HideWindow needed
		HideWindow: true,
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		_ = proc.cmd.Process.Kill()
	}
}
