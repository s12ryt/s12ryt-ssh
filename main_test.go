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

func TestApplicationRemotePreferencesUseSeparateNonSensitiveFile(t *testing.T) {
	metadataPath, _ := applicationPaths()
	want := filepath.Join(filepath.Dir(metadataPath), "remote-preferences.json")
	if got := applicationRemotePreferencesPath(); got != want {
		t.Fatalf("remote preferences path = %q, want %q", got, want)
	}
	if got := applicationRemotePreferencesPath(); got == applicationPreferencesPath() || got == metadataPath {
		t.Fatalf("remote preferences path %q must be separate from language preferences and vault metadata", got)
	}
}
