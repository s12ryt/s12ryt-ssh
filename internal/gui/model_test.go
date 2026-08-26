package gui

import (
	"context"
	"path/filepath"
	"testing"

	"s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/config"
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
