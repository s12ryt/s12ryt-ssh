package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/securestore"
	"s12ryt-ssh/internal/storage"
	"s12ryt-ssh/internal/vault"
)

func TestServiceRegisterLoginAndUpdate(t *testing.T) {
	ctx := context.Background()
	remote := vault.NewObjectBackend(storage.NewMemoryStorage())
	secrets := securestore.NewMemoryStore()
	metadataPath := filepathForTest(t)
	service := NewService(metadataPath, secrets, func(context.Context, Bootstrap) (BackendHandle, error) {
		return BackendHandle{Backend: remote, Probe: func(context.Context) error { return nil }}, nil
	})
	bootstrap := Bootstrap{
		Backend: "s3",
		S3:      config.S3Profile{Name: "vault", Endpoint: "https://r2.example", AccessKey: "access", SecretKey: "bootstrap-secret", Bucket: "vault"},
	}
	profiles := &config.Store{SSH: []config.SSHProfile{{Name: "server", Host: "host", Password: "profile-secret"}}}

	registration, err := service.Register(ctx, bootstrap, "alice", "password", profiles)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if registration.ID == "" || registration.RecoveryKey == "" {
		t.Fatalf("invalid registration: %+v", registration)
	}
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metadata), "bootstrap-secret") || strings.Contains(string(metadata), "profile-secret") {
		t.Fatalf("metadata contains secret: %s", metadata)
	}

	session, err := service.Login(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	got, err := session.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, profiles) {
		t.Fatalf("profiles mismatch: got %+v want %+v", got, profiles)
	}
	updated := &config.Store{DB: []config.DBProfile{{Name: "db", Type: "postgres", Host: "db", Port: 5432}}}
	if err := session.SaveProfiles(ctx, updated); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := service.Login(ctx, "alice", "password")
	if err != nil {
		t.Fatalf("Login after update: %v", err)
	}
	defer second.Close()
	got, err = second.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("updated profiles mismatch: got %+v want %+v", got, updated)
	}
	if _, err := service.Login(ctx, "alice", "wrong"); err == nil {
		t.Fatal("wrong password should fail")
	}
}

func TestServiceConfigurationStatusAndMetadata(t *testing.T) {
	ctx := context.Background()
	metadataPath := filepathForTest(t)
	remote := vault.NewObjectBackend(storage.NewMemoryStorage())
	service := NewService(metadataPath, securestore.NewMemoryStore(), func(context.Context, Bootstrap) (BackendHandle, error) {
		return BackendHandle{Backend: remote}, nil
	})
	if service.Configured() {
		t.Fatal("missing metadata must report unconfigured")
	}
	if _, err := service.Metadata(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Metadata on missing file: %v", err)
	}
	bootstrap := Bootstrap{Backend: "s3", S3: config.S3Profile{
		Endpoint: "https://r2.example", AccessKey: "access", SecretKey: "bootstrap-secret", Bucket: "vault",
	}}
	registration, err := service.Register(ctx, bootstrap, "alice", "password", &config.Store{})
	if err != nil {
		t.Fatal(err)
	}
	if !service.Configured() {
		t.Fatal("registered service must report configured")
	}
	metadata, err := service.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.VaultID != registration.ID || metadata.Name != "alice" || metadata.Backend != "s3" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
}

func TestServiceRecoverRotatesLogin(t *testing.T) {
	ctx := context.Background()
	remote := vault.NewObjectBackend(storage.NewMemoryStorage())
	service := NewService(filepathForTest(t), securestore.NewMemoryStore(), func(context.Context, Bootstrap) (BackendHandle, error) {
		return BackendHandle{Backend: remote}, nil
	})
	bootstrap := Bootstrap{Backend: "s3", S3: config.S3Profile{
		Endpoint: "https://r2.example", AccessKey: "access", SecretKey: "bootstrap-secret", Bucket: "vault",
	}}
	registration, err := service.Register(ctx, bootstrap, "alice", "password", &config.Store{})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := service.Recover(ctx, registration.RecoveryKey, "bob", "new-password")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if rotated.ID != registration.ID || rotated.Name != "bob" || rotated.RecoveryKey == registration.RecoveryKey {
		t.Fatalf("unexpected rotated registration: %+v", rotated)
	}
	if _, err := service.Login(ctx, "alice", "password"); err == nil {
		t.Fatal("old credentials should fail after recovery")
	}
	session, err := service.Login(ctx, "bob", "new-password")
	if err != nil {
		t.Fatalf("new credentials should work: %v", err)
	}
	defer session.Close()
}

func filepathForTest(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "metadata.json")
}
