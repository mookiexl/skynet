package tools

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func init() {
	RegisterTool("memory_list", MemoryListDefinition, MemoryList)
}

//
// MemoryList implements the tool handler for the MemoryList tool.

// MemoryList returns all stored memory entries.
func MemoryList(args map[string]any) (any, error) {
	maxEntriesFloat, ok := ArgFloat(args, "max_entries")
	maxEntries := DefaultMaxEntries
	if ok {
		maxEntries = int(maxEntriesFloat)
	}
	if maxEntries > HardMaxEntries {
		maxEntries = HardMaxEntries
	}

	memories, err := ListMemoryFiles()
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Sort by modification time (most recent first)
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].ModTime.After(memories[j].ModTime)
	})

	var entriesStr strings.Builder
	count := 0
	for i, mem := range memories {
		if i >= maxEntries {
			break
		}
		count++
		if i > 0 {
			entriesStr.WriteString("\n")
		}
		fmt.Fprintf(&entriesStr, "%s, %s, %s, %d bytes, %s", mem.Hash, mem.Name, mem.Description, mem.Size, mem.ModTime.Format(time.RFC3339))
	}

	return map[string]any{
		"memories":      strings.TrimRight(entriesStr.String(), "\n"),
		"count":         count,
		"total_count":   len(memories),
		"limit_reached": len(memories) > maxEntries,
	}, nil
}

// MemoryListDefinition defines the tool schema.
var MemoryListDefinition = map[string]any{
	"description": "List all memories with their descriptions and hash IDs, ordered by most recently modified.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"max_entries": map[string]any{"type": "integer", "default": DefaultMaxEntries, "description": "Maximum number of memories to return"},
		},
		"required": []string{},
	},
}
