package tools

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	RegisterTool("file_find_replace", FindAndReplaceDefinition, FindAndReplace)
}

// FindAndReplace implements the tool handler for the FindAndReplace tool.

func parseFindAndReplaceArgs(args map[string]any) (string, string, string, bool, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return "", "", "", false, fmt.Errorf("path required")
	}
	searchText, ok := ArgString(args, "find")
	if !ok || searchText == "" {
		return "", "", "", false, fmt.Errorf("search text is required and must be non-empty")
	}
	replacement, ok := ArgString(args, "replace_with")
	if !ok {
		return "", "", "", false, fmt.Errorf("replacement text required")
	}
	replaceAll, ok := ArgBool(args, "replace_all")
	if !ok {
		return "", "", "", false, fmt.Errorf("replace_all is required")
	}
	return path, searchText, replacement, replaceAll, nil
}

// FindAndReplace finds and replaces text in a file.
func FindAndReplace(args map[string]any) (any, error) {
	path, searchText, replacement, replaceAll, err := parseFindAndReplaceArgs(args)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	if searchText == replacement {
		return ErrorResult("Search text and replacement are identical, nothing to replace"), nil
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
	if !strings.Contains(content, searchText) {
		return ErrorResult("search text not found in file"), nil
	}

	replacementsMade, newContent, err := ApplyReplace(content, searchText, replacement, replaceAll)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Only create a rollback entry after the transformation succeeded.
	// Pass the already-read content so the snapshot matches exactly what
	// was read (no redundant read / TOCTOU gap).
	hash, reason := CreateRollbackEntryFromContent(relPath, data)

	if err := AtomicWriteFile(absPath, []byte(newContent)); err != nil {
		return ErrorResult(err.Error()), nil
	}

	resp := map[string]any{
		"success":           true,
		"replacements_made": replacementsMade,
	}
	switch reason {
	case RollbackCreated:
		resp["rollback"] = hash
	case RollbackUnavailable:
		resp["rollback"] = RollbackUnavailable
	}
	return resp, nil
}

// FindAndReplaceDefinition defines the tool schema.
var FindAndReplaceDefinition = map[string]any{
	"description": "Find and replace specific text in a file with another string or nothing.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":         map[string]any{"type": "string", "description": "File path relative to current directory"},
			"find":         map[string]any{"type": "string", "description": "Exact text to search for"},
			"replace_with": map[string]any{"type": "string", "description": "The text to replace the match with. Leave blank to delete text."},
			"replace_all":  map[string]any{"type": "boolean", "description": "Replace all occurrences of search string. Use with caution!"},
		},
		"required": []string{"path", "find", "replace_with", "replace_all"},
	},
}
