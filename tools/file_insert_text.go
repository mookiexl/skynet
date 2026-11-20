package tools

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

func init() {
	RegisterTool("file_insert_text", FileInsertTextDefinition, ExecuteFileInsertText)
}

// FileInsertTextDefinition defines the tool schema.
var FileInsertTextDefinition = map[string]any{
	"description": "Inserts text before or after a specific string anchor.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path to the file",
			},
			"anchor_text": map[string]any{
				"type":        "string",
				"description": "The anchor string. Not deleted.",
			},
			"text_to_insert": map[string]any{
				"type":        "string",
				"description": "The text to be inserted. Don't repeat anchor!",
			},
			"insert_before": map[string]any{
				"type":        "boolean",
				"description": "If true, text is inserted before the anchor. If false, after.",
			},
			"insert_all": map[string]any{
				"type":        "boolean",
				"description": "If true, inserts relative to every occurrence of the anchor. Use with caution.",
			},
			"auto_newline": map[string]any{
				"type":        "boolean",
				"default":     true,
				"description": "Enabled by default, disable ONLY when inserting in the middle of a line",
			},
		},
		"required": []string{"path", "anchor_text", "text_to_insert", "insert_before", "insert_all"},
	},
}

// buildInsertString determines the final insert string, conditionally adding
// newlines at the boundaries only if they aren't already present.
func buildInsertString(content string, pos int, insert string, addNewlines bool) string {
	if !addNewlines {
		return insert
	}

	leadingNL := "\n"
	if pos > 0 && content[pos-1] == '\n' {
		leadingNL = ""
	}

	trailingNL := "\n"
	if pos < len(content) && content[pos] == '\n' {
		trailingNL = ""
	}

	return leadingNL + insert + trailingNL
}

// findAnchorIndices returns all starting indices of anchor in content.
func findAnchorIndices(content, anchor string) []int {
	var indices []int
	start := 0
	for {
		idx := strings.Index(content[start:], anchor)
		if idx == -1 {
			return indices
		}
		indices = append(indices, start+idx)
		start += idx + 1
	}
}

// insertAt performs a single insertion at the given position.
func insertAt(content string, pos int, insertStr string) string {
	return content[:pos] + insertStr + content[pos:]
}

// insertTextAroundAnchor inserts the text before or after the anchor in content.
func insertTextAroundAnchor(content, anchor, insert string, insertBefore, insertAll, addNewlines bool) (string, int, error) {
	if !strings.Contains(content, anchor) {
		return "", 0, fmt.Errorf("anchor text '%s' not found in file", anchor)
	}
	count := strings.Count(content, anchor)
	if !insertAll && count > 1 {
		return "", count, fmt.Errorf("multiple occurrences (%d) of anchor text found. Set 'insert_all' to true to insert relative to all or refine your anchor", count)
	}

	indices := findAnchorIndices(content, anchor)
	if !insertAll {
		indices = indices[:1]
	}

	// Process from last to first to preserve earlier indices
	for i := range slices.Backward(indices) {
		idx := indices[i]
		pos := idx
		if !insertBefore {
			pos = idx + len(anchor)
		}
		insertStr := buildInsertString(content, pos, insert, addNewlines)
		content = insertAt(content, pos, insertStr)
	}

	return content, count, nil
}

// ExecuteFileInsertText finds the anchor_text and inserts text_to_insert before or after it.
func ExecuteFileInsertText(args map[string]any) (any, error) {
	path, ok1 := ArgString(args, "path")
	anchor, ok2 := ArgString(args, "anchor_text")
	insert, ok3 := ArgString(args, "text_to_insert")
	insertBefore, ok4 := ArgBool(args, "insert_before")
	insertAll, ok5 := ArgBool(args, "insert_all")
	addNewlines, ok := ArgBool(args, "auto_newline")
	if !ok {
		addNewlines = true // schema default
	}

	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return ErrorResult("missing required arguments: path, anchor_text, text_to_insert, insert_before, insert_all are required"), nil
	}
	if anchor == "" {
		return ErrorResult("anchor_text must be non-empty"), nil
	}

	validatedPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	relPath, err := relToWorkDir(validatedPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	data, err := os.ReadFile(validatedPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error reading file: %v", err)), nil
	}

	content := string(data)
	var count int
	newContent, count, err := insertTextAroundAnchor(content, anchor, insert, insertBefore, insertAll, addNewlines)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Only create a rollback entry after the transformation succeeded.
	// Pass the already-read content so the snapshot matches exactly what
	// was read (no redundant read / TOCTOU gap).
	hash, reason := CreateRollbackEntryFromContent(relPath, data)

	if err := AtomicWriteFile(validatedPath, []byte(newContent)); err != nil {
		return ErrorResult(err.Error()), nil
	}

	direction := "after"
	if insertBefore {
		direction = "before"
	}
	resp := map[string]any{"success": true, "message": fmt.Sprintf("Inserted %s %d anchor occurrence(s)", direction, count)}
	switch reason {
	case RollbackCreated:
		resp["rollback"] = hash
	case RollbackUnavailable:
		resp["rollback"] = RollbackUnavailable
	}
	return resp, nil
}
