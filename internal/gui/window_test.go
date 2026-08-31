package gui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestWindowLoadsRememberedPasswordAndMarksAutomaticSignIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(remote.TokenPair{
			AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
			Account: remote.Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	service := remote.NewService(path, securestore.NewMemoryStore(), server.Client())
	if _, err := service.LoginWithOptions(context.Background(), server.URL, "alice", "remembered-password", true); err != nil {
		t.Fatal(err)
	}

	ui := NewWindowWithPreferences(service, "")
	if ui.remotePassword.Text() != "remembered-password" {
		t.Fatalf("remote password = %q", ui.remotePassword.Text())
	}
	if !ui.remoteRememberPassword.Value {
		t.Fatal("remembered password must enable the remember-password control")
	}
	if !ui.remoteAutoLoginPending {
		t.Fatal("remembered password must schedule automatic sign-in")
	}
}

func TestFailedAutomaticSignInClearsRememberedPasswordAndRequiresInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(remote.TokenPair{
			AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
			Account: remote.Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	service := remote.NewService(path, securestore.NewMemoryStore(), server.Client())
	if _, err := service.LoginWithOptions(context.Background(), server.URL, "alice", "remembered-password", true); err != nil {
		t.Fatal(err)
	}
	ui := NewWindowWithPreferences(service, "")
	ui.clearFailedRememberedPassword()

	if ui.remotePassword.Text() != "" || ui.remoteRememberPassword.Value || ui.remoteAutoLoginPending {
		t.Fatalf("automatic credential state = password %q, remember %v, pending %v", ui.remotePassword.Text(), ui.remoteRememberPassword.Value, ui.remoteAutoLoginPending)
	}
	if _, err := service.RememberedPassword(); !errors.Is(err, securestore.ErrNotFound) {
		t.Fatalf("RememberedPassword() error = %v", err)
	}
}

func TestAutomaticSignInFailureClearsSavedPasswordAfterAsyncError(t *testing.T) {
	var loginCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		loginCalls++
		if loginCalls > 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(remote.TokenPair{
			AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
			Account: remote.Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	service := remote.NewService(path, securestore.NewMemoryStore(), server.Client())
	if _, err := service.LoginWithOptions(context.Background(), server.URL, "alice", "remembered-password", true); err != nil {
		t.Fatal(err)
	}
	ui := NewWindowWithPreferences(service, "")
	if !ui.startRemoteAutoLogin() {
		t.Fatal("startRemoteAutoLogin should queue the saved sign-in")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("automatic sign-in did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()

	if ui.remotePassword.Text() != "" || ui.remoteRememberPassword.Value || ui.remoteAutoLoginPending {
		t.Fatalf("automatic failure state = password %q, remember %v, pending %v", ui.remotePassword.Text(), ui.remoteRememberPassword.Value, ui.remoteAutoLoginPending)
	}
	if _, err := service.RememberedPassword(); !errors.Is(err, securestore.ErrNotFound) {
		t.Fatalf("RememberedPassword() error = %v", err)
	}
}

func TestManualSignInFailureKeepsNewlyEnteredPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	service := remote.NewService(filepath.Join(t.TempDir(), "remote-preferences.json"), securestore.NewMemoryStore(), server.Client())
	ui := NewWindowWithPreferences(service, "")
	ui.remoteURL.SetText(server.URL)
	ui.remoteUsername.SetText("alice")
	ui.remotePassword.SetText("newly-entered-password")
	if !ui.tryRemoteSignIn() {
		t.Fatal("manual sign-in should queue the request")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("manual sign-in did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()
	if ui.remotePassword.Text() != "newly-entered-password" {
		t.Fatalf("manual failure cleared entered password: %q", ui.remotePassword.Text())
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

type appearanceRemoteSession struct {
	fakeRemoteSession
	preferences remote.SSHWorkspacePreferences
}

func (s *appearanceRemoteSession) SSHWorkspacePreferences(context.Context) (remote.SSHWorkspacePreferences, error) {
	return s.preferences, nil
}

func (s *appearanceRemoteSession) UpdateSSHWorkspacePreferences(_ context.Context, input remote.SSHWorkspacePreferencesInput) (remote.SSHWorkspacePreferences, error) {
	s.preferences.TerminalAppearance = input.TerminalAppearance
	s.preferences.Version++
	return s.preferences, nil
}

func TestRefreshTerminalAppearanceLoadsAccountDefaultsWithoutSecrets(t *testing.T) {
	ui := NewWindow(nil)
	session := &appearanceRemoteSession{preferences: remote.SSHWorkspacePreferences{
		TerminalAppearance: remote.SSHTerminalAppearance{
			Font: remote.SSHTerminalFontSystem, FontSize: 18,
			Foreground: "#f0f0f0", Background: "#080808",
		},
	}}
	ui.model.SetRemoteSession(session, true)
	if !ui.refreshTerminalAppearance() {
		t.Fatal("refreshTerminalAppearance must start an async request")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("terminal appearance refresh did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()
	if ui.terminalAppearance.Font != terminalFontSystem || ui.terminalAppearance.FontSize != 18 {
		t.Fatalf("terminal appearance = %+v", ui.terminalAppearance)
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

func TestQueueSSHTabApplyStopsAfterWindowClose(t *testing.T) {
	ui := NewWindow(nil)
	for range cap(ui.events) {
		ui.events <- asyncEvent{}
	}

	queued := make(chan struct{})
	go func() {
		ui.queueSSHTabApply(func() {})
		close(queued)
	}()

	select {
	case <-queued:
		t.Fatal("queue should wait while the event channel is full")
	case <-time.After(20 * time.Millisecond):
	}

	if err := ui.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-queued:
	case <-time.After(time.Second):
		t.Fatal("queued SSH work must stop waiting after window close")
	}

	cleaned := false
	ui.queueSSHTabApply(func() {}, func() { cleaned = true })
	if !cleaned {
		t.Fatal("discarded SSH work must release its captured resources")
	}
}

func TestWindowCloseCleansQueuedSSHTabResources(t *testing.T) {
	ui := NewWindow(nil)
	cleaned := false
	if !ui.queueSSHTabApply(func() {}, func() { cleaned = true }) {
		t.Fatal("SSH work should queue while the window is open")
	}
	if cleaned {
		t.Fatal("queued SSH resources must remain available until the event is consumed")
	}

	if err := ui.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !cleaned {
		t.Fatal("closing the window must release resources owned by unconsumed SSH events")
	}
}

func TestWindowCloseDoesNotWaitForPendingSSHTransport(t *testing.T) {
	ui := NewWindow(nil)
	transport := &testSSHTransport{closedSignal: make(chan struct{})}
	factoryStarted := make(chan struct{})
	releaseFactory := make(chan struct{})
	factoryReleased := false
	defer func() {
		if !factoryReleased {
			close(releaseFactory)
		}
	}()
	type acquireResult struct {
		lease *sshConnectionLease
		err   error
	}
	acquired := make(chan acquireResult, 1)
	go func() {
		lease, err := ui.sshPool.acquire(sshConnectionKey{HostID: "host-1", Version: 1}, func() (sshTransport, error) {
			close(factoryStarted)
			<-releaseFactory
			return transport, nil
		})
		acquired <- acquireResult{lease: lease, err: err}
	}()
	<-factoryStarted

	closed := make(chan error, 1)
	go func() { closed <- ui.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close() waited for an in-flight SSH transport factory")
	}

	close(releaseFactory)
	factoryReleased = true
	result := <-acquired
	if result.err == nil || result.lease != nil {
		t.Fatalf("acquisition after close = lease %v, error %v", result.lease, result.err)
	}
	select {
	case <-transport.closedSignal:
	case <-time.After(time.Second):
		t.Fatal("late transport was not closed")
	}
	if transport.closed != 1 {
		t.Fatalf("late transport close count = %d, want 1", transport.closed)
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
