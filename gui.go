package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"image/color"
	"maps"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"skynet/subagent"
	"skynet/tools"
)

// equalRowLayout divides available width equally among all objects in a row.
type equalRowLayout struct{}

func (e equalRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	cellW := size.Width / float32(len(objects))
	for i, o := range objects {
		o.Resize(fyne.NewSize(cellW, size.Height))
		o.Move(fyne.NewPos(cellW*float32(i), 0))
	}
}

func (e equalRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return fyne.NewSize(450, 45)
}

// fyneModel holds the Fyne-specific UI state bound to ServerState
type fyneModel struct {
	toolRows         []fyne.CanvasObject
	remoteBtn        *widget.Button
	startBtn         *widget.Button
	logLabel         *widget.Label
	subagentLogLabel *widget.Label
	portEntry        *widget.Entry
	subagentLLMEntry *widget.Entry
}

// refreshStartButton syncs the start/stop button's text and color with the
// server's actual running state.
func refreshStartButton(btn *widget.Button) {
	if IsServerActive() {
		btn.SetText("Stop Server")
		btn.Importance = widget.HighImportance
	} else {
		btn.SetText("Start Server")
		btn.Importance = widget.WarningImportance
	}
}

// setRemoteBtnText syncs the remote access button's label with the state.
func setRemoteBtnText(btn *widget.Button) {
	if State.IsRemoteAccessEnabled() {
		btn.SetText("Remote Access")
	} else {
		btn.SetText("Local Only")
	}
}

// newBlackScroll wraps a label in a black-background scrolling container
// (used for the Log and Subagent tabs).
func newBlackScroll(label *widget.Label) *container.Scroll {
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	return container.NewScroll(container.NewStack(bg, container.NewVBox(label)))
}

// buildFyneModel creates the Fyne model from current ServerState
func buildFyneModel() fyneModel {
	// Build sorted tool name list from registry
	State.mu.RLock()
	toolNames := make([]string, 0, len(State.EnabledTools))
	for name := range State.EnabledTools {
		toolNames = append(toolNames, name)
	}
	State.mu.RUnlock()

	sort.Strings(toolNames)

	// Header row: MAIN | SUB | Tool
	headerRow := container.NewHBox(widget.NewLabel("MAIN"), widget.NewLabel("SUB"), widget.NewLabel("Tool"))

	toolRows := make([]fyne.CanvasObject, 0, len(toolNames)+1)
	toolRows = append(toolRows, headerRow)

	for _, name := range toolNames {
		State.mu.RLock()
		clientChecked := State.EnabledTools[name]
		subChecked := State.SubagentEnabledTools[name]
		State.mu.RUnlock()

		// MAIN checkbox: enabled for client
		mainCb := widget.NewCheck("", func(checked bool) {
			State.SetToolEnabled(name, checked)
		})
		mainCb.Checked = clientChecked

		// SUB checkbox: enabled for subagent
		subCb := widget.NewCheck("", func(checked bool) {
			State.SetSubagentToolEnabled(name, checked)
		})
		subCb.Checked = subChecked

		// Row: MAIN | SUB | tool name
		nameLabel := widget.NewLabel(name)
		toolRows = append(toolRows, container.NewHBox(mainCb, subCb, nameLabel))
	}

	// (var + assign needed: closures may not reference a variable in its own
	// := initializer expression)
	var remoteBtn *widget.Button
	remoteBtn = widget.NewButton("", func() {
		State.ToggleRemoteAccess()
		setRemoteBtnText(remoteBtn)
	})
	setRemoteBtnText(remoteBtn)

	var startBtn *widget.Button
	startBtn = widget.NewButton("", func() {
		if IsServerActive() {
			StopServer()
		} else {
			startServerFyne()
		}
		refreshStartButton(startBtn)
	})
	refreshStartButton(startBtn)

	logLabel := widget.NewLabel("")
	logLabel.SetText(getLogText())

	subagentLogLabel := widget.NewLabel("")
	subagentLogLabel.SetText(GetSubagentLogText())

	portEntry := widget.NewEntry()
	portEntry.PlaceHolder = "Port (e.g. 8080)"
	portEntry.SetText(strconv.Itoa(State.GetPort()))
	portEntry.OnChanged = func(_ string) {
		if p, err := strconv.Atoi(portEntry.Text); err == nil {
			State.SetPort(p)
		}
	}

	subagentLLMEntry := widget.NewEntry()
	subagentLLMEntry.PlaceHolder = "Subagent LLM API (e.g. http://localhost:8080/v1)"
	subagentLLMEntry.SetText(State.GetSubagentLLMAPI())
	subagentLLMEntry.OnChanged = func(s string) {
		State.SetSubagentLLMAPI(s)
	}

	return fyneModel{
		toolRows:         toolRows,
		remoteBtn:        remoteBtn,
		startBtn:         startBtn,
		logLabel:         logLabel,
		subagentLogLabel: subagentLogLabel,
		portEntry:        portEntry,
		subagentLLMEntry: subagentLLMEntry,
	}
}

