package fizzy

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings provides local key-value storage for STM app state.
// Stored as a JSON file at ~/.local/share/stm/settings.json.
type Settings struct {
	path   string
	values map[string]string
}

// NewSettings loads or creates settings from the standard data directory.
func NewSettings() (*Settings, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}

	appDir := filepath.Join(dataDir, "stm")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(appDir, "settings.json")
	values := make(map[string]string)

	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &values)
	}

	return &Settings{path: path, values: values}, nil
}

// Get retrieves a setting value by key. Returns empty string if not found.
func (s *Settings) Get(key string) string {
	return s.values[key]
}

// Set stores a setting value.
func (s *Settings) Set(key, value string) error {
	s.values[key] = value
	data, err := json.MarshalIndent(s.values, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
