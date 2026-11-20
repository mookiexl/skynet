package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// configField is one configurable setting: its label, a pointer to the
// corresponding field of the runtime Config, and the entry that edits it.
// Every field requires an integer >= 1.
type configField struct {
	label string
	ptr   *int
	entry *widget.Entry
}

// configSection groups related settings under a header.
type configSection struct {
	name   string
	fields []configField
}

// configValueWidth is the fixed width of the numeric value entry in each row.
const configValueWidth = 100.0

// valueRowLayout places a label in the leading (flexible) space and a value
// entry with a fixed width in the trailing space of each settings row.
type valueRowLayout struct{}

func (l valueRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	labelW := size.Width - configValueWidth
	objects[0].Resize(fyne.NewSize(labelW, size.Height))
	objects[0].Move(fyne.NewPos(0, 0))
	objects[1].Resize(fyne.NewSize(configValueWidth, size.Height))
	objects[1].Move(fyne.NewPos(labelW, 0))
}

func (l valueRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := objects[0].MinSize().Height
	if e := objects[1].MinSize().Height; e > h {
		h = e
	}
	return fyne.NewSize(objects[0].MinSize().Width+configValueWidth, h)
}

// showConfigDialog opens the Config popup from the top-bar Config button.
// It edits the 12 runtime limits; OK validates, applies them to the tools
// and subagent packages, persists them to .skynet.yml, and closes the
// dialog. Cancel closes without changes.
func showConfigDialog(w fyne.Window) {
	cfg := State.GetConfig()

	sections := []configSection{
		{
			name: "General",
			fields: []configField{
				{"Max response length (chars)", &cfg.MaxTextLimit, nil},
				{"Shell auto-BG timeout (sec)", &cfg.ShellTimeLimitSeconds, nil},
				{"Max rollback size (MB)", &cfg.MaxRollbackSizeMB, nil},
				{"Loop History", &cfg.MaxLoopHistory, nil},
				{"Max Sleep (sec)", &cfg.MaxSleepSeconds, nil},
			},
		},
		{
			name: "Ripgrep",
			fields: []configField{
				{"Default result count", &cfg.RipgrepMaxResults, nil},
			},
		},
		{
			name: "Web search",
			fields: []configField{
				{"Max characters per page", &cfg.ExaMaxCharacters, nil},
			},
		},
		{
			name: "Subagent",
			fields: []configField{
				{"Request timeout (sec)", &cfg.SubagentRequestTimeoutSeconds, nil},
				{"Deadline (sec)", &cfg.SubagentDeadlineSeconds, nil},
				{"Turn limit", &cfg.SubagentMaxIterations, nil},
				{"Think budget (tok)", &cfg.SubagentThinkingBudgetTokens, nil},
			},
		},
		{
			name: "Multimodal",
			fields: []configField{
				{"Max image size (MB)", &cfg.MaxImageSizeMB, nil},
				{"Max image dimension (px)", &cfg.MaxImageDimension, nil},
			},
		},
	}

	fields := make([]configField, 0, 16)
	rows := make([]fyne.CanvasObject, 0, 16)
	for _, sec := range sections {
		rows = append(rows, widget.NewLabelWithStyle(sec.name, 0, fyne.TextStyle{Bold: true})) //nolint:exhaustruct_v5
		for i := range sec.fields {
			f := &sec.fields[i]
			f.entry = widget.NewEntry()
			f.entry.SetText(strconv.Itoa(*f.ptr))
			fields = append(fields, *f)
			rows = append(rows, container.New(valueRowLayout{}, widget.NewLabel(f.label), f.entry))
		}
	}

	// Validation error, shown above the OK/Cancel buttons.
	errSeg := &widget.TextSegment{
		Text:  "",
		Style: widget.RichTextStyle{ColorName: theme.ColorNameError, Inline: true}, //nolint:exhaustruct_v5
	}
	errLabel := widget.NewRichText()
	errLabel.Segments = []widget.RichTextSegment{errSeg}
	setErr := func(text string) {
		errSeg.Text = text
		errLabel.Refresh()
	}

	content := container.NewPadded(container.NewVBox(append(rows, errLabel)...))
	dlg := dialog.NewCustomConfirm("Config", "OK", "Cancel", content, func(ok bool) {
		if !ok {
			GlobalLog.AddLine("Config dialog: cancelled")
			return
		}
		setErr("")
		for _, f := range fields {
			v, err := strconv.Atoi(strings.TrimSpace(f.entry.Text))
			if err != nil || v < 1 {
				setErr(fmt.Sprintf("Invalid value for %s: must be an integer >= 1", f.label))
				return
			}
			*f.ptr = v
		}
		State.SetConfig(cfg)
		applyToolsConfig(cfg)
		subagentMgr.SetLimits(subagentLimits(cfg))
		saveSettingsFyne()
		GlobalLog.AddLine("Config dialog: saved")
	}, w)
	dlg.Show()
}
