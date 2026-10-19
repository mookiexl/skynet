package tools

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	RegisterTool("file_read_from_line", ReadFromFileDefinition, ReadFromFile)
}

// ReadFromFile reads specific lines from a file.
func ReadFromFile(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return ErrorResult("File path is required"), nil
	}

	startLine, ok := ArgFloat(args, "start_line")
	if !ok {
		return ErrorResult("Start line number is required"), nil
	}

	numLines, ok := ArgFloat(args, "num_lines")
	if !ok {
		numLines = DefaultNumLines
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error reading file: %v", err)), nil
	}

	lines := strings.Split(string(data), "\n")

	startIdx := max(0, int(startLine)-1)
	if startIdx >= len(lines) {
		return ErrorResult(fmt.Sprintf("Start line %d exceeds file length (total lines: %d)", int(startLine), len(lines))), nil
	}

	endIdx := min(len(lines), startIdx+int(numLines))

	selectedLines := lines[startIdx:endIdx]

	// Check for line numbers parameter
	withLineNumbers, _ := ArgBool(args, "show_line_numbers")

	// Format output with or without line numbers
	var contentStr string
	if withLineNumbers {
		var numberedLines []string
		for i, line := range selectedLines {
			lineNum := startIdx + i + 1 // 1-indexed line number
			numberedLines = append(numberedLines, fmt.Sprintf("%d: %s", lineNum, line))
		}
		contentStr = strings.Join(numberedLines, "\n")
	} else {
		contentStr = strings.Join(selectedLines, "\n")
	}

	// Build result
	result := map[string]any{
		"content":             contentStr,
		"start_line":          int(startLine),
		"num_lines_returned":  len(selectedLines),
		"total_lines_in_file": len(lines),
	}

	if len(contentStr) > MaxTextLimit {
		result["truncation_warning"] = fmt.Sprintf("Output truncated to %d characters.", MaxTextLimit)
		result["content"] = contentStr[:MaxTextLimit]
		return result, nil
	}

	if int(numLines) > len(selectedLines) {
		result["truncation_warning"] = fmt.Sprintf("Reached end of file. Returned %d lines (from line %d). Total lines in file: %d.", len(selectedLines), int(startLine), len(lines))
	}

	return result, nil
}

// ReadFromFileDefinition defines the tool schema.
var ReadFromFileDefinition = map[string]any{
	"description": "Read specific lines from a file starting at a given line number.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":              map[string]any{"type": "string", "description": "File path relative to current directory"},
			"start_line":        map[string]any{"type": "integer", "description": "Starting line number (1-indexed)"},
			"num_lines":         map[string]any{"type": "integer", "default": DefaultNumLines, "description": "Number of lines to read (default: 50)"},
			"show_line_numbers": map[string]any{"type": "boolean", "description": "Include line numbers"},
		},
		"required": []string{"path", "start_line", "show_line_numbers"},
	},
}
