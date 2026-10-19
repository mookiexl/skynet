package tools

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	RegisterTool("file_append", FileAppendDefinition, FileAppend)
}

// FileAppend appends text to the end of a file.
func FileAppend(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return ErrorResult("Path is required"), nil
	}

	appendText, ok := ArgString(args, "text")
	if !ok {
		return ErrorResult("Text to append is required"), nil
	}

	addNewlines, ok := ArgBool(args, "add_newlines")
	if !ok {
		return ErrorResult("add_newlines is required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	relPath, err := relToWorkDir(absPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error reading file: %v", err)), nil
	}

	content := string(data)
	if addNewlines && content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += appendText

	// Only create a rollback entry after the transformation succeeded.
	// Pass the already-read content so the snapshot matches exactly what
	// was read (no redundant read / TOCTOU gap).
	hash, reason := CreateRollbackEntryFromContent(relPath, data)

	if err := AtomicWriteFile(absPath, []byte(content)); err != nil {
		return ErrorResult(err.Error()), nil
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

// FileAppendDefinition defines the tool schema.
var FileAppendDefinition = map[string]any{
	"description": "Append text to the end of a file.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path relative to current directory"},
			"text": map[string]any{"type": "string", "description": "Text to append to the end of the file"},
			"add_newlines": map[string]any{
				"type":        "boolean",
				"description": "Automatic newline before and after the inserted text, if one isn't already present.",
			},
		},
		"required": []string{"path", "text", "add_newlines"},
	},
}
