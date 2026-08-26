package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"s12ryt-ssh/internal/config"
)

func testStore() *config.Store {
	return &config.Store{
		SSH: []config.SSHProfile{{Name: "web", Host: "10.0.0.5", Port: 22, User: "root", Password: "ssh-secret"}},
		S3:  []config.S3Profile{{Name: "r2", Endpoint: "https://r2.example", AccessKey: "access", SecretKey: "storage-secret", Bucket: "data"}},
		DB:  []config.DBProfile{{Name: "db", Type: "postgres", Host: "db.example", Port: 5432, User: "admin", Password: "db-secret", Database: "app"}},
	}
}

func TestCreateAndDecryptRoundTrip(t *testing.T) {
	want := testStore()
	registration, ciphertext, err := Create("alice", "correct horse battery staple", want)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if registration.ID == "" || registration.RecoveryKey == "" {
		t.Fatal("Create should generate an ID and recovery key")
	}
	if strings.Contains(string(ciphertext), "ssh-secret") || strings.Contains(string(ciphertext), "storage-secret") || strings.Contains(string(ciphertext), "db-secret") {
		t.Fatal("ciphertext contains a profile secret")
	}

	got, err := Decrypt(ciphertext, "alice", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestDecryptRejectsWrongCredentials(t *testing.T) {
	registration, ciphertext, err := Create("alice", "secret", testStore())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		username string
		password string
	}{
		{name: "wrong password", username: registration.Name, password: "wrong"},
		{name: "wrong name", username: "bob", password: "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decrypt(ciphertext, tc.username, tc.password); err == nil {
				t.Fatal("expected credential error")
			}
		})
	}
}

func TestRecoverRewrapsVaultWithNewCredentials(t *testing.T) {
	want := testStore()
	registration, ciphertext, err := Create("alice", "old-password", want)
	if err != nil {
		t.Fatal(err)
	}

	rotatedRegistration, rotated, err := Recover(ciphertext, registration.RecoveryKey, "bob", "new-password")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if rotatedRegistration.ID != registration.ID || rotatedRegistration.Name != "bob" || rotatedRegistration.RecoveryKey == registration.RecoveryKey || rotatedRegistration.RecoveryKey == "" {
		t.Fatalf("Recover should preserve ID and issue a fresh recovery key: %+v", rotatedRegistration)
	}
	if bytes.Equal(rotated, ciphertext) {
		t.Fatal("recovered vault should have fresh wrapping metadata")
	}
	if _, err := Decrypt(rotated, "alice", "old-password"); err == nil {
		t.Fatal("old credentials should no longer decrypt recovered vault")
	}
	if _, _, err := Recover(rotated, registration.RecoveryKey, "carol", "another-password"); err == nil {
		t.Fatal("old recovery key should not open the replaced vault")
	}
	got, err := Decrypt(rotated, "bob", "new-password")
	if err != nil {
		t.Fatalf("Decrypt recovered vault: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("recovered data mismatch: got %+v want %+v", got, want)
	}
	if _, _, err := Recover(rotated, rotatedRegistration.RecoveryKey, "carol", "another-password"); err != nil {
		t.Fatalf("new recovery key should work: %v", err)
	}
}

func TestUpdatePreservesIdentityAndRecoveryKey(t *testing.T) {
	registration, ciphertext, err := Create("alice", "old-password", testStore())
	if err != nil {
		t.Fatal(err)
	}
	want := &config.Store{DB: []config.DBProfile{{Name: "reporting", Type: "postgres", Host: "db", Port: 5432}}}

	updated, err := Update(ciphertext, registration.Name, "old-password", want)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	updatedRegistration, err := envelopeRegistration(updated)
	if err != nil {
		t.Fatal(err)
	}
	if updatedRegistration.ID != registration.ID || updatedRegistration.Name != registration.Name {
		t.Fatalf("Update changed identity: got %+v want %+v", updatedRegistration, registration)
	}
	if _, err := Decrypt(updated, registration.Name, "old-password"); err != nil {
		t.Fatalf("updated vault should decrypt: %v", err)
	}
	got, err := Decrypt(updated, registration.Name, "old-password")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("updated profiles mismatch: got %+v want %+v", got, want)
	}
	if _, _, err := Recover(updated, registration.RecoveryKey, "bob", "new-password"); err != nil {
		t.Fatalf("original recovery key should remain valid: %v", err)
	}
}

func envelopeRegistration(data []byte) (Registration, error) {
	env, err := parse(data)
	if err != nil {
		return Registration{}, err
	}
	return Registration{ID: env.ID, Name: env.Name}, nil
}

func TestCreateValidatesCredentials(t *testing.T) {
	for _, tc := range []struct {
		name     string
		username string
		password string
	}{
		{name: "missing name", username: "", password: "secret"},
		{name: "missing password", username: "alice", password: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Create(tc.username, tc.password, testStore()); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDecryptRejectsMalformedEnvelope(t *testing.T) {
	if _, err := Decrypt([]byte(`{"version":999}`), "alice", "secret"); err == nil || errors.Is(err, ErrInvalidEnvelope) == false {
		t.Fatalf("expected invalid envelope error, got %v", err)
	}

	_, ciphertext, err := Create("alice", "secret", testStore())
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(ciphertext, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["ciphertext"] = "not-base64"
	corrupted, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(corrupted, "alice", "secret"); err == nil {
		t.Fatal("expected corrupted envelope error")
	}
}
