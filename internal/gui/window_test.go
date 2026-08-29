package gui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/securestore"
)

func TestWindowLoadsOnlyNonSensitiveRemoteLoginPreferences(t *testing.T) {
	remotePath := filepath.Join(t.TempDir(), "remote-preferences.json")
	if err := remote.SavePreferences(remotePath, remote.Preferences{
		BaseURL: "https://auth.example.com", Username: "alice", DeviceID: "device-1",
	}); err != nil {
		t.Fatal(err)
	}
	remoteService := remote.NewService(remotePath, securestore.NewMemoryStore(), nil)
	ui := NewWindowWithPreferences(remoteService, "")
	if ui.remoteURL.Text() != "https://auth.example.com" || ui.remoteUsername.Text() != "alice" {
		t.Fatalf("remote login fields = url %q username %q", ui.remoteURL.Text(), ui.remoteUsername.Text())
	}
	if ui.remotePassword.Text() != "" {
		t.Fatal("remote password must never load from preferences")
	}
}

func TestRemoteWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	ui := NewWindow(nil)
	ui.language = "zh-TW"
	sources := []string{
		"Sign in with authentication service",
		"Use a complete HTTP or HTTPS URL. The password is never saved.",
		"Authentication service URL",
		"Account",
		"Sign in remotely",
		"Restore saved session",
		"Remote workspace",
		"Remote sign-in URL, account, and password are required",
		"Signing in to authentication service...",
		"Restoring remote session...",
		"Remote authentication service is unavailable",
		"SSH access is not enabled for this account.",
	}
	for _, source := range sources {
		if got := ui.text(source); got == source {
			t.Fatalf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestRemoteCredentialValidationUsesStableTranslationSource(t *testing.T) {
	if err := validateRemoteCredentials("", "alice", "password"); err == nil || err.Error() != "Remote sign-in URL, account, and password are required" {
		t.Fatalf("empty URL error = %v", err)
	}
	if err := validateRemoteCredentials("https://auth.example.com", "", "password"); err == nil || err.Error() != "Remote sign-in URL, account, and password are required" {
		t.Fatalf("empty account error = %v", err)
	}
	if err := validateRemoteCredentials("https://auth.example.com", "alice", ""); err == nil || err.Error() != "Remote sign-in URL, account, and password are required" {
		t.Fatalf("empty password error = %v", err)
	}
}

func TestActivateRemoteSessionRefreshesSSHHostsWhenEnabled(t *testing.T) {
	ui := NewWindow(nil)
	session := &fakeRemoteSession{}
	ui.activateRemoteSession(session, true)
	if !ui.busy {
		t.Fatal("activateRemoteSession must refresh SSH hosts when enabled")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("SSH hosts refresh did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()
	if len(ui.sshHosts) != 1 || ui.sshHosts[0].ID != "host-1" {
		t.Fatalf("ssh hosts after refresh = %+v", ui.sshHosts)
	}
	if ui.busy {
		t.Fatal("pump must clear the busy flag")
	}

	ui.activateRemoteSession(session, false)
	if ui.busy {
		t.Fatal("activateRemoteSession must not refresh SSH hosts when disabled")
	}
	select {
	case <-ui.events:
		t.Fatal("disabled session must not queue another refresh")
	default:
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
