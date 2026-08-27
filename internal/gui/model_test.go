package gui

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/securestore"
	"s12ryt-ssh/internal/storage"
	"s12ryt-ssh/internal/vault"
)

func TestNewModelChoosesSetupOrLogin(t *testing.T) {
	missing := app.NewService(filepath.Join(t.TempDir(), "metadata.json"), securestore.NewMemoryStore(), nil)
	if got := NewModel(missing).Screen; got != ScreenSetup {
		t.Fatalf("missing vault screen: got %v", got)
	}

	service, _ := registeredService(t)
	model := NewModel(service)
	if model.Screen != ScreenLogin {
		t.Fatalf("configured vault screen: got %v", model.Screen)
	}
	if model.AccountName != "alice" {
		t.Fatalf("account name: %q", model.AccountName)
	}
}

type fakeRemoteSession struct {
	logoutCount int
}

func (s *fakeRemoteSession) Account() remote.Account {
	return remote.Account{ID: "account-1", Username: "remote-alice"}
}

func (s *fakeRemoteSession) Resources(context.Context) ([]remote.Resource, error) {
	return nil, nil
}

func (s *fakeRemoteSession) ListObjects(context.Context, string, string) ([]remote.S3Object, error) {
	return nil, nil
}

func (s *fakeRemoteSession) UploadObject(context.Context, string, string, io.ReadSeeker, int64) (remote.UploadResult, error) {
	return remote.UploadResult{}, nil
}

func (s *fakeRemoteSession) DownloadObject(context.Context, string, string) (remote.Download, error) {
	return remote.Download{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (s *fakeRemoteSession) DeleteObject(context.Context, string, string) error {
	return nil
}

func (s *fakeRemoteSession) Tables(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *fakeRemoteSession) Query(context.Context, string, string, []any) (remote.SQLQueryResult, error) {
	return remote.SQLQueryResult{}, nil
}

func (s *fakeRemoteSession) Exec(context.Context, string, string, []any) (remote.SQLExecResult, error) {
	return remote.SQLExecResult{}, nil
}

func (s *fakeRemoteSession) Logout(context.Context) error {
	s.logoutCount++
	return nil
}

func TestModelRemoteWorkspaceExcludesSSHAndReturnsToOriginalScreen(t *testing.T) {
	missing := app.NewService(filepath.Join(t.TempDir(), "metadata.json"), securestore.NewMemoryStore(), nil)
	model := NewModelWithRemote(missing, nil)
	model.BeginRemoteLogin()
	if model.Screen != ScreenRemoteLogin {
		t.Fatalf("remote login screen = %v", model.Screen)
	}

	session := &fakeRemoteSession{}
	model.SetRemoteSession(session)
	if model.Screen != ScreenRemoteWorkspace || model.Tab != TabStorage || model.RemoteAccountName != "remote-alice" {
		t.Fatalf("remote workspace state = %+v", model)
	}
	model.SelectTab(TabSSH)
	if model.Tab != TabStorage {
		t.Fatalf("remote workspace accepted SSH tab: %v", model.Tab)
	}
	model.SelectTab(TabDatabase)
	if model.Tab != TabDatabase {
		t.Fatalf("remote database tab = %v", model.Tab)
	}
	if err := model.LogoutRemote(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.logoutCount != 1 || model.Screen != ScreenSetup || model.RemoteSession != nil {
		t.Fatalf("remote logout state = %+v, logout count = %d", model, session.logoutCount)
	}
}

func TestModelCancelsRemoteLoginBackToConfiguredLocalLogin(t *testing.T) {
	service, _ := registeredService(t)
	model := NewModelWithRemote(service, nil)
	model.BeginRemoteLogin()
	model.CancelRemoteLogin()
	if model.Screen != ScreenLogin || model.AccountName != "alice" {
		t.Fatalf("cancel remote login state = %+v", model)
	}
}

func TestModelTransitionsThroughRecoveryAndWorkspace(t *testing.T) {
	service, _ := registeredService(t)
	model := NewModel(service)
	registration := vault.Registration{ID: "vault-id", Name: "alice", RecoveryKey: "recovery-key"}
	model.SetRegistration(registration)
	if model.Screen != ScreenRecovery || model.RecoveryKey != registration.RecoveryKey {
		t.Fatalf("recovery state: %+v", model)
	}
	model.ContinueFromRecovery()
	if model.Screen != ScreenLogin || model.AccountName != registration.Name {
		t.Fatalf("login state: %+v", model)
	}
	model.BeginRecovery()
	if model.Screen != ScreenRecovery || model.RecoveryKey != "" {
		t.Fatalf("recovery form state: %+v", model)
	}
	model.ContinueFromRecovery()

	session, err := service.Login(context.Background(), "alice", "password")
	if err != nil {
		t.Fatal(err)
	}
	model.SetSession(session)
	if model.Screen != ScreenWorkspace || model.Tab != TabSSH {
		t.Fatalf("workspace state: %+v", model)
	}
	model.SelectTab(TabStorage)
	if model.Tab != TabStorage {
		t.Fatalf("selected tab: %v", model.Tab)
	}
	if err := model.Logout(); err != nil {
		t.Fatal(err)
	}
	if model.Screen != ScreenLogin || model.Session != nil {
		t.Fatalf("logout state: %+v", model)
	}
}

func registeredService(t *testing.T) (*app.Service, vault.Registration) {
	t.Helper()
	remote := vault.NewObjectBackend(storage.NewMemoryStorage())
	service := app.NewService(filepath.Join(t.TempDir(), "metadata.json"), securestore.NewMemoryStore(), func(context.Context, app.Bootstrap) (app.BackendHandle, error) {
		return app.BackendHandle{Backend: remote}, nil
	})
	bootstrap := app.Bootstrap{Backend: "s3", S3: config.S3Profile{
		Endpoint: "https://r2.example", AccessKey: "access", SecretKey: "bootstrap-secret", Bucket: "vault",
	}}
	registration, err := service.Register(context.Background(), bootstrap, "alice", "password", &config.Store{})
	if err != nil {
		t.Fatal(err)
	}
	return service, registration
}
