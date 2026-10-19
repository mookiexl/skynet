package tools

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	RegisterTool("memory_delete", MemoryDeleteDefinition, MemoryDelete)
}

//
// MemoryDelete implements the tool handler for the MemoryDelete tool.

// MemoryDelete removes a memory entry by its hash ID.
func MemoryDelete(args map[string]any) (any, error) {
	hash, ok := ArgString(args, "hash")
	if !ok || hash == "" {
		return ErrorResult("Hash is required"), nil
	}

	mem, err := FindMemoryByHash(hash)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Delete the file
	if err := os.Remove(mem.FilePath); err != nil {
		return ErrorResult(fmt.Sprintf("Error deleting memory: %v", err)), nil
	}

	name := strings.TrimSuffix(mem.FileName, ".txt")
	return map[string]any{
		"success": true,
		"hash":    hash,
		"name":    name,
		"message": fmt.Sprintf("Memory '%s' deleted successfully", name),
	}, nil
}

// MemoryDeleteDefinition defines the tool schema.
var MemoryDeleteDefinition = map[string]any{
	"description": "Delete a memory by hash ID.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{"type": "string", "description": "Hash ID of the memory to delete (derived from description)"},
		},
		"required": []string{"hash"},
	},
}
