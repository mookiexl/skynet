// Package tools provides tool handlers for file, memory, and expression operations.
package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// boundedBuffer is a mutex-guarded buffer that keeps only the last MaxTextLimit bytes.
type boundedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, _ = b.buf.Write(p)
	limit := int(MaxTextLimit.Load())
	if b.buf.Len() > limit {
		b.buf.Next(b.buf.Len() - limit)
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// bgProcess tracks a background process and its captured output.
type bgProcess struct {
	pgid             int
	pid              int
	cmd              *exec.Cmd
	stdout           *boundedBuffer
	stderr           *boundedBuffer
	startedAt        time.Time
	exitCode         int
	exited           bool
	externallyKilled bool
}

var (
	bgMu        sync.Mutex
	bgProcesses = make(map[string]*bgProcess)
)

// nextBackgroundID generates a new unique background process id.
// Must be called with bgMu held.
func nextBackgroundID() string {
	for {
		var buf [4]byte
		_, _ = rand.Read(buf[:])
		id := hex.EncodeToString(buf[:])
		if _, exists := bgProcesses[id]; !exists {
			return id
		}
	}
}

// evictFinishedProcesses drops oldest finished entries beyond BackgroundMaxEntries.
// Must be called with bgMu held.
func evictFinishedProcesses() {
	if len(bgProcesses) <= BackgroundMaxEntries {
		return
	}
	type finishedEntry struct {
		id        string
		startedAt time.Time
	}
	var finished []finishedEntry
	for id, p := range bgProcesses {
		if p.exited {
			finished = append(finished, finishedEntry{id, p.startedAt})
		}
	}
	if len(finished) == 0 {
		return
	}
	for i := 0; i < len(finished); i++ {
		for j := i + 1; j < len(finished); j++ {
			if finished[j].startedAt.Before(finished[i].startedAt) {
				finished[i], finished[j] = finished[j], finished[i]
			}
		}
	}
	for _, e := range finished {
		if len(bgProcesses) <= BackgroundMaxEntries {
			break
		}
		delete(bgProcesses, e.id)
	}
}

// buildBackgroundCheckResult builds the check result for a tracked process.
// Must be called with bgMu held.
func buildBackgroundCheckResult(id string, proc *bgProcess) map[string]any {
	stdout, _ := TruncateOutput(proc.stdout.String())
	stderr, _ := TruncateOutput(proc.stderr.String())

	result := map[string]any{
		"id":         id,
		"running":    !proc.exited,
		"stdout":     stdout,
		"stderr":     stderr,
		"elapsed_ms": time.Since(proc.startedAt).Milliseconds(),
	}
	if proc.exited {
		result["exit_code"] = proc.exitCode
	}
	return result
}

// TerminateAllBackgroundProcesses kills all tracked background processes.
func TerminateAllBackgroundProcesses() {
	bgMu.Lock()
	var procs []*bgProcess
	for _, proc := range bgProcesses {
		if !proc.exited {
			proc.externallyKilled = true
			procs = append(procs, proc)
		}
	}
	bgProcesses = make(map[string]*bgProcess)
	bgMu.Unlock()

	for _, proc := range procs {
		terminateBackgroundProcess(proc)
	}
}

// registerBackgroundProcessWithPendingWait registers an already-started command as a
// background process when a goroutine is already calling cmd.Wait().
// The done channel receives the result of cmd.Wait() from the existing goroutine.
// startTime is the original process start time, so elapsed_ms is accurate.
func registerBackgroundProcessWithPendingWait(cmd *exec.Cmd, pgid int, stdout, stderr *boundedBuffer, done <-chan error, startTime time.Time) string {
	pid := cmd.Process.Pid
	bgMu.Lock()
	id := nextBackgroundID()
	proc := &bgProcess{ //nolint: exhaustruct_v5 // exitCode/exited set by the Wait goroutine
		pgid:      pgid,
		pid:       pid,
		cmd:       cmd,
		stdout:    stdout,
		stderr:    stderr,
		startedAt: startTime,
	}
	bgProcesses[id] = proc
	evictFinishedProcesses()
	bgMu.Unlock()
	// Starting a new process means the agent is about to wait for *this*
	// process, so drop any stale wake from an earlier termination.
	ClearSleepWake()

	go func() {
		err := <-done
		exitCode := 0
		if err != nil {
			exitErr, ok := errors.AsType[*exec.ExitError](err)
			if !ok {
				exitCode = -1
			} else {
				exitCode = exitErr.ExitCode()
			}
		}
		bgMu.Lock()
		if proc.externallyKilled {
			exitCode = -1
		}
		proc.exitCode = exitCode
		proc.exited = true
		bgMu.Unlock()

		// Wake any active sleep so the agent can pick up the finished process.
		signalSleepWake("process " + id + " termination")
	}()

	return id
}

// ExecuteBackgroundCheck returns status and captured output for a tracked background process.
func ExecuteBackgroundCheck(args map[string]any) (any, error) {
	id, ok := ArgString(args, "id")
	if !ok || id == "" {
		return ErrorResult("no id provided"), nil
	}

	bgMu.Lock()
	proc, exists := bgProcesses[id]
	var result map[string]any
	if !exists {
		result = ErrorResult(fmt.Sprintf("no background process found with id '%s'", id))
	} else {
		result = buildBackgroundCheckResult(id, proc)
	}
	bgMu.Unlock()

	return result, nil
}

// ExecuteBackgroundTerminate terminates a tracked background process and its child processes.
func ExecuteBackgroundTerminate(args map[string]any) (any, error) {
	id, ok := ArgString(args, "id")
	if !ok || id == "" {
		return ErrorResult("no id provided"), nil
	}

	bgMu.Lock()
	proc, exists := bgProcesses[id]
	if !exists {
		bgMu.Unlock()
		return ErrorResult(fmt.Sprintf("no background process found with id '%s'", id)), nil
	}

	if proc.exited {
		result := map[string]any{
			"id":         id,
			"terminated": true,
			"running":    false,
			"exit_code":  proc.exitCode,
			"elapsed_ms": time.Since(proc.startedAt).Milliseconds(),
		}
		bgMu.Unlock()
		return result, nil
	}
	proc.externallyKilled = true
	// Release the lock before the (potentially slow) external kill so
	// check/start/terminate calls are not blocked for its duration.
	bgMu.Unlock()

	terminateBackgroundProcess(proc)

	// Wait briefly for the process to exit so we can report its exit code.
	deadline := time.Now().Add(BackgroundTerminateWait)
	for {
		bgMu.Lock()
		exited := proc.exited
		bgMu.Unlock()
		if exited || time.Now().After(deadline) {
			break
		}
		time.Sleep(BackgroundTerminatePollInterval)
	}

	bgMu.Lock()
	stillExited := proc.exited
	finalExitCode := proc.exitCode
	bgMu.Unlock()

	result := map[string]any{
		"id":         id,
		"terminated": true,
		"elapsed_ms": time.Since(proc.startedAt).Milliseconds(),
	}
	if stillExited {
		result["running"] = false
		result["exit_code"] = finalExitCode
	}
	return result, nil
}
