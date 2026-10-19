package main

import (
	"context"
	"net/http"
	"sync"

	"skynet/tools"
)

var (
	serverMutex  sync.Mutex
	httpServer   *http.Server
	serverActive = false
)

// IsServerActive reports whether the HTTP server is running.
func IsServerActive() bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	return serverActive
}

// setServerActive sets the server-active flag.
func setServerActive(v bool) {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	serverActive = v
}

// StopServer gracefully stops the HTTP server and terminates all
// background processes.
func StopServer() {
	serverMutex.Lock()
	if httpServer != nil {
		_ = httpServer.Shutdown(context.Background())
		httpServer = nil
	}
	serverMutex.Unlock()

	setServerActive(false)
	tools.TerminateAllBackgroundProcesses()
}
