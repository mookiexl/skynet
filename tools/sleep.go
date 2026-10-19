package tools

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MaxSleepSeconds is the maximum allowed sleep duration in seconds.
const MaxSleepSeconds = 600

// sleepWoken is a persistent "wake" flag. It is raised whenever a background
// process terminates (or the user signals a wake) and is cleared only when a
// sleep consumes it. Because it is NOT reset when a sleep begins, a sleep that
// starts after a process has already finished wakes immediately.
var sleepWoken atomic.Int32

// sleepWakeMu guards sleepWakeReason.
var sleepWakeMu sync.Mutex

// sleepWakeReason records why a sleep was (or will be) woken early, so the wake
// result can name the cause. Each wake signal overwrites it, so when several
// processes terminate before a sleep the reason refers to the one that finished
// last. It is consumed (cleared) together with the flag when a sleep wakes.
var sleepWakeReason string

// signalSleepWake raises the persistent wake flag and records the reason.
func signalSleepWake(reason string) {
	sleepWakeMu.Lock()
	sleepWakeReason = reason
	sleepWakeMu.Unlock()
	sleepWoken.Store(1)
}

// consumeWakeReason clears the wake flag and returns the recorded reason.
func consumeWakeReason() string {
	sleepWoken.Store(0)
	sleepWakeMu.Lock()
	reason := sleepWakeReason
	sleepWakeReason = ""
	sleepWakeMu.Unlock()
	return reason
}

// ClearSleepWake resets the persistent wake flag and drops any recorded
// reason. It is called when a new background process starts, so a stale
// termination from an earlier process is not attributed to a later sleep:
// starting a new process means the agent is about to wait for *that* process.
func ClearSleepWake() {
	sleepWoken.Store(0)
	sleepWakeMu.Lock()
	sleepWakeReason = ""
	sleepWakeMu.Unlock()
}

// WakeSleep signals any active sleep (main or subagent) to wake up early.
func WakeSleep() {
	signalSleepWake("user")
}

// SleepDefinition defines the tool schema.
var SleepDefinition = map[string]any{
	"description": "Wait a specified number of seconds. Wakes early on process finish or user signal.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"seconds": map[string]any{
				"type":        "integer",
				"description": "Number of seconds to sleep (1-600)",
			},
		},
		"required": []string{"seconds"},
	},
}

func init() {
	RegisterTool("sleep", SleepDefinition, ExecuteSleep)
}

// ExecuteSleep waits for the given number of seconds, checking for a wake
// signal each second. It wakes early if the flag was already raised (a process
// finished before the sleep started, or the user pressed Wake Up) or if the
// flag is raised during the sleep.
func ExecuteSleep(args map[string]any) (any, error) {
	sec, ok := args["seconds"].(float64)
	if !ok {
		return ErrorResult("seconds must be a number"), nil
	}
	seconds := int(sec)
	if seconds < 1 {
		return ErrorResult("seconds must be at least 1"), nil
	}
	if seconds > MaxSleepSeconds {
		return ErrorResult(fmt.Sprintf("seconds must be at most %d", MaxSleepSeconds)), nil
	}

	// If the wake flag is already raised (e.g. a process finished before we
	// started sleeping), wake immediately.
	if sleepWoken.Load() == 1 {
		reason := consumeWakeReason()
		if reason == "" {
			reason = "unknown cause"
		}
		return map[string]any{"result": fmt.Sprintf("awakened by %s", reason)}, nil
	}

	for i := range seconds {
		time.Sleep(time.Second)
		if sleepWoken.Load() == 1 {
			elapsed := i + 1
			reason := consumeWakeReason()
			if reason == "" {
				reason = "unknown cause"
			}
			return map[string]any{"result": fmt.Sprintf("awakened by %s after %d seconds", reason, elapsed)}, nil
		}
	}

	return map[string]any{"result": fmt.Sprintf("slept %d seconds", seconds)}, nil
}
