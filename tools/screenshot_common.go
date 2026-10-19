//go:build unix

package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// newTempScreenshot creates a temporary file path for screenshots and removes any existing file.
func newTempScreenshot() string {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("skynet_screenshot_%d.png", time.Now().UnixNano()))
	_ = os.Remove(tmpFile)
	return tmpFile
}

// runCapture executes the capture command and reads the resulting file.
func runCapture(cmd *exec.Cmd, tmpFile string) ([]byte, error) {
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot: %w", err)
	}

	return data, nil
}

// takeScreenshotKDE takes a screenshot using Spectacle on KDE Wayland.
func takeScreenshotKDE() ([]byte, error) {
	// Check if spectacle is available
	if _, err := exec.LookPath("spectacle"); err != nil {
		return nil, fmt.Errorf("spectacle not found. Install with: sudo apt-get install kde-spectacle")
	}

	tmpFile := newTempScreenshot()
	cmd := exec.Command("spectacle", "--activewindow", "-b", "0", "-w", "500", "-o", tmpFile)
	return runCapture(cmd, tmpFile)
}

// takeScreenshotWayland takes a screenshot using grim on wlroots-based Wayland compositors.
func takeScreenshotWayland() ([]byte, error) {
	// Check if grim is available
	if _, err := exec.LookPath("grim"); err != nil {
		return nil, fmt.Errorf("grim not found. Install with: sudo apt-get install grim")
	}

	tmpFile := newTempScreenshot()
	cmd := exec.Command("grim", tmpFile)

	return runCapture(cmd, tmpFile)
}

// takeScreenshotFallback uses scrot or import for X11 screenshot.
func takeScreenshotFallback() ([]byte, error) {
	// Try scrot first
	if _, err := exec.LookPath("scrot"); err == nil {
		tmpFile := newTempScreenshot()
		cmd := exec.Command("scrot", tmpFile)
		return runCapture(cmd, tmpFile)
	}

	// Try ImageMagick import
	if _, err := exec.LookPath("import"); err != nil {
		return nil, fmt.Errorf("no screenshot tool found. Install grim, scrot, or ImageMagick")
	}

	tmpFile := newTempScreenshot()
	cmd := exec.Command("import", "-window", "root", tmpFile)
	return runCapture(cmd, tmpFile)
}

// takeScreenshot captures a screenshot using the best available tool for the current display server.
func takeScreenshot() ([]byte, error) {
	// Detect display server type
	isWayland := os.Getenv("XDG_SESSION_TYPE") == "wayland" ||
		os.Getenv("WAYLAND_DISPLAY") != ""

	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	isKDE := strings.Contains(desktop, "KDE")

	if isWayland {
		if isKDE {
			return takeScreenshotKDE()
		}
		return takeScreenshotWayland()
	}

	// X11 fallback - check grim once
	if _, err := exec.LookPath("grim"); err == nil {
		return takeScreenshotWayland()
	}
	return takeScreenshotFallback()
}

// buildScreenshotResult builds the result map with the screenshot data.
func buildScreenshotResult(screenshotData []byte, command string, delaySeconds int) any {
	// Optionally resize image
	resized, err := limitImageSize(screenshotData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error processing image size: %v", err))
	}

	base64Data := base64.StdEncoding.EncodeToString(resized)

	// Build note
	note := fmt.Sprintf("Screenshot taken %d seconds after command execution", delaySeconds)
	if len(screenshotData) != len(resized) {
		note += fmt.Sprintf(" (resized from %s to %s)", FormatFileSize(len(screenshotData)), FormatFileSize(len(resized)))
	}

	return map[string]any{
		"size":       len(resized),
		"size_str":   FormatFileSize(len(resized)),
		"data":       base64Data,
		"media_type": "image/png",
		"is_image":   true,
		"command":    command,
		"delay_s":    delaySeconds,
		"note":       note,
	}
}
