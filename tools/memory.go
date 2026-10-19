package tools

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryHash calculates a hash from a memory description.
// Returns the first MemoryHashLen hex characters of the MD5 hash.
func MemoryHash(description string) string {
	hash := md5.Sum([]byte(description))
	return fmt.Sprintf("%x", hash)[:MemoryHashLen]
}

// CreateSlug creates a URL-safe slug from a description string.
// Truncates to SlugMaxLength characters and replaces non-alphanumeric characters with underscores.
func CreateSlug(description string) string {
	if len(description) > SlugMaxLength {
		description = description[:SlugMaxLength]
	}
	description = strings.ToLower(description)
	var result strings.Builder
	for _, r := range description {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}

// MemoryFileInfo represents a memory file's metadata and content
type MemoryFileInfo struct {
	Name        string // Display name (underscores replaced with spaces)
	FileName    string // Original filename including .txt extension
	Description string // First line of the file
	Hash        string // Hash of the description
	Size        int    // File size in bytes
	ModTime     time.Time
	FilePath    string // Path to the file
	Content     string // Full file content
}

// ListMemoryFiles scans the .memory directory and returns info about all memory files.
// Returns an empty slice (not an error) if the directory doesn't exist.
// Memories are intentionally NOT cached — the directory is re-scanned from disk on every call (live reload).
func ListMemoryFiles() ([]MemoryFileInfo, error) {
	memoryDir := ".memory"

	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		return []MemoryFileInfo{}, nil
	}

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil, fmt.Errorf("error reading memory directory: %w", err)
	}

	var memories []MemoryFileInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(memoryDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)
		lines := strings.SplitN(contentStr, "\n", 2)

		description := ""
		if len(lines) > 0 {
			description = lines[0]
		}

		// Get display name: remove .txt, replace underscores with spaces
		baseName := strings.TrimSuffix(entry.Name(), ".txt")
		displayName := strings.ReplaceAll(baseName, "_", " ")

		memories = append(memories, MemoryFileInfo{
			Name:        displayName,
			FileName:    entry.Name(),
			Description: description,
			Hash:        MemoryHash(description),
			Size:        len(contentStr),
			ModTime:     info.ModTime(),
			FilePath:    filePath,
			Content:     contentStr,
		})
	}

	return memories, nil
}

// FindMemoryByHash finds a memory file by its hash ID.
// Returns the MemoryFileInfo and nil error if found, or a "not found" error if not.
func FindMemoryByHash(hash string) (*MemoryFileInfo, error) {
	memories, err := ListMemoryFiles()
	if err != nil {
		return nil, err
	}

	for _, mem := range memories {
		if mem.Hash == hash {
			return &mem, nil
		}
	}

	return nil, fmt.Errorf("no memory found with hash '%s'", hash)
}
