package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/securestore"
)

type authFixture struct {
	server         *httptest.Server
	mu             sync.Mutex
	login          int
	refresh        int
	logout         int
	resourceTokens []string
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	fixture := &authFixture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.login++
		fixture.mu.Unlock()
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
			DeviceID string `json:"deviceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if input.Username != "alice" || input.Password != "long-password" || input.DeviceID != "desktop-a" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
			return
		}
		writeJSON(w, TokenPair{
			AccessToken: "access-1", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh-1", RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			Account: Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	})
	mux.HandleFunc("/auth/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.refresh++
		fixture.mu.Unlock()
		var input struct {
			RefreshToken string `json:"refreshToken"`
			DeviceID     string `json:"deviceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if input.RefreshToken != "refresh-1" || input.DeviceID != "desktop-a" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid refresh token")
			return
		}
		writeJSON(w, TokenPair{
			AccessToken: "access-2", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
			RefreshToken: "refresh-2", RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			Account: Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
		})
	})
	mux.HandleFunc("/auth/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.logout++
		fixture.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer access-2" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid access token")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/auth/api/v1/resources", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		fixture.mu.Lock()
		fixture.resourceTokens = append(fixture.resourceTokens, token)
		attempt := len(fixture.resourceTokens)
		fixture.mu.Unlock()
		if token == "access-1" && attempt > 1 {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "expired access token")
			return
		}
		if token != "access-1" && token != "access-2" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid access token")
			return
		}
		writeJSON(w, map[string]any{"resources": []Resource{
			{ID: "storage-1", Name: "assigned-storage", Kind: "s3", Enabled: true, Operations: []Operation{"s3.read", "s3.write"}},
			{ID: "database-1", Name: "assigned-db", Kind: "postgres", Enabled: true, Operations: []Operation{"sql.tables", "sql.query"}},
		}})
	})
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)
	return fixture
}

func TestLoginPersistsOnlyRefreshTokenAndListsResources(t *testing.T) {
	fixture := newAuthFixture(t)
	client, err := NewClient(fixture.server.URL+"/auth", fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	secrets := securestore.NewMemoryStore()
	session, err := Login(context.Background(), client, secrets, "alice", "long-password", "desktop-a")
	if err != nil {
		t.Fatal(err)
	}

	stored, err := secrets.Load(refreshTokenKey(client.BaseURL(), "alice", "desktop-a"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != "refresh-1" {
		t.Fatalf("stored refresh token = %q", stored)
	}
	if strings.Contains(string(stored), "long-password") || strings.Contains(string(stored), "access-1") {
		t.Fatal("secure store contains password or access token")
	}

	resources, err := session.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 || resources[0].Operations[0] != "s3.read" {
		t.Fatalf("resources = %+v", resources)
	}
}

func TestRestoreRotatesRefreshTokenAndUnauthorizedRequestRetriesOnce(t *testing.T) {
	fixture := newAuthFixture(t)
	client, err := NewClient(fixture.server.URL+"/auth", fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	secrets := securestore.NewMemoryStore()
	key := refreshTokenKey(client.BaseURL(), "alice", "desktop-a")
	if err := secrets.Save(key, []byte("refresh-1")); err != nil {
		t.Fatal(err)
	}

	session, err := Restore(context.Background(), client, secrets, "alice", "desktop-a")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := secrets.Load(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != "refresh-2" {
		t.Fatalf("rotated refresh token = %q", stored)
	}

	resources, err := session.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 {
		t.Fatalf("resources = %+v", resources)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.refresh != 1 {
		t.Fatalf("refresh count = %d", fixture.refresh)
	}
}

func TestSessionRefreshesAfterUnauthorizedAndLogoutDeletesRefresh(t *testing.T) {
	fixture := newAuthFixture(t)
	client, err := NewClient(fixture.server.URL+"/auth", fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	secrets := securestore.NewMemoryStore()
	session, err := Login(context.Background(), client, secrets, "alice", "long-password", "desktop-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Resources(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Resources(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := session.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = secrets.Load(refreshTokenKey(client.BaseURL(), "alice", "desktop-a"))
	if !errors.Is(err, securestore.ErrNotFound) {
		t.Fatalf("refresh token after logout error = %v", err)
	}

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.refresh != 1 || fixture.logout != 1 {
		t.Fatalf("refresh=%d logout=%d", fixture.refresh, fixture.logout)
	}
	if got := strings.Join(fixture.resourceTokens, ","); got != "access-1,access-1,access-2" {
		t.Fatalf("resource tokens = %s", got)
	}
}

func TestLoginReturnsStructuredAPIError(t *testing.T) {
	fixture := newAuthFixture(t)
	client, err := NewClient(fixture.server.URL+"/auth", fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = Login(context.Background(), client, securestore.NewMemoryStore(), "alice", "wrong", "desktop-a")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("Login() error = %T %v", err, err)
	}
	if apiError.StatusCode != http.StatusUnauthorized || apiError.Code != "invalid_credentials" {
		t.Fatalf("API error = %+v", apiError)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
