package tools

import (
	"fmt"
	"os"
)

func init() {
	RegisterTool("make_directory", MakeDirectoryDefinition, MakeDirectory)
}

//
// MakeDirectory implements the tool handler for the MakeDirectory tool.

// MakeDirectory creates a new directory.
func MakeDirectory(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return ErrorResult("path is required"), nil
	}

	parents, _ := ArgBool(args, "parents")

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	if parents {
		err = os.MkdirAll(absPath, FileModeDirectory)
	} else {
		err = os.Mkdir(absPath, FileModeDirectory)
	}

	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return map[string]any{
		"absolute_path": absPath,
		"parents":       parents,
		"message":       fmt.Sprintf("Directory '%s' created successfully", path),
	}, nil
}

// MakeDirectoryDefinition defines the tool schema.
var MakeDirectoryDefinition = map[string]any{
	"description": "Create a new directory.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Directory path to create"},
			"parents": map[string]any{"type": "boolean", "default": false, "description": "Create parent directories if needed (like mkdir -p)"},
		},
		"required": []string{"path"},
	},
}
