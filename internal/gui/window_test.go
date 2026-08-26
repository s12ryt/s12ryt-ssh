package gui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageProfilePreservesPathStyle(t *testing.T) {
	ui := NewWindow(nil)
	ui.storageName.SetText("r2")
	ui.storageEndpoint.SetText("https://account.r2.cloudflarestorage.com")
	ui.storageAccess.SetText("access")
	ui.storageSecret.SetText("secret")
	ui.storageBucket.SetText("data")
	ui.storagePathStyle = true

	profile, err := ui.storageProfile()
	if err != nil {
		t.Fatalf("storageProfile() error = %v", err)
	}
	if !profile.UsePathStyle {
		t.Fatal("storageProfile() lost the path-style setting")
	}
}

func TestToggleStoragePathStyleAffectsProfile(t *testing.T) {
	ui := NewWindow(nil)
	ui.storageName.SetText("r2")
	ui.storageEndpoint.SetText("https://account.r2.cloudflarestorage.com")
	ui.storageAccess.SetText("access")
	ui.storageSecret.SetText("secret")
	ui.storageBucket.SetText("data")

	ui.toggleStoragePathStyle()
	profile, err := ui.storageProfile()
	if err != nil {
		t.Fatalf("storageProfile() error = %v", err)
	}
	if !profile.UsePathStyle {
		t.Fatal("path-style toggle was not reflected in the storage profile")
	}
}

func TestCloseCancelsInteractiveTerminalContext(t *testing.T) {
	ui := NewWindow(nil)
	ctx, cancel := context.WithCancel(context.Background())
	ui.terminalCancel = cancel

	if err := ui.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Close() did not cancel the interactive terminal context")
	}
}

func TestWindowLanguagePreferenceDefaultsAndToggles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	ui := NewWindowWithPreferences(nil, path)
	if ui.language != "en" {
		t.Fatalf("default language = %q, want en", ui.language)
	}
	ui.toggleLanguage()
	if ui.language != "zh-TW" {
		t.Fatalf("toggled language = %q, want zh-TW", ui.language)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preferences: %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["password"]; ok {
		t.Fatal("preferences must not contain password")
	}
}

func TestLocalValidationMessagesAreStableTranslationSources(t *testing.T) {
	ui := NewWindow(nil)

	if _, err := ui.setupBootstrap(); err == nil || err.Error() != "S3 endpoint, bucket, access key, and secret key are required" {
		t.Fatalf("empty S3 bootstrap error = %v", err)
	}

	ui.setupBackend = "sql"
	ui.setupDBType.SetText("postgres")
	ui.setupDBHost.SetText("localhost")
	ui.setupDBUser.SetText("user")
	ui.setupDBPassword.SetText("password")
	ui.setupDBDatabase.SetText("app")
	if _, err := ui.setupBootstrap(); err == nil || err.Error() != "SQL type, host, port, user, password, and database are required" {
		t.Fatalf("incomplete SQL bootstrap error = %v", err)
	}

	ui.setupDBPort.SetText("not-a-port")
	if _, err := ui.setupBootstrap(); err == nil || err.Error() != "port must be between 1 and 65535" {
		t.Fatalf("invalid port error = %v", err)
	}

	if err := validateVaultCredentials("", ""); err == nil || err.Error() != "vault name and password are required" {
		t.Fatalf("empty vault credentials error = %v", err)
	}
	if err := validateLoginCredentials("", ""); err == nil || err.Error() != "vault name and password are required" {
		t.Fatalf("empty login credentials error = %v", err)
	}
	if err := validateRecoveryCredentials("", "", ""); err == nil || err.Error() != "recovery key, new vault name, and new vault password are required" {
		t.Fatalf("empty recovery credentials error = %v", err)
	}
}
