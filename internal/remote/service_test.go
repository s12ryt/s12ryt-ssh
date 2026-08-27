package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/securestore"
)

func TestServiceLoginPersistsNonSensitivePreferencesAndReusesDevice(t *testing.T) {
	var mu sync.Mutex
	var deviceIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		var input loginRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		deviceIDs = append(deviceIDs, input.DeviceID)
		index := len(deviceIDs)
		mu.Unlock()
		writeJSON(w, TokenPair{
			AccessToken: "access", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh-" + string(rune('0'+index)), RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
			Account: Account{ID: "account-1", Username: input.Username}, SessionID: "session-1",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	service := NewService(path, securestore.NewMemoryStore(), server.Client())
	if _, err := service.Login(context.Background(), server.URL+"/", "alice", "long-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(context.Background(), server.URL, "alice", "another-password"); err != nil {
		t.Fatal(err)
	}

	preferences, err := service.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if preferences.BaseURL != server.URL || preferences.Username != "alice" || preferences.DeviceID == "" {
		t.Fatalf("preferences = %+v", preferences)
	}
	mu.Lock()
	if len(deviceIDs) != 2 || deviceIDs[0] == "" || deviceIDs[0] != deviceIDs[1] {
		t.Fatalf("device IDs = %v", deviceIDs)
	}
	mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, "password") || strings.Contains(lower, "refresh") || strings.Contains(lower, "access") {
		t.Fatalf("preferences contain sensitive fields: %s", data)
	}
}

func TestServiceRestoreUsesSavedPreferencesAndRotatesSecureToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/refresh" {
			http.NotFound(w, r)
			return
		}
		var input refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if input.RefreshToken != "saved-refresh" || input.DeviceID != "device-1" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid refresh token")
			return
		}
		writeJSON(w, TokenPair{
			AccessToken: "access-2", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "rotated-refresh", RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
			Account: Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	preferences := Preferences{BaseURL: server.URL, Username: "alice", DeviceID: "device-1"}
	if err := SavePreferences(path, preferences); err != nil {
		t.Fatal(err)
	}
	secrets := securestore.NewMemoryStore()
	if err := secrets.Save(refreshTokenKey(server.URL, "alice", "device-1"), []byte("saved-refresh")); err != nil {
		t.Fatal(err)
	}
	service := NewService(path, secrets, server.Client())
	session, err := service.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if session.Account().Username != "alice" {
		t.Fatalf("account = %+v", session.Account())
	}
	stored, err := secrets.Load(refreshTokenKey(server.URL, "alice", "device-1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != "rotated-refresh" {
		t.Fatalf("stored token = %q", stored)
	}
}

func TestServiceRestoreRequiresCompletePreferences(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "missing.json"), securestore.NewMemoryStore(), nil)
	if _, err := service.Restore(context.Background()); err != ErrNoPreferences {
		t.Fatalf("Restore() error = %v", err)
	}
}
