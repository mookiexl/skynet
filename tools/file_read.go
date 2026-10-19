package tools

import (
	"fmt"
	"os"
)

func init() {
	RegisterTool("file_read", ReadFileDefinition, ReadFile)
}

// ReadFile reads the contents of a file.
func ReadFile(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return ErrorResult("File path is required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error reading file: %v", err)), nil
	}

	contentStr := string(data)

	// Apply truncation after formatting
	if len(contentStr) > MaxTextLimit {
		contentStr = contentStr[:MaxTextLimit]

		result := map[string]any{
			"content":            contentStr,
			"size":               len(data),
			"truncation_warning": fmt.Sprintf("File was truncated to %d characters. Total file size: %d bytes.", MaxTextLimit, len(data)),
		}
		return result, nil
	}

	result := map[string]any{
		"content": contentStr,
	}

	return result, nil
}

// ReadFileDefinition defines the tool schema.
var ReadFileDefinition = map[string]any{
	"description": "Read contents of a file.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path relative to current directory"},
		},
		"required": []string{"path"},
	},
}
