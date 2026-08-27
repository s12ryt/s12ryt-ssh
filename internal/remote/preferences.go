package remote

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const preferencesVersion = 1

// Preferences contains only non-sensitive remote login fields.
type Preferences struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
}

type preferencesFile struct {
	Version int `json:"version"`
	Preferences
}

// LoadPreferences loads a valid versioned preference file or returns empty preferences.
func LoadPreferences(path string) (Preferences, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, err
	}
	var stored preferencesFile
	if err := json.Unmarshal(data, &stored); err != nil || stored.Version != preferencesVersion {
		return Preferences{}, nil
	}
	client, err := NewClient(stored.BaseURL, nil)
	if err != nil || strings.TrimSpace(stored.Username) == "" || strings.TrimSpace(stored.DeviceID) == "" {
		return Preferences{}, nil
	}
	return Preferences{
		BaseURL:  client.BaseURL(),
		Username: strings.TrimSpace(stored.Username),
		DeviceID: strings.TrimSpace(stored.DeviceID),
	}, nil
}

// SavePreferences atomically stores only the remote URL, username, and device ID.
func SavePreferences(path string, preferences Preferences) error {
	client, err := NewClient(preferences.BaseURL, nil)
	if err != nil {
		return err
	}
	preferences.BaseURL = client.BaseURL()
	preferences.Username = strings.TrimSpace(preferences.Username)
	preferences.DeviceID = strings.TrimSpace(preferences.DeviceID)
	if preferences.Username == "" || preferences.DeviceID == "" {
		return errors.New("remote: username and device ID are required")
	}
	data, err := json.MarshalIndent(preferencesFile{
		Version:     preferencesVersion,
		Preferences: preferences,
	}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
