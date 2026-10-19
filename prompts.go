package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Prompt represents a loaded prompt with its name and content
type Prompt struct {
	Name        string
	Description string
	Content     string
}

// findPromptsDir tries to locate the prompts directory
func findPromptsDir() string {
	possiblePaths := []string{
		".prompts", // Relative to current directory
		filepath.Join(filepath.Dir(os.Args[0]), ".prompts"), // Relative to binary
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// ScanPrompts dynamically scans the prompts directory and returns all prompts
// Prompts are intentionally NOT cached — they are re-scanned from disk on every request (live reload).
func ScanPrompts() []Prompt {
	promptsDir := findPromptsDir()
	if promptsDir == "" {
		fmt.Println("Warning: prompts directory not found")
		return []Prompt{}
	}

	// Read all files in the prompts directory
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		fmt.Printf("Warning: could not read prompts directory: %v\n", err)
		return []Prompt{}
	}

	var prompts []Prompt

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .txt and .md files
		fileName := entry.Name()
		if !strings.HasSuffix(fileName, ".txt") && !strings.HasSuffix(fileName, ".md") {
			continue
		}

		// Get the prompt name (filename without extension)
		promptName := strings.TrimSuffix(fileName, ".txt")
		promptName = strings.TrimSuffix(promptName, ".md")

		// Read the file content
		filePath := filepath.Join(promptsDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Warning: could not read prompt file %s: %v\n", filePath, err)
			continue
		}

		prompts = append(prompts, Prompt{
			Name:        promptName,
			Description: "Prompt: " + promptName,
			Content:     string(content),
		})
	}

	return prompts
}

// GetPrompt retrieves a single prompt by name (dynamic load)
func GetPrompt(name string) (*Prompt, error) {
	prompts := ScanPrompts()

	for _, prompt := range prompts {
		if prompt.Name == name {
			return &prompt, nil
		}
	}

	return nil, fmt.Errorf("prompt %s not found", name)
}
