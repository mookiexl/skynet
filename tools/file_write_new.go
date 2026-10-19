package tools

import (
	"fmt"
	"os"
)

func init() {
	RegisterTool("file_write_new", FileWriteNewDefinition, FileWriteNew)
}

// FileWriteNew writes content to a new file.
func FileWriteNew(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	content, okC := ArgString(args, "content")
	if !ok || !okC {
		return ErrorResult("Path and content are required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Check if file already exists - error if it does
	if _, err := os.Stat(absPath); err == nil {
		return ErrorResult(fmt.Sprintf("File '%s' already exists. Use 'file_overwrite' to replace it.", path)), nil
	}

	if err := writeFile(absPath, []byte(content)); err != nil {
		return ErrorResult(err.Error()), nil
	}

	return map[string]any{
		"success": true,
	}, nil
}

// FileWriteNewDefinition defines the tool schema.
var FileWriteNewDefinition = map[string]any{
	"description": "Write content to a new file. Fails if file already exists.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "File path relative to current directory"},
			"content": map[string]any{"type": "string", "description": "The text content to write"},
		},
		"required": []string{"path", "content"},
	},
}
