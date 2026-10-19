//go:build unix

// Package tools provides tool handlers for file, memory, and expression operations.
package tools

// terminateBackgroundProcess kills the process group of the tracked process.
func terminateBackgroundProcess(proc *bgProcess) {
	killProcessGroupByPGID(proc.pgid)
}
