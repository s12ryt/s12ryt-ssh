package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/securestore"
)

type sshFixture struct {
	server  *httptest.Server
	mu      sync.Mutex
	_calls  []string
	bodies  []string
	hosts   map[string]SSHHost
	secrets map[string]SSHHostInput
	nextID  int
}

func (f *sshFixture) record(call, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f._calls = append(f._calls, call)
	f.bodies = append(f.bodies, body)
}

func (f *sshFixture) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f._calls...)
}

func (f *sshFixture) lastBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		return ""
	}
	return f.bodies[len(f.bodies)-1]
}

func newSSHFixture(t *testing.T) *sshFixture {
	t.Helper()
	fixture := &sshFixture{hosts: map[string]SSHHost{}, secrets: map[string]SSHHostInput{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/auth/api/v1/resources", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-1" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid access token")
			return
		}
		writeJSON(w, map[string]any{
			"resources":  []Resource{},
			"sshEnabled": true,
		})
	})
	mux.HandleFunc("/auth/api/v1/ssh/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-1" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid access token")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/auth/api/v1/ssh/")
		body := ""
		if r.Body != nil {
			data := make([]byte, 4096)
			n, _ := r.Body.Read(data)
			body = string(data[:n])
		}
		fixture.record(r.Method+" /api/v1/ssh/"+path, body)

		if path == "hosts" && r.Method == http.MethodPost {
			var input SSHHostInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			fixture.nextID++
			id := fmt.Sprintf("host-%d", fixture.nextID)
			fixture.secrets[id] = input
			host := SSHHost{
				ID: id, Name: input.Name, Host: input.Host, Port: input.Port, Username: input.Username,
				HasPassword: input.Password != "", HasPrivateKey: input.PrivateKey != "",
				HasKeyPassphrase: input.KeyPassphrase != "",
				CreatedAt:        1700000000000, UpdatedAt: 1700000000000,
			}
			fixture.hosts[id] = host
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, host)
			return
		}
		if path == "hosts" && r.Method == http.MethodGet {
			fixture.mu.Lock()
			hosts := make([]SSHHost, 0, len(fixture.hosts))
			for _, host := range fixture.hosts {
				hosts = append(hosts, host)
			}
			fixture.mu.Unlock()
			writeJSON(w, map[string]any{"hosts": hosts})
			return
		}
		rest, found := strings.CutPrefix(path, "hosts/")
		if !found || rest == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		hostID, suffix, _ := strings.Cut(rest, "/")
		fixture.mu.Lock()
		host, exists := fixture.hosts[hostID]
		secret := fixture.secrets[hostID]
		fixture.mu.Unlock()
		if !exists {
			writeAPIError(w, http.StatusNotFound, "not_found", "ssh host not found")
			return
		}
		switch {
		case suffix == "" && r.Method == http.MethodPatch:
			var input SSHHostInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			host.Name, host.Host, host.Port, host.Username = input.Name, input.Host, input.Port, input.Username
			fixture.mu.Lock()
			fixture.hosts[hostID] = host
			fixture.mu.Unlock()
			writeJSON(w, host)
		case suffix == "" && r.Method == http.MethodDelete:
			fixture.mu.Lock()
			delete(fixture.hosts, hostID)
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case suffix == "credentials" && r.Method == http.MethodGet:
			writeJSON(w, SSHHostCredentials{
				ID: host.ID, Name: host.Name, Host: host.Host, Port: host.Port, Username: host.Username,
				Password: secret.Password, PrivateKey: secret.PrivateKey, KeyPassphrase: secret.KeyPassphrase,
				TrustedFingerprint: host.TrustedFingerprint,
			})
		case suffix == "fingerprint" && r.Method == http.MethodPut:
			var input struct {
				Fingerprint string `json:"fingerprint"`
			}
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			host.TrustedFingerprint = input.Fingerprint
			fixture.mu.Lock()
			fixture.hosts[hostID] = host
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *sshFixture) loginSession(t *testing.T) *Session {
	t.Helper()
	client, err := NewClient(f.server.URL+"/auth", f.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Login(context.Background(), client, securestore.NewMemoryStore(), "alice", "long-password", "desktop-a")
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSessionSSHHostLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	hosts, err := session.SSHHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("initial hosts = %+v", hosts)
	}

	created, err := session.CreateSSHHost(ctx, SSHHostInput{
		Name: "web", Host: "web.example.com", Port: 22, Username: "deploy", Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "host-1" || created.Name != "web" || created.Port != 22 || !created.HasPassword || created.HasPrivateKey {
		t.Fatalf("created host = %+v", created)
	}
	if created.TrustedFingerprint != "" {
		t.Fatalf("created fingerprint = %q", created.TrustedFingerprint)
	}
	if !strings.Contains(fixture.lastBody(), `"password":"hunter2hunter2"`) {
		t.Fatalf("create body = %s", fixture.lastBody())
	}

	hosts, err = session.SSHHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].ID != "host-1" {
		t.Fatalf("hosts after create = %+v", hosts)
	}

	updated, err := session.UpdateSSHHost(ctx, "host-1", SSHHostInput{
		Name: "web-2", Host: "web2.example.com", Port: 2222, Username: "deploy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "web-2" || updated.Host != "web2.example.com" || updated.Port != 2222 {
		t.Fatalf("updated host = %+v", updated)
	}

	credentials, err := session.SSHHostCredentials(ctx, "host-1")
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Host != "web2.example.com" || credentials.Port != 2222 || credentials.Username != "deploy" {
		t.Fatalf("credentials = %+v", credentials)
	}
	if credentials.Password != "hunter2hunter2" {
		t.Fatalf("credentials password = %q", credentials.Password)
	}

	if err := session.SetSSHHostFingerprint(ctx, "host-1", "SHA256:abc123"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fixture.lastBody(), `"fingerprint":"SHA256:abc123"`) {
		t.Fatalf("fingerprint body = %s", fixture.lastBody())
	}

	hosts, err = session.SSHHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].TrustedFingerprint != "SHA256:abc123" {
		t.Fatalf("hosts after fingerprint = %+v", hosts)
	}

	if err := session.DeleteSSHHost(ctx, "host-1"); err != nil {
		t.Fatal(err)
	}
	hosts, err = session.SSHHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("hosts after delete = %+v", hosts)
	}
}

func TestResourcesOverviewDecodesSSHEnabled(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)

	overview, err := session.ResourcesOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !overview.SSHEnabled {
		t.Fatalf("overview = %+v", overview)
	}
	if len(overview.Resources) != 0 {
		t.Fatalf("overview resources = %+v", overview.Resources)
	}

	resources, err := session.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 0 {
		t.Fatalf("resources = %+v", resources)
	}
}

func TestSSHHostErrorsSurfaceAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			writeJSON(w, TokenPair{
				AccessToken: "access-1", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
				RefreshToken: "refresh-1", RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
				Account: Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
			})
			return
		}
		writeAPIError(w, http.StatusForbidden, "ssh_disabled", "SSH access is disabled for this account")
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL+"/auth", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Login(context.Background(), client, securestore.NewMemoryStore(), "alice", "long-password", "desktop-a")
	if err != nil {
		t.Fatal(err)
	}

	_, err = session.CreateSSHHost(context.Background(), SSHHostInput{Name: "web", Host: "h", Port: 22, Username: "u", Password: "p"})
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("CreateSSHHost() error = %T %v", err, err)
	}
	if apiError.StatusCode != http.StatusForbidden || apiError.Code != "ssh_disabled" {
		t.Fatalf("API error = %+v", apiError)
	}
}
