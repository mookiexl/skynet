package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AtomicWriteFile writes data to a file atomically using a temp file + rename.
func AtomicWriteFile(path string, data []byte) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, FileModeRegularFile); err != nil {
		return fmt.Errorf("error writing temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("error renaming file: %w", err)
	}
	return nil
}

// writeFile creates parent directories and atomically writes data to the path.
func writeFile(path string, data []byte) error {
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, FileModeDirectory); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := AtomicWriteFile(path, data); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}

// ReadTransformWrite reads a file, applies a transformation, and writes it back atomically.
// Returns an error if reading, transforming, or writing fails.
func ReadTransformWrite(path string, transform func(string) (string, error)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	newContent, err := transform(string(data))
	if err != nil {
		return err
	}
	if err := AtomicWriteFile(path, []byte(newContent)); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}

// FormatFileSize returns a human-readable file size string.
func FormatFileSize(size int) string {
	sizeStr := fmt.Sprintf("%d KB", size/KBConversion)
	if size >= MBConversion {
		sizeStr = fmt.Sprintf("%.1f MB", float64(size)/float64(MBConversion))
	}
	return sizeStr
}

// resolveSymlinks resolves the longest existing prefix of path via
// EvalSymlinks and re-appends the (possibly nonexistent) trailing
// components. This lets callers check containment for paths that do
// not yet exist, which filepath.EvalSymlinks rejects on its own.
func resolveSymlinks(path string) (string, error) {
	p := path
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return resolved + strings.TrimPrefix(path, p), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", err
		}
		p = parent
	}
}

// ValidatePath validates a path and returns the absolute path.// ValidatePath validates a path and returns the absolute path.
// Returns an error if the path is invalid or escapes the working directory.
func ValidatePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to determine working directory: %w", err)
	}

	// The path must be on the same volume as the working directory.
	// (On Unix both VolumeName values are ""; on Windows this rejects
	// cross-drive paths such as C:\ vs D:\.)
	if filepath.VolumeName(absPath) != filepath.VolumeName(baseDir) {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}

	// Containment check via filepath.Rel: works uniformly whether baseDir
	// is a drive root (e.g. "C:\"), the filesystem root ("/"), or any
	// ordinary directory, with no trailing-separator special cases.
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}

	// Also reject paths that escape the working directory via symlinks:
	// resolve both the path and the working directory, then re-run the
	// containment check on the resolved paths. (For the target path, only
	// the existing prefix can be resolved, since the file may not exist yet.)
	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		resolvedBase = baseDir
	}
	resolvedPath, err := resolveSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}
	rel, err = filepath.Rel(resolvedBase, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("access denied: path escapes allowed directory")
	}

	return absPath, nil
}
