package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	RegisterTool("memory_overwrite", MemoryOverwriteDefinition, MemoryOverwrite)
}

// MemoryOverwrite replaces the content of an existing memory.
func MemoryOverwrite(args map[string]any) (any, error) {
	hash, content, err := parseMemoryOverwriteArgs(args)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	mem, err := FindMemoryByHash(hash)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Optional new description
	finalDescription := mem.Description
	if newDesc, ok := ArgString(args, "description"); ok && newDesc != "" {
		finalDescription = newDesc
	}

	// Truncate content to the maximum allowed limit
	if len(content) > MaxTextLimit {
		content = content[:MaxTextLimit]
	}

	// If description changes, the filename and hash change
	if finalDescription != mem.Description {
		result, err := overwriteMemoryWithNewHash(mem, finalDescription, content)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}
		return result, nil
	}

	result, err := overwriteMemoryInPlace(mem, hash, content)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	return result, nil
}

func parseMemoryOverwriteArgs(args map[string]any) (string, string, error) {
	hash, ok := ArgString(args, "hash")
	if !ok || hash == "" {
		return "", "", fmt.Errorf("hash is required")
	}
	content, ok := ArgString(args, "content")
	if !ok {
		return "", "", fmt.Errorf("content is required")
	}
	return hash, content, nil
}

func overwriteMemoryWithNewHash(mem *MemoryFileInfo, finalDescription, content string) (map[string]any, error) {
	newHash := MemoryHash(finalDescription)
	slug := CreateSlug(finalDescription)
	newFileName := slug + "_" + newHash + ".txt"
	newFilePath := filepath.Join(".memory", newFileName)

	newFileContent := mem.Content
	if content != "" {
		newFileContent = finalDescription + "\n" + content
	}

	if err := AtomicWriteFile(newFilePath, []byte(newFileContent)); err != nil {
		return nil, fmt.Errorf("error writing new memory: %w", err)
	}

	if err := os.Remove(mem.FilePath); err != nil {
		return nil, fmt.Errorf("error removing old memory: %w", err)
	}

	return map[string]any{
		"success":         true,
		"hash_changed_to": newHash,
		"name":            strings.TrimSuffix(newFileName, ".txt"),
		"size":            len(newFileContent),
	}, nil
}

func overwriteMemoryInPlace(mem *MemoryFileInfo, hash, content string) (map[string]any, error) {
	newContent := mem.Content
	if content != "" {
		newContent = mem.Description + "\n" + content
	}

	if err := AtomicWriteFile(mem.FilePath, []byte(newContent)); err != nil {
		return nil, fmt.Errorf("error updating memory: %w", err)
	}

	return map[string]any{
		"success": true,
		"hash":    hash,
		"name":    strings.TrimSuffix(mem.FileName, ".txt"),
		"size":    len(newContent),
	}, nil
}

// MemoryOverwriteDefinition defines the tool schema.
var MemoryOverwriteDefinition = map[string]any{
	"description": "Completely replace the content of an existing memory by hash ID. Optionally update the description.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash":        map[string]any{"type": "string", "description": "Hash ID of the memory to overwrite (derived from description)"},
			"content":     map[string]any{"type": "string", "description": "The new content to replace the existing memory with"},
			"description": map[string]any{"type": "string", "description": "Optional new description for the memory"},
		},
		"required": []string{"hash", "content"},
	},
}
