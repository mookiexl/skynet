// Package main contains the MCP server implementation.
package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"skynet/subagent"
	"skynet/tools"
)

// applyToolsConfig syncs the runtime Config into the tools package limits.
func applyToolsConfig(cfg Config) {
	tools.MaxTextLimit.Store(int64(cfg.MaxTextLimit))
	tools.ShellTimeLimitSeconds.Store(int64(cfg.ShellTimeLimitSeconds))
	tools.MaxRollbackSize.Store(int64(cfg.MaxRollbackSizeMB) * tools.MBConversion)
	tools.MaxLoopHistory.Store(int64(cfg.MaxLoopHistory))
	tools.MaxSleepSeconds.Store(int64(cfg.MaxSleepSeconds))
	tools.RipgrepMaxResults.Store(int64(cfg.RipgrepMaxResults))
	tools.ExaMaxCharacters.Store(int64(cfg.ExaMaxCharacters))
	tools.MaxImageSize.Store(int64(cfg.MaxImageSizeMB) * tools.MBConversion)
	tools.MaxImageDimension.Store(int64(cfg.MaxImageDimension))
}

// subagentLimits converts the runtime Config into the subagent run budgets.
func subagentLimits(cfg Config) subagent.Limits {
	return subagent.Limits{
		MaxIterations:         cfg.SubagentMaxIterations,
		RequestTimeout:        time.Duration(cfg.SubagentRequestTimeoutSeconds) * time.Second,
		RunDeadline:           time.Duration(cfg.SubagentDeadlineSeconds) * time.Second,
		ReasoningBudgetTokens: cfg.SubagentThinkingBudgetTokens,
	}
}

func main() {
	// Load persisted settings
	settings, loaded := LoadSettings()
	if loaded {
		fmt.Println("Loading settings from", settingsFile)
	}

	// The embedded Config IS the runtime config; LoadSettings starts from
	// DefaultSettings, so any key missing from .skynet.yml keeps its default.
	cfg := settings.Config

	// Initialize the single source of truth
	def := DefaultSettings()
	State = &ServerState{
		mu:                   sync.RWMutex{},
		EnabledTools:         make(map[string]bool),
		SubagentEnabledTools: make(map[string]bool),
		Port:                 def.Port,
		RemoteAccess:         def.RemoteAccess,
		SubagentLLMAPI:       def.SubagentLLMAPI,
		Config:               cfg,
	}

	// Sync the runtime Config into the tools package limits
	applyToolsConfig(cfg)

	// Wire the subagent module into the host and register its tools
	// BEFORE initializing enabled tools from the registry, so the
	// subagent tools are included in State.EnabledTools (and thus the
	// GUI and dispatcher).
	subagentMgr = subagent.NewManager(subagent.Host{
		IsEnabled:   State.IsSubagentToolEnabled,
		LLMAPI:      State.GetSubagentLLMAPI,
		Limits:      subagentLimits(cfg),
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
