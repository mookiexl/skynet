package tools

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTool("memory_edit", MemoryEditDefinition, MemoryEdit)
}

// MemoryEdit searches and replaces text in an existing memory.
func MemoryEdit(args map[string]any) (any, error) {
	hash, ok := ArgString(args, "hash")
	if !ok || hash == "" {
		return ErrorResult("Hash is required"), nil
	}

	searchText, ok := ArgString(args, "search_text")
	if !ok {
		return ErrorResult("Search text is required"), nil
	}

	replacement, ok := ArgString(args, "replacement")
	if !ok {
		return ErrorResult("Replacement text is required"), nil
	}

	replaceAll, _ := ArgBool(args, "replace_all")

	mem, err := FindMemoryByHash(hash)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	var count int
	var newSize int
	err = ReadTransformWrite(mem.FilePath, func(content string) (string, error) {
		if !strings.Contains(content, searchText) {
			return "", fmt.Errorf("search text not found in memory '%s'", hash)
		}
		c, newContent, err := ApplyReplace(content, searchText, replacement, replaceAll)
		if err != nil {
			return "", err
		}
		count = c
		newSize = len(newContent)
		return newContent, nil
	})
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return map[string]any{
		"success":           true,
		"hash":              hash,
		"name":              strings.TrimSuffix(mem.FileName, ".txt"),
		"replacements_made": count,
		"size":              newSize,
	}, nil
}

// MemoryEditDefinition defines the tool schema.
var MemoryEditDefinition = map[string]any{
	"description": "Search and replace text in an existing memory by hash ID.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash":        map[string]any{"type": "string", "description": "Hash ID of the memory to edit (derived from description)"},
			"search_text": map[string]any{"type": "string", "description": "The exact text to search for"},
			"replacement": map[string]any{"type": "string", "description": "The text to replace with"},
			"replace_all": map[string]any{"type": "boolean", "default": false, "description": "If true, replaces all occurrences. If false, only the first."},
		},
		"required": []string{"hash", "search_text", "replacement"},
	},
}