// startServerFyne starts the HTTP server in a goroutine
func startServerFyne() {
	// Optimistically mark the server active before the delayed start so a
	// second click is routed to StopServer() instead of launching a second server.
	setServerActive(true)
	time.Sleep(tools.ServerStartDelayMs * time.Millisecond)
	GlobalLog.AddLine("Starting server...")
	go func() {
		port := State.GetPort()
		mux := http.NewServeMux()
		mux.HandleFunc("/mcp", corsMiddleware(mcpHandler))
		serverMutex.Lock()
		httpServer = &http.Server{ //nolint: exhaustruct_v5
			Addr:    ":" + strconv.Itoa(port),
			Handler: mux,
		}
		serverMutex.Unlock()
		// Bail out if the user stopped the server during the start delay.
		if !IsServerActive() {
			return
		}
		GlobalLog.AddLine("Server started on port " + strconv.Itoa(port))
		if err := httpServer.ListenAndServe(); err != nil {
			setServerActive(false)
			GlobalLog.AddLine("Server start failed: " + err.Error())
		}
	}()
}

// getLogText returns the current log buffer as a string
func getLogText() string {
	return GlobalLog.Tail(tools.TUILogDisplayLines)
}

// ShowToolTUI creates and runs the Fyne GUI for tool configuration
func ShowToolTUI(a fyne.App) {
	w := a.NewWindow("Skynet MCP Server")
	w.SetIcon(icon)
	w.Resize(fyne.NewSize(1000, 900))

	model := buildFyneModel()

	// Top bar: banner image with subtle background
	topBg := canvas.NewRectangle(theme.Color(theme.ColorNameHover))
	banner := canvas.NewImageFromResource(fyne.NewStaticResource("banner.png", bannerImage))
	banner.FillMode = canvas.ImageFillOriginal
	topBar := container.NewPadded(container.NewStack(topBg, banner))

	// Current working directory label under title bar
	cwdDir, _ := os.Getwd()
	cwdLabel := widget.NewLabel(cwdDir)
	configBtn := widget.NewButton("Config", func() {
		showConfigDialog(w)
	})

	cwdRow := container.NewBorder(nil, nil, nil, configBtn, cwdLabel)
	topArea := container.NewVBox(topBar, cwdRow)

	// Left column: one row per tool with MAIN + SUB checkboxes
	leftScroll := container.NewScroll(container.NewVBox(model.toolRows...))
	leftScroll.SetMinSize(fyne.NewSize(320, 0))

	// Right column: Log / Subagent tabs, each with black-background scrolling output
	logScroll := newBlackScroll(model.logLabel)
	subagentScroll := newBlackScroll(model.subagentLogLabel)

	// Tabs on top: Log (works as before) and Subagent (streamed subagent output)
	rightTabs := container.NewAppTabs(
		container.NewTabItem("Log", logScroll),
		container.NewTabItem("Subagent", subagentScroll),
	)

	// Clear Loop History button
	clearHistoryBtn := widget.NewButton("Clear Loop", func() {
		ClearLoopHistory()
		GlobalLog.AddLine("Loop history cleared")
	})

	// Terminate Subagent button
	terminateBtn := widget.NewButton("Terminate Sub", func() {
		if err := subagent.Terminate(); err != nil {
			GlobalLog.AddLine("Terminate: " + err.Error())
		} else {
			GlobalLog.AddLine("Subagent termination requested")
		}
	})

	// Terminate Background Processes button
	bgpTerminateBtn := widget.NewButton("Terminate Bg", func() {
		tools.TerminateAllBackgroundProcesses()
		GlobalLog.AddLine("Background processes terminated")
	})

	portLabel := widget.NewLabel("Server Port:")
	subagentLLMLabel := widget.NewLabel("Subagent LLM API:")

	// Bottom bar: port + subagent LLM API rows (center), remote access (left),
	// clear/start/stop (right). Labels sit to the left of each entry (leading).
	portRow := container.NewBorder(nil, nil, portLabel, nil, model.portEntry)
	subagentRow := container.NewBorder(nil, nil, subagentLLMLabel, nil, model.subagentLLMEntry)
	leftContent := container.NewVBox(portRow, subagentRow)

	// Bottom bar buttons: two rows on the right
	wakeUpBtn := widget.NewButton("Wake Up", func() {
		tools.WakeSleep()
		GlobalLog.AddLine("Wake Up: sleep interrupted")
	})
	topRowBtns := container.New(equalRowLayout{}, clearHistoryBtn, model.remoteBtn, model.startBtn)
	bottomRowBtns := container.New(equalRowLayout{}, wakeUpBtn, terminateBtn, bgpTerminateBtn)
	rightContent := container.NewVBox(topRowBtns, bottomRowBtns)

	bottomBg := canvas.NewRectangle(theme.Color(theme.ColorNameHover))
	bottomContent := container.NewBorder(
		nil,          // top
		nil,          // bottom
		nil,          // leading (left)
		rightContent, // trailing (right)
		leftContent,  // center (expands to fill)
	)
	bottomBar := container.NewPadded(container.NewStack(bottomBg, bottomContent))

	// Left-right split: left column at fixed natural width, right fills remaining space
	w.SetContent(container.NewBorder(
		topArea,    // top
		bottomBar,  // bottom
		leftScroll, // left (sized to content)
		nil,        // right
		rightTabs,  // center (fills remaining space)
	))

	// Start the server automatically
	startServerFyne()

	// Refresh log and subagent displays periodically
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			fyne.Do(func() {
				model.logLabel.SetText(getLogText())
				model.subagentLogLabel.SetText(GetSubagentLogText())
				refreshStartButton(model.startBtn)
				logScroll.ScrollToBottom()
				subagentScroll.ScrollToBottom()
			})
		}
	}()

	// Apply window icon after window is shown (Wayland fix)
	fyne.DoAndWait(func() { w.SetIcon(icon) })

	// Save settings on window close
	w.SetCloseIntercept(func() {
		ticker.Stop()
		tools.TerminateAllBackgroundProcesses()
		saveSettingsFyne()
		w.Close()
	})

	w.ShowAndRun()
}

// saveSettingsFyne saves settings on exit
func saveSettingsFyne() {
	settings := Settings{
		EnabledTools:         make(map[string]bool),
		SubagentEnabledTools: make(map[string]bool),
		Port:                 State.GetPort(),
		RemoteAccess:         State.IsRemoteAccessEnabled(),
		SubagentLLMAPI:       State.GetSubagentLLMAPI(),
		Config:               State.GetConfig(),
	}

	State.mu.RLock()
	maps.Copy(settings.EnabledTools, State.EnabledTools)
	maps.Copy(settings.SubagentEnabledTools, State.SubagentEnabledTools)
	State.mu.RUnlock()

	_ = SaveSettings(settings)
}
