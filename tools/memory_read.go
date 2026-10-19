package tools

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTool("memory_read", MemoryReadDefinition, MemoryRead)
}

//
// MemoryRead implements the tool handler for the MemoryRead tool.

// MemoryRead retrieves a memory entry by its hash ID.
func MemoryRead(args map[string]any) (any, error) {
	hash, ok := ArgString(args, "hash")
	if !ok || hash == "" {
		return ErrorResult("Hash is required"), nil
	}

	mem, err := FindMemoryByHash(hash)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Get actual content (everything after first line)
	actualContent := mem.Content
	lines := strings.SplitN(mem.Content, "\n", 2)
	if len(lines) > 1 {
		actualContent = lines[1]
	}

	if len(actualContent) > MaxTextLimit {
		actualContent = actualContent[:MaxTextLimit]
		return map[string]any{
			"success":            true,
			"hash":               hash,
			"name":               strings.TrimSuffix(mem.FileName, ".txt"),
			"description":        mem.Description,
			"content":            actualContent,
			"size":               mem.Size,
			"truncation_warning": fmt.Sprintf("Memory content truncated to %d characters.", MaxTextLimit),
		}, nil
	}

	return map[string]any{
		"success":     true,
		"hash":        hash,
		"name":        strings.TrimSuffix(mem.FileName, ".txt"),
		"description": mem.Description,
		"content":     actualContent,
		"size":        mem.Size,
	}, nil
}

// MemoryReadDefinition defines the tool schema.
var MemoryReadDefinition = map[string]any{
	"description": "Read a memory by hash ID.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{"type": "string", "description": "Hash ID of the memory to read (derived from description)"},
		},
		"required": []string{"hash"},
	},
}
