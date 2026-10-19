package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"skynet/tools"
)

const settingsFile = ".skynet.yml"

// DefaultSubagentLLMAPI is the default subagent LLM API endpoint.
const DefaultSubagentLLMAPI = "http://localhost:8080/v1"

// Settings represents the server configuration
type Settings struct {
	EnabledTools         map[string]bool `yaml:"enabled_tools"`
	SubagentEnabledTools map[string]bool `yaml:"subagent_enabled_tools"`
	Port                 int             `yaml:"port"`
	RemoteAccess         bool            `yaml:"remote_access"`
	SubagentLLMAPI       string          `yaml:"subagent_llm_api"`
}

// DefaultSettings returns a Settings struct with default values
func DefaultSettings() Settings {
	return Settings{
		EnabledTools:         make(map[string]bool),
		SubagentEnabledTools: make(map[string]bool),
		Port:                 tools.DefaultServerPort,
		RemoteAccess:         false,
		SubagentLLMAPI:       DefaultSubagentLLMAPI,
	}
}

// LoadSettings loads settings from the .skynet.yml file
// Returns default settings if file doesn't exist or is invalid
// The second return value indicates whether settings were successfully loaded from disk
func LoadSettings() (Settings, bool) {
	settings := DefaultSettings()

	wd, err := os.Getwd()
	if err != nil {
		return settings, false
	}

	filePath := filepath.Join(wd, settingsFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: Could not read settings file: %v\n", err)
		}
		return settings, false
	}

	err = yaml.Unmarshal(data, &settings)
	if err != nil {
		fmt.Printf("Warning: Could not parse settings file: %v\n", err)
		return settings, false
	}

	return settings, true
}

// SaveSettings saves settings to the .skynet.yml file
func SaveSettings(settings Settings) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get working directory: %w", err)
	}

	filePath := filepath.Join(wd, settingsFile)

	data, err := yaml.Marshal(&settings)
	if err != nil {
		return fmt.Errorf("could not marshal settings: %w", err)
	}

	err = os.WriteFile(filePath, data, tools.FileModeRegularFile)
	if err != nil {
		return fmt.Errorf("could not write settings file: %w", err)
	}

	return nil
}
