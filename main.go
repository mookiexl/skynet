// Package main contains the MCP server implementation.
package main

import (
	"fmt"
	"os"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"skynet/subagent"
	"skynet/tools"
)

func main() {
	// Load persisted settings
	settings, loaded := LoadSettings()
	if loaded {
		fmt.Println("Loading settings from", settingsFile)
	}

	// Initialize the single source of truth
	def := DefaultSettings()
	State = &ServerState{
		mu:                   sync.RWMutex{},
		EnabledTools:         make(map[string]bool),
		SubagentEnabledTools: make(map[string]bool),
		Port:                 def.Port,
		RemoteAccess:         def.RemoteAccess,
		SubagentLLMAPI:       def.SubagentLLMAPI,
	}

	// Wire the subagent module into the host and register its tools
	// BEFORE initializing enabled tools from the registry, so the
	// subagent tools are included in State.EnabledTools (and thus the
	// GUI and dispatcher).
	subagentMgr = subagent.NewManager(subagent.Host{
		IsEnabled:   State.IsSubagentToolEnabled,
		LLMAPI:      State.GetSubagentLLMAPI,
		HostLog:     GlobalLog.AddLine,
		SubagentLog: AddSubagentLine,
	})
	registerSubagentTools()

	// Initialize enabled tools from registry
	for name := range tools.Registry {
		// Use loaded setting if available, otherwise default to true
		if enabled, exists := settings.EnabledTools[name]; exists {
			State.EnabledTools[name] = enabled
		} else {
			State.EnabledTools[name] = true
		}

		// Subagent tools default to enabled unless the tool is listed in
		// defaultSubagentDisabledTools; an explicit .skynet.yml setting
		// always takes precedence.
		if enabled, exists := settings.SubagentEnabledTools[name]; exists {
			State.SubagentEnabledTools[name] = enabled
		} else {
			State.SubagentEnabledTools[name] = !isDefaultDisabledForSubagent(name)
		}
	}

	// Apply loaded port setting
	if settings.Port > 0 && settings.Port < tools.MaxPort {
		State.Port = settings.Port
	}

	// Apply loaded remote access setting
	State.RemoteAccess = settings.RemoteAccess

	// Apply loaded subagent LLM API setting
	if settings.SubagentLLMAPI != "" {
		State.SubagentLLMAPI = settings.SubagentLLMAPI
	}

	// Force X11 backend (Wayland icon support is flaky in Fyne v2.8)
	if os.Getenv("FYNENATIVE") == "" {
		_ = os.Setenv("XDG_SESSION_TYPE", "x11")
	}

	// Create Fyne app and show TUI
	fyneApp := app.NewWithID("skynet-mcp-server")
	fyneApp.SetIcon(icon)
	app.SetMetadata(fyne.AppMetadata{
		ID:         "skynet-mcp-server",
		Name:       "Skynet MCP Server",
		Version:    "1.0.0",
		Build:      1,
		Icon:       icon,
		Migrations: map[string]bool{"fyneDo": true},
		Release:    false,
		Custom:     map[string]string{},
	})
	ShowToolTUI(fyneApp)
}
