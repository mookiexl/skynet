// Package tools provides tool handlers for file, memory, and expression operations.
package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func init() {
	RegisterTool("ripgrep", RipgrepDefinition, ExecuteRipgrep)
}

// RipgrepDefinition defines the tool schema.
var RipgrepDefinition = map[string]any{
	"description": "Search file contents. Only works inside the working directory.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The search pattern",
			},
			"path": map[string]any{
				"type":        "string",
				"default":     ".",
				"description": "Where to search",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Pattern for filenames",
			},
			"case_sensitive": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Case sensitive search (default: smart case)",
			},
			"max_depth": map[string]any{
				"type":        "integer",
				"description": "Maximum depth",
			},
			"count": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Only output the count of matches",
			},
			"hidden": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Search hidden",
			},
			"text": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Search binary files",
			},
			"context_before": map[string]any{
				"type":        "integer",
				"default":     0,
				"description": "Context lines before each match",
			},
			"context_after": map[string]any{
				"type":        "integer",
				"default":     0,
				"description": "Context lines after each match",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"default":     20,
				"description": "Maximum number of matches to return",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "File type to search",
			},
			"word_match": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Match whole words only",
			},
			"regex": map[string]any{
				"type":        "boolean",
				"default":     true,
				"description": "Pattern is regex, or plain text",
			},
		},
		"required": []string{"pattern"},
	},
}

type ripgrepArgs struct {
	pattern       string
	path          string
	glob          string
	caseSensitive bool
	maxDepth      int
	count         bool
	hidden        bool
	text          bool
	contextBefore int
	contextAfter  int
	maxResults    int
	fileType      string
	wordMatch     bool
	regex         bool
}

func parseRipgrepArgs(args map[string]any) ripgrepArgs {
	pattern, _ := ArgString(args, "pattern")
	path, _ := ArgString(args, "path")
	if path == "" {
		path = "."
	}
	glob, _ := ArgString(args, "glob")
	caseSensitive, _ := ArgBool(args, "case_sensitive")
	count, _ := ArgBool(args, "count")
	hidden, _ := ArgBool(args, "hidden")
	text, _ := ArgBool(args, "text")
	wordMatch, _ := ArgBool(args, "word_match")
	regex, _ := ArgBool(args, "regex")

	maxDepth := -1
	if f, ok := ArgFloat(args, "max_depth"); ok {
		maxDepth = int(f)
	}

	contextBefore := 0
	if f, ok := ArgFloat(args, "context_before"); ok {
		contextBefore = int(f)
	}

	contextAfter := 0
	if f, ok := ArgFloat(args, "context_after"); ok {
		contextAfter = int(f)
	}

	maxResults := 0
	if f, ok := ArgFloat(args, "max_results"); ok {
		maxResults = int(f)
	}

	fileType, _ := ArgString(args, "type")

	return ripgrepArgs{
		pattern:       pattern,
		path:          path,
		glob:          glob,
		caseSensitive: caseSensitive,
		maxDepth:      maxDepth,
		count:         count,
		hidden:        hidden,
		text:          text,
		contextBefore: contextBefore,
		contextAfter:  contextAfter,
		maxResults:    maxResults,
		fileType:      fileType,
		wordMatch:     wordMatch,
		regex:         regex,
	}
}

func buildRipgrepCmd(args ripgrepArgs) ([]string, error) {
	if args.pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	cmdArgs := []string{}

	// Case sensitivity
	if args.caseSensitive {
		cmdArgs = append(cmdArgs, "--case-sensitive")
	}

	// Literal (fixed string) search unless regex is explicitly requested.
	// The regex parameter is required, so no pattern sniffing is needed.
	if !args.regex {
		cmdArgs = append(cmdArgs, "-F")
	}

	// Word match
	if args.wordMatch {
		cmdArgs = append(cmdArgs, "-w")
	}

	// Count mode
	if args.count {
		cmdArgs = append(cmdArgs, "-c")
	}

	// Hidden files
	if args.hidden {
		cmdArgs = append(cmdArgs, "-H")
	}

	// Binary files as text
	if args.text {
		cmdArgs = append(cmdArgs, "-a")
	}

	// Max depth
	if args.maxDepth > 0 {
		cmdArgs = append(cmdArgs, "--max-depth", fmt.Sprintf("%d", args.maxDepth))
	}

	// Context lines
	if args.contextBefore > 0 {
		cmdArgs = append(cmdArgs, "-B", fmt.Sprintf("%d", args.contextBefore))
	}
	if args.contextAfter > 0 {
		cmdArgs = append(cmdArgs, "-A", fmt.Sprintf("%d", args.contextAfter))
	}

	// Max results
	if args.maxResults > 0 {
		cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", args.maxResults))
	}

	// Glob pattern
	if args.glob != "" {
		cmdArgs = append(cmdArgs, "--glob", args.glob)
	}

	// File type
	if args.fileType != "" {
		cmdArgs = append(cmdArgs, "--type", args.fileType)
	}

	// Pattern and path
	cmdArgs = append(cmdArgs, args.pattern)
	if args.path != "." {
		cmdArgs = append(cmdArgs, args.path)
	}

	return cmdArgs, nil
}

func runSearch(cmdName string, cmdArgs []string) (string, string, error) {
	cmd := exec.Command(cmdName, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// ripgrepInstallHint returns platform-specific instructions for installing ripgrep,
// worded so an AI agent can act on them.
func ripgrepInstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "winget install BurntSushi.ripgrep (or: scoop install ripgrep, choco install ripgrep)"
	case "darwin":
		return "brew install ripgrep"
	default:
		return "sudo apt install ripgrep (Debian/Ubuntu) | sudo dnf install ripgrep (Fedora) | sudo pacman -S ripgrep (Arch) | brew install ripgrep"
	}
}

// ExecuteRipgrep searches file contents using ripgrep.
func ExecuteRipgrep(args map[string]any) (any, error) {
	parsedArgs := parseRipgrepArgs(args)
	cmdArgs, err := buildRipgrepCmd(parsedArgs)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Check that ripgrep is installed before attempting to run it.
	if _, lookErr := exec.LookPath("rg"); lookErr != nil {
		return map[string]any{
			"error":     fmt.Sprintf("ripgrep (rg) is not installed or not in PATH: %v. Install it with: %s", lookErr, ripgrepInstallHint()),
			"exit_code": -1,
		}, nil
	}

	stdout, stderr, err := runSearch("rg", cmdArgs)
	stdoutTruncated, stdoutWasTruncated := TruncateOutput(stdout)
	stderrTruncated, stderrWasTruncated := TruncateOutput(stderr)

	result := map[string]any{
		"stdout": strings.TrimRight(stdoutTruncated, "\n"),
	}

	if err != nil {
		result["exit_code"] = -1
	}

	if stderrTruncated != "" {
		result["stderr"] = strings.TrimRight(stderrTruncated, "\n")
	}

	// Add truncation warning if output was truncated
	if stdoutWasTruncated || stderrWasTruncated {
		warning := fmt.Sprintf("Output was truncated to %d characters.", MaxTextLimit)
		if stdoutWasTruncated {
			warning += fmt.Sprintf(" stdout total length: %d chars.", len(stdout))
		}
		if stderrWasTruncated {
			warning += fmt.Sprintf(" stderr total length: %d chars.", len(stderr))
		}
		result["truncation_warning"] = warning
	}

	return result, nil
}
