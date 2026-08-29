package gui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestWindowFiltersAssignedResourcesByRemoteTabAndPermission(t *testing.T) {
	ui := NewWindow(nil)
	ui.remoteResources = []remote.Resource{
		{ID: "s3", Name: "Storage", Kind: "s3", Enabled: true, Operations: []remote.Operation{remote.OperationS3Read}},
		{ID: "mysql", Name: "MySQL", Kind: "mysql", Enabled: true, Operations: []remote.Operation{remote.OperationSQLTables, remote.OperationSQLQuery}},
		{ID: "postgres", Name: "Postgres", Kind: "postgres", Enabled: false, Operations: []remote.Operation{remote.OperationSQLExec}},
	}
	storage := ui.remoteResourceIndices(TabStorage)
	if len(storage) != 1 || ui.remoteResources[storage[0]].ID != "s3" {
		t.Fatalf("storage indices = %v", storage)
	}
	database := ui.remoteResourceIndices(TabDatabase)
	if len(database) != 1 || ui.remoteResources[database[0]].ID != "mysql" {
		t.Fatalf("database indices = %v", database)
	}
	ui.remoteIndex = database[0]
	if !ui.remoteAllows(remote.OperationSQLQuery) || ui.remoteAllows(remote.OperationSQLExec) {
		t.Fatalf("remote permissions = %+v", ui.remoteResources[database[0]].Operations)
	}
}

func TestFormatRemoteRowsUsesServerColumnOrder(t *testing.T) {
	result := remote.SQLQueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]any{{float64(1), "alice"}, {float64(2), "bob"}},
	}
	got := formatRemoteRows(result)
	if !strings.Contains(got, "id=1 | name=alice") || !strings.Contains(got, "id=2 | name=bob") {
		t.Fatalf("formatted rows = %q", got)
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
		"Assigned connections",
		"No assigned connections.",
		"Remote sign-in URL, account, and password are required",
		"Signing in to authentication service...",
		"Restoring remote session...",
		"Permission not granted for this operation",
		"No connection selected",
		"Remote authentication service is unavailable",
		"Loading assigned connections...",
		"Listing remote objects...",
		"Assigned S3 / R2",
		"Assigned SQL database",
		"Uploaded ",
		"Remote objects and operation output",
		"Remote database output",
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

func TestSelectedRemoteResourceRequiresEnabledGrant(t *testing.T) {
	ui := NewWindow(nil)
	ui.model.RemoteSession = &fakeRemoteSession{}
	ui.remoteResources = []remote.Resource{
		{ID: "s3", Name: "Storage", Kind: "s3", Enabled: true, Operations: []remote.Operation{remote.OperationS3Read}},
	}
	ui.remoteIndex = 0
	if _, resource, err := ui.selectedRemoteResource(remote.OperationS3Read); err != nil || resource.ID != "s3" {
		t.Fatalf("selectedRemoteResource(read) = resource %+v, error %v", resource, err)
	}
	if _, _, err := ui.selectedRemoteResource(remote.OperationS3Delete); err == nil || err.Error() != "Permission not granted for this operation" {
		t.Fatalf("selectedRemoteResource(delete) error = %v", err)
	}
	ui.remoteResources[0].Enabled = false
	if _, _, err := ui.selectedRemoteResource(remote.OperationS3Read); err == nil || err.Error() != "Permission not granted for this operation" {
		t.Fatalf("disabled resource error = %v", err)
	}
}

func TestActivateRemoteSessionSelectsFirstUsableStorageResource(t *testing.T) {
	ui := NewWindow(nil)
	session := &fakeRemoteSession{}
	ui.activateRemoteSession(session, []remote.Resource{
		{ID: "disabled", Name: "Disabled", Kind: "s3", Enabled: false, Operations: []remote.Operation{remote.OperationS3Read}},
		{ID: "database", Name: "Database", Kind: "mysql", Enabled: true, Operations: []remote.Operation{remote.OperationSQLQuery}},
		{ID: "storage", Name: "Storage", Kind: "s3", Enabled: true, Operations: []remote.Operation{remote.OperationS3Read}},
	}, false)
	if ui.model.Screen != ScreenRemoteWorkspace || ui.model.Tab != TabStorage {
		t.Fatalf("remote state = screen %v tab %v", ui.model.Screen, ui.model.Tab)
	}
	if ui.remoteIndex != 2 {
		t.Fatalf("selected resource index = %d, want 2", ui.remoteIndex)
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
