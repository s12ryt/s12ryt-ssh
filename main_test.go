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

	root := applicationConfigRoot()
	wantRoot := filepath.Join(configDir, "s12ryt-ssh")
	if root != wantRoot {
		t.Fatalf("config root = %q, want %q", root, wantRoot)
	}
	if got := applicationSecurestoreDir(); got != filepath.Join(wantRoot, "securestore") {
		t.Fatalf("securestore path = %q, want under %q", got, wantRoot)
	}
}

func TestApplicationPreferencesUseSeparateNonSensitiveFile(t *testing.T) {
	want := filepath.Join(applicationConfigRoot(), "preferences.json")
	if got := applicationPreferencesPath(); got != want {
		t.Fatalf("preferences path = %q, want %q", got, want)
	}
}

func TestApplicationRemotePreferencesUseSeparateNonSensitiveFile(t *testing.T) {
	root := applicationConfigRoot()
	want := filepath.Join(root, "remote-preferences.json")
	if got := applicationRemotePreferencesPath(); got != want {
		t.Fatalf("remote preferences path = %q, want %q", got, want)
	}
	if got := applicationRemotePreferencesPath(); got == applicationPreferencesPath() {
		t.Fatalf("remote preferences path %q must be separate from language preferences", got)
	}
}
