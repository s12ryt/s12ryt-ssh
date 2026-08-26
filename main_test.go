package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationPathsUseUserConfigDirectory(t *testing.T) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() error = %v", err)
	}

	metadataPath, securestoreDir := applicationPaths()
	wantRoot := filepath.Join(configDir, "s12ryt-ssh")
	if metadataPath != filepath.Join(wantRoot, "metadata.json") {
		t.Fatalf("metadata path = %q, want under %q", metadataPath, wantRoot)
	}
	if securestoreDir != filepath.Join(wantRoot, "securestore") {
		t.Fatalf("securestore path = %q, want under %q", securestoreDir, wantRoot)
	}
}

func TestApplicationPreferencesUseSeparateNonSensitiveFile(t *testing.T) {
	metadataPath, _ := applicationPaths()
	if got, want := applicationPreferencesPath(), filepath.Join(filepath.Dir(metadataPath), "preferences.json"); got != want {
		t.Fatalf("preferences path = %q, want %q", got, want)
	}
}
