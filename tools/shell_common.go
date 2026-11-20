package tools

import (
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// buildShellResult builds a shell execution result map with truncation handling.
func buildShellResult(stdoutRaw, stderrRaw string, elapsedMs int64, exitCode int) map[string]any {
	stdoutTruncated, stdoutWasTruncated := TruncateOutput(stdoutRaw)
	stderrTruncated, stderrWasTruncated := TruncateOutput(stderrRaw)

	result := map[string]any{
		"stdout":     stdoutTruncated,
		"stderr":     stderrTruncated,
		"exit_code":  exitCode,
		"elapsed_ms": elapsedMs,
	}

	// Add truncation warning if output was truncated
	if stdoutWasTruncated || stderrWasTruncated {
		warning := fmt.Sprintf("Output was truncated to %d characters.", int(MaxTextLimit.Load()))
		if stdoutWasTruncated {
			warning += fmt.Sprintf(" stdout total length: %d chars.", len(stdoutRaw))
		}
		if stderrWasTruncated {
			warning += fmt.Sprintf(" stderr total length: %d chars.", len(stderrRaw))
		}
		result["truncation_warning"] = warning
	}

	return result
}

// runShellWithBackgroundTransition executes an already-started *exec.Cmd.
// If the command completes within ShellTimeLimit, returns the output.
// If the time limit is exceeded, registers the process as a background process
// and returns partial output + the background ID.
// The cmd must already be started (cmd.Start() called) with stdout/stderr wired.
func runShellWithBackgroundTransition(cmd *exec.Cmd, pgid int, stdout, stderr *boundedBuffer, startTime time.Time) map[string]any {
	// Spawn goroutine for cmd.Wait()
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Select on done + timer
	timer := time.After(time.Duration(ShellTimeLimitSeconds.Load()) * time.Second)
	select {
	case err := <-done:
		// Command completed
		elapsedMs := time.Since(startTime).Milliseconds()
		exitCode := 0
		if err != nil {
			exitError, ok := errors.AsType[*exec.ExitError](err)
			if !ok {
				return map[string]any{
					"error":     fmt.Sprintf("Command execution error: %v", err),
					"stdout":    stdout.String(),
					"stderr":    stderr.String(),
					"exit_code": -1,
				}
			}
			exitCode = exitError.ExitCode()
		}
		return buildShellResult(stdout.String(), stderr.String(), elapsedMs, exitCode)
	case <-timer:
		// Time limit exceeded - continue in background
		elapsedMs := time.Since(startTime).Milliseconds()
		id := registerBackgroundProcessWithPendingWait(cmd, pgid, stdout, stderr, done, startTime)
		stdoutRaw := stdout.String()
		stderrRaw := stderr.String()
		stdoutTruncated, stdoutWasTruncated := TruncateOutput(stdoutRaw)
		stderrTruncated, stderrWasTruncated := TruncateOutput(stderrRaw)
		result := map[string]any{
			"stdout":        stdoutTruncated,
			"stderr":        stderrTruncated,
			"elapsed_ms":    elapsedMs,
			"background_id": id,
			"message":       "time limit exceeded, continuing in background as " + id,
		}
		if stdoutWasTruncated || stderrWasTruncated {
			warning := fmt.Sprintf("Output was truncated to %d characters.", int(MaxTextLimit.Load()))
			if stdoutWasTruncated {
				warning += fmt.Sprintf(" stdout total length: %d chars.", len(stdoutRaw))
			}
			if stderrWasTruncated {
				warning += fmt.Sprintf(" stderr total length: %d chars.", len(stderrRaw))
			}
			result["truncation_warning"] = warning
		}
		return result
	}
}
