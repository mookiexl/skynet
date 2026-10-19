package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	typeFile      = "file"
	typeDirectory = "directory"
)

func init() {
	RegisterTool("list", ListDirectoryDefinition, ListDirectory)
}

func parseListArgs(args map[string]any) (string, bool, int, bool) {
	path, _ := ArgString(args, "path")
	if path == "" {
		path = "."
	}
	recursive, _ := ArgBool(args, "recursive")
	maxEntriesFloat, ok := ArgFloat(args, "max_entries")
	maxEntries := DefaultMaxEntries
	if ok {
		maxEntries = int(maxEntriesFloat)
	}
	if maxEntries > HardMaxEntries {
		maxEntries = HardMaxEntries
	}
	showHidden, _ := ArgBool(args, "show_hidden")
	return path, recursive, maxEntries, showHidden
}

func entryType(isDir bool) string {
	if isDir {
		return typeDirectory
	}
	return typeFile
}

func buildEntry(name, path, fullPath string, isDir bool, size int64) map[string]any {
	entry := map[string]any{
		"name":      name,
		"path":      path,
		"type":      entryType(isDir),
		"full_path": fullPath,
		"is_hidden": len(name) > 0 && name[0] == '.',
	}
	if !isDir {
		entry["size"] = size
	}
	return entry
}

func listRecursive(absPath string, showHidden bool, maxEntries int) ([]map[string]any, error) {
	entries := make([]map[string]any, 0)
	err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if !showHidden && len(name) > 0 && name[0] == '.' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(absPath, path)
		if relPath == "." {
			return nil
		}
		if len(entries) >= maxEntries {
			return fmt.Errorf("limit reached")
		}
		var size int64
		if !info.IsDir() {
			size = info.Size()
		}
		entries = append(entries, buildEntry(name, relPath, path, info.IsDir(), size))
		return nil
	})
	return entries, err
}

func listNonRecursive(absPath string, showHidden bool, maxEntries int) ([]map[string]any, error) {
	entries := make([]map[string]any, 0)
	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name := f.Name()
		if !showHidden && len(name) > 0 && name[0] == '.' {
			continue
		}
		if len(entries) >= maxEntries {
			break
		}
		var size int64
		if !f.IsDir() {
			info, _ := f.Info()
			size = info.Size()
		}
		entries = append(entries, buildEntry(name, name, filepath.Join(absPath, name), f.IsDir(), size))
	}
	return entries, nil
}

func buildListResult(absPath string, entries []map[string]any, recursive bool, maxEntries int) map[string]any {
	totalFiles, totalDirs := 0, 0
	var entriesStr strings.Builder

	for _, entry := range entries {
		if entry["type"] == typeFile {
			totalFiles++
		} else {
			totalDirs++
		}

		var name string
		if recursive {
			name, _ = entry["path"].(string)
		} else {
			name, _ = entry["name"].(string)
		}

		typeStr, _ := entry["type"].(string)
		line := fmt.Sprintf("%s, %s", name, typeStr)
		if typeStr == typeFile {
			if size, ok := entry["size"].(int64); ok {
				line += fmt.Sprintf(", %d bytes", size)
			}
		}
		entriesStr.WriteString(line + "\n")
	}

	result := map[string]any{
		"absolute_path":           absPath,
		"entries":                 strings.TrimRight(entriesStr.String(), "\n"),
		"total_files_found":       totalFiles,
		"total_directories_found": totalDirs,
		"limit_reached":           len(entries) >= maxEntries,
	}
	if len(entries) >= maxEntries {
		result["truncation_warning"] = fmt.Sprintf("Results were truncated to %d entries. There may be more files/directories not shown.", maxEntries)
	}
	return result
}

// ListDirectory lists files and directories.
func ListDirectory(args map[string]any) (any, error) {
	path, recursive, maxEntries, showHidden := parseListArgs(args)
	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	var entries []map[string]any
	if recursive {
		entries, err = listRecursive(absPath, showHidden, maxEntries)
	} else {
		entries, err = listNonRecursive(absPath, showHidden, maxEntries)
	}

	if err != nil && err.Error() != "limit reached" {
		return ErrorResult(err.Error()), nil
	}

	return buildListResult(absPath, entries, recursive, maxEntries), nil
}

// ListDirectoryDefinition defines the tool schema.
var ListDirectoryDefinition = map[string]any{
	"description": "List files and directories.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string", "default": ".", "description": "Directory path to list"},
			"recursive":   map[string]any{"type": "boolean", "default": false, "description": "Whether to list files recursively"},
			"max_entries": map[string]any{"type": "integer", "default": DefaultMaxEntries, "description": "Maximum number of entries to return"},
			"show_hidden": map[string]any{"type": "boolean", "default": false, "description": "Whether to include hidden files/folders"},
		},
		"required": []string{},
	},
}
