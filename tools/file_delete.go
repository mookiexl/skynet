package tools

import (
	"fmt"
	"os"
)

func init() {
	RegisterTool("file_delete", DeleteFileDefinition, DeleteFile)
}

// DeleteFile removes the file at the specified path.
func DeleteFile(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok {
		return ErrorResult("Path is required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	relPath, err := relToWorkDir(absPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	hash, reason := CreateRollbackEntry(absPath, relPath)

	if err := os.Remove(absPath); err != nil {
		return ErrorResult(fmt.Sprintf("Error deleting file: %v", err)), nil
	}

	resp := map[string]any{
		"success": true,
	}
	switch reason {
	case RollbackCreated:
		resp["rollback"] = hash
	case RollbackUnavailable:
		resp["rollback"] = RollbackUnavailable
	}
	return resp, nil
}

// DeleteFileDefinition defines the tool schema.
var DeleteFileDefinition = map[string]any{
	"description": "Delete a file.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path relative to current directory"},
		},
		"required": []string{"path"},
	},
}
