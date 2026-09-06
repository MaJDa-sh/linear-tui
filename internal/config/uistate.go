package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// UIState holds transient UI preferences that should persist across restarts.
type UIState struct {
	CollapsedStages map[string]bool `json:"collapsed_stages"`
}

// UIStateFilePath returns the default UI state file path.
func UIStateFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".linear-tui", "ui_state.json"), nil
}

// LoadUIState loads UI state from disk. Returns an empty state if the file does not exist.
func LoadUIState(path string) (UIState, error) {
	state := UIState{CollapsedStages: make(map[string]bool)}
	if path == "" {
		return state, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, fmt.Errorf("read ui state file: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parse ui state file: %w", err)
	}

	if state.CollapsedStages == nil {
		state.CollapsedStages = make(map[string]bool)
	}

	return state, nil
}

// SaveUIState writes UI state to disk, creating directories as needed.
func SaveUIState(path string, state UIState) error {
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create ui state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ui state: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write ui state file: %w", err)
	}

	return nil
}
