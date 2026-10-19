package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// MemoryDirMaxDescriptionLen is the max length for memory descriptions.
	MemoryDirMaxDescriptionLen = 300
)

func init() {
	RegisterTool("memory_write_new", MemoryWriteNewDefinition, MemoryWriteNew)
}

// MemoryWriteNew adds a new memory entry.
func MemoryWriteNew(args map[string]any) (any, error) {
	description, ok := ArgString(args, "description")
	if !ok || description == "" {
		return ErrorResult("Description is required"), nil
	}

	content, ok := ArgString(args, "content")
	if !ok {
		content = ""
	}

	// Truncate description and content if needed
	if len(description) > MemoryDirMaxDescriptionLen {
		description = description[:MemoryDirMaxDescriptionLen]
	}

	if len(content) > MaxTextLimit {
		content = content[:MaxTextLimit]
	}

	// Create memory directory if it doesn't exist
	memoryDir := ".memory"
	if err := os.MkdirAll(memoryDir, FileModeDirectory); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create memory directory: %v", err)), nil
	}

	// Generate filename: slug + hash
	hash := MemoryHash(description)
	fileName := CreateSlug(description) + "_" + hash + ".txt"
	filePath := filepath.Join(memoryDir, fileName)

	// Check if memory already exists (overwrite not allowed)
	if _, err := os.Stat(filePath); err == nil {
		return map[string]any{
			"error":     "Memory with this description already exists. Use memory_edit or memory_overwrite to modify it.",
			"exists":    true,
			"file_name": fileName,
		}, nil
	}

	// Build file content: description on first line, then content
	fileContent := description
	if content != "" {
		fileContent += "\n" + content
	}

	if err := AtomicWriteFile(filePath, []byte(fileContent)); err != nil {
		return ErrorResult(fmt.Sprintf("Error writing memory: %v", err)), nil
	}

	return map[string]any{
		"success":     true,
		"description": description,
		"hash":        hash,
		"size":        len(content),
	}, nil
}

// MemoryWriteNewDefinition defines the tool schema.
var MemoryWriteNewDefinition = map[string]any{
	"description": "Add a new memory. Overwrite not allowed - use memory_edit or memory_overwrite to modify existing memories.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{"type": "string", "description": "Brief one-line description (truncated to 300 characters)"},
			"content":     map[string]any{"type": "string", "description": fmt.Sprintf("Memory content (truncated to %d characters)", MaxTextLimit)},
		},
		"required": []string{"description"},
	},
}
