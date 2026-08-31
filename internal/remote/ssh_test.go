package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/securestore"
)

type sshFixture struct {
	server            *httptest.Server
	mu                sync.Mutex
	_calls            []string
	bodies            []string
	hosts             map[string]SSHHost
	secrets           map[string]SSHHostInput
	tunnels           map[string]SSHTunnelRule
	snippets          map[string]SSHCommandSnippet
	snippetSecrets    map[string]map[string]string
	keys              map[string]SSHKeyIdentity
	keySecrets        map[string]SSHKeyIdentitySecrets
	fingerprints      map[string][]SSHHostFingerprint
	history           map[string]SSHSessionHistory
	nextID            int
	nextTunnelID      int
	nextSnippetID     int
	nextKeyID         int
	nextFingerprintID int
	nextHistoryID     int
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
	fixture := &sshFixture{
		hosts:          map[string]SSHHost{},
		secrets:        map[string]SSHHostInput{},
		tunnels:        map[string]SSHTunnelRule{},
		snippets:       map[string]SSHCommandSnippet{},
		snippetSecrets: map[string]map[string]string{},
		keys:           map[string]SSHKeyIdentity{},
		keySecrets:     map[string]SSHKeyIdentitySecrets{},
		fingerprints:   map[string][]SSHHostFingerprint{},
		history:        map[string]SSHSessionHistory{},
	}
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

		if path == "preferences" && r.Method == http.MethodGet {
			writeJSON(w, SSHWorkspacePreferences{
				TerminalAppearance: SSHTerminalAppearance{
					Font:       SSHTerminalFontBuiltin,
					FontSize:   13,
					Foreground: "#d7e6e2",
					Background: "#101c1b",
				},
				Version: 1,
			})
			return
		}
		if path == "preferences" && r.Method == http.MethodPatch {
			var input SSHWorkspacePreferencesInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, SSHWorkspacePreferences{
				TerminalAppearance: input.TerminalAppearance,
				Version:            2,
			})
			return
		}

		if path == "workspace/export" && r.Method == http.MethodPost {
			writeJSON(w, map[string]string{"package": "encrypted-workspace-package"})
			return
		}
		if path == "workspace/import/preview" && r.Method == http.MethodPost {
			writeJSON(w, SSHWorkspaceImportPreview{
				IncludesSecrets: true,
				Counts: SSHWorkspaceResourceCounts{
					Hosts: 1,
				},
				Conflicts: []SSHWorkspaceImportConflict{{
					Kind:     SSHWorkspaceImportHost,
					Name:     "web",
					Conflict: true,
				}},
			})
			return
		}
		if path == "workspace/import/apply" && r.Method == http.MethodPost {
			writeJSON(w, SSHWorkspaceImportResult{
				IncludesSecrets: true,
				Counts: SSHWorkspaceImportApplyCounts{
					Copied: 1,
				},
				Items: []SSHWorkspaceImportPlanItem{{
					Kind:       SSHWorkspaceImportHost,
					Name:       "web",
					Action:     SSHWorkspaceImportCopy,
					TargetName: "web (2)",
				}},
			})
			return
		}

		if path == "session-history" && r.Method == http.MethodPost {
			var input SSHSessionHistoryInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			host, exists := fixture.hosts[input.HostID]
			if !exists {
				fixture.mu.Unlock()
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh host not found")
				return
			}
			fixture.nextHistoryID++
			id := fmt.Sprintf("history-%d", fixture.nextHistoryID)
			startedAt := int64(1700000000000 + fixture.nextHistoryID)
			history := SSHSessionHistory{
				ID: id, HostID: input.HostID, HostName: host.Name,
				Status: input.Status, StartedAt: startedAt,
			}
			if input.LatencyMS != nil {
				history.LatencyMS = *input.LatencyMS
			}
			if input.ErrorMessage != nil {
				history.ErrorMessage = *input.ErrorMessage
			}
			if input.Status == SSHSessionFailed || input.Status == SSHSessionClosed {
				history.EndedAt = &startedAt
			}
			fixture.history[id] = history
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, history)
			return
		}
		if path == "session-history" && r.Method == http.MethodGet {
			fixture.mu.Lock()
			history := make([]SSHSessionHistory, 0, len(fixture.history))
			for _, entry := range fixture.history {
				history = append(history, entry)
			}
			fixture.mu.Unlock()
			sort.Slice(history, func(i, j int) bool {
				return history[i].StartedAt > history[j].StartedAt
			})
			writeJSON(w, map[string]any{"history": history})
			return
		}
		if historyID, found := strings.CutPrefix(path, "session-history/"); found && historyID != "" && r.Method == http.MethodPatch {
			var input SSHSessionHistoryUpdate
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			history, exists := fixture.history[historyID]
			if !exists {
				fixture.mu.Unlock()
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh session history not found")
				return
			}
			history.Status = input.Status
			if input.LatencyMS != nil {
				history.LatencyMS = *input.LatencyMS
			}
			if input.ErrorMessage != nil {
				history.ErrorMessage = *input.ErrorMessage
			}
			if input.Status == SSHSessionFailed || input.Status == SSHSessionClosed {
				endedAt := int64(1700000001000 + fixture.nextHistoryID)
				history.EndedAt = &endedAt
			} else {
				history.EndedAt = nil
			}
			fixture.history[historyID] = history
			fixture.mu.Unlock()
			writeJSON(w, history)
			return
		}

		if path == "keys" && r.Method == http.MethodPost {
			var input SSHKeyIdentityInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			enabled := true
			if input.Enabled != nil {
				enabled = *input.Enabled
			}
			fixture.mu.Lock()
			fixture.nextKeyID++
			id := fmt.Sprintf("key-%d", fixture.nextKeyID)
			identity := SSHKeyIdentity{
				ID: id, Name: input.Name, PublicKey: input.PublicKey,
				Fingerprint: input.Fingerprint, HasPassphrase: input.KeyPassphrase != "",
				Enabled: enabled, Version: 1,
			}
			fixture.keys[id] = identity
			fixture.keySecrets[id] = SSHKeyIdentitySecrets{
				PrivateKey: input.PrivateKey, KeyPassphrase: input.KeyPassphrase,
			}
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, identity)
			return
		}
		if path == "keys" && r.Method == http.MethodGet {
			fixture.mu.Lock()
			keys := make([]SSHKeyIdentity, 0, len(fixture.keys))
			for _, identity := range fixture.keys {
				keys = append(keys, identity)
			}
			fixture.mu.Unlock()
			writeJSON(w, map[string]any{"keys": keys})
			return
		}
		if keyPath, found := strings.CutPrefix(path, "keys/"); found && keyPath != "" {
			keyID, suffix, _ := strings.Cut(keyPath, "/")
			fixture.mu.Lock()
			identity, exists := fixture.keys[keyID]
			secrets := fixture.keySecrets[keyID]
			fixture.mu.Unlock()
			if !exists {
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh key identity not found")
				return
			}
			switch {
			case suffix == "secrets" && r.Method == http.MethodGet:
				writeJSON(w, secrets)
				return
			case suffix == "" && r.Method == http.MethodPatch:
				var input SSHKeyIdentityInput
				if err := json.Unmarshal([]byte(body), &input); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				identity.Name, identity.PublicKey = input.Name, input.PublicKey
				identity.Fingerprint = input.Fingerprint
				if input.Enabled != nil {
					identity.Enabled = *input.Enabled
				}
				identity.Version++
				if input.PrivateKey != "" || input.KeyPassphrase != "" {
					secrets = SSHKeyIdentitySecrets{
						PrivateKey: input.PrivateKey, KeyPassphrase: input.KeyPassphrase,
					}
					identity.HasPassphrase = input.KeyPassphrase != ""
				}
				fixture.mu.Lock()
				fixture.keys[keyID] = identity
				fixture.keySecrets[keyID] = secrets
				fixture.mu.Unlock()
				writeJSON(w, identity)
				return
			case suffix == "" && r.Method == http.MethodDelete:
				fixture.mu.Lock()
				delete(fixture.keys, keyID)
				delete(fixture.keySecrets, keyID)
				fixture.mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if path == "snippets" && r.Method == http.MethodPost {
			var input SSHCommandSnippetInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			enabled := true
			if input.Enabled != nil {
				enabled = *input.Enabled
			}
			fixture.mu.Lock()
			fixture.nextSnippetID++
			id := fmt.Sprintf("snippet-%d", fixture.nextSnippetID)
			secretNames := make([]string, 0, len(input.Secrets))
			secrets := make(map[string]string, len(input.Secrets))
			for name, value := range input.Secrets {
				secretNames = append(secretNames, name)
				secrets[name] = value
			}
			sort.Strings(secretNames)
			snippet := SSHCommandSnippet{
				ID: id, Name: input.Name, Command: input.Command,
				Variables: append([]string(nil), input.Variables...), SecretNames: secretNames,
				Enabled: enabled, Version: 1, CreatedAt: 1700000000000, UpdatedAt: 1700000000000,
			}
			fixture.snippets[id] = snippet
			fixture.snippetSecrets[id] = secrets
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, snippet)
			return
		}
		if path == "snippets" && r.Method == http.MethodGet {
			fixture.mu.Lock()
			snippets := make([]SSHCommandSnippet, 0, len(fixture.snippets))
			for _, snippet := range fixture.snippets {
				snippets = append(snippets, snippet)
			}
			fixture.mu.Unlock()
			writeJSON(w, map[string]any{"snippets": snippets})
			return
		}
		if snippetPath, found := strings.CutPrefix(path, "snippets/"); found && snippetPath != "" {
			snippetID, suffix, _ := strings.Cut(snippetPath, "/")
			fixture.mu.Lock()
			snippet, exists := fixture.snippets[snippetID]
			fixture.mu.Unlock()
			if !exists {
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh snippet not found")
				return
			}
			switch {
			case suffix == "secrets" && r.Method == http.MethodGet:
				fixture.mu.Lock()
				secrets := make(map[string]string, len(fixture.snippetSecrets[snippetID]))
				for name, value := range fixture.snippetSecrets[snippetID] {
					secrets[name] = value
				}
				fixture.mu.Unlock()
				writeJSON(w, secrets)
				return
			case suffix == "" && r.Method == http.MethodPatch:
				var input SSHCommandSnippetInput
				if err := json.Unmarshal([]byte(body), &input); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if input.Enabled != nil {
					snippet.Enabled = *input.Enabled
				}
				snippet.Name, snippet.Command = input.Name, input.Command
				snippet.Variables = append([]string(nil), input.Variables...)
				snippet.Version++
				fixture.mu.Lock()
				fixture.snippets[snippetID] = snippet
				if input.Secrets != nil {
					secrets := make(map[string]string, len(input.Secrets))
					for name, value := range input.Secrets {
						secrets[name] = value
					}
					fixture.snippetSecrets[snippetID] = secrets
					snippet.SecretNames = make([]string, 0, len(secrets))
					for name := range secrets {
						snippet.SecretNames = append(snippet.SecretNames, name)
					}
					sort.Strings(snippet.SecretNames)
					fixture.snippets[snippetID] = snippet
				}
				fixture.mu.Unlock()
				writeJSON(w, snippet)
				return
			case suffix == "" && r.Method == http.MethodDelete:
				fixture.mu.Lock()
				delete(fixture.snippets, snippetID)
				delete(fixture.snippetSecrets, snippetID)
				fixture.mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if path == "tunnels" && r.Method == http.MethodPost {
			var input SSHTunnelInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			fixture.nextTunnelID++
			id := fmt.Sprintf("tunnel-%d", fixture.nextTunnelID)
			tunnel := SSHTunnelRule{
				ID: id, Name: input.Name, HostID: input.HostID, Type: input.Type,
				ListenHost: input.ListenHost, ListenPort: input.ListenPort,
				TargetHost: input.TargetHost, TargetPort: input.TargetPort,
				Enabled: input.Enabled, AutoStart: input.AutoStart, Version: 1,
			}
			fixture.tunnels[id] = tunnel
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, tunnel)
			return
		}
		if path == "tunnels" && r.Method == http.MethodGet {
			fixture.mu.Lock()
			tunnels := make([]SSHTunnelRule, 0, len(fixture.tunnels))
			for _, tunnel := range fixture.tunnels {
				tunnels = append(tunnels, tunnel)
			}
			fixture.mu.Unlock()
			writeJSON(w, map[string]any{"tunnels": tunnels})
			return
		}
		if tunnelPath, found := strings.CutPrefix(path, "tunnels/"); found && strings.HasSuffix(tunnelPath, "/runtime") && r.Method == http.MethodPatch {
			tunnelID := strings.TrimSuffix(tunnelPath, "/runtime")
			fixture.mu.Lock()
			tunnel, exists := fixture.tunnels[tunnelID]
			fixture.mu.Unlock()
			if !exists {
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh tunnel not found")
				return
			}
			var input SSHTunnelRuntimeUpdate
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			tunnel.Running = input.Running
			tunnel.TrafficUpBytes = input.TrafficUpBytes
			tunnel.TrafficDownBytes = input.TrafficDownBytes
			fixture.mu.Lock()
			fixture.tunnels[tunnelID] = tunnel
			fixture.mu.Unlock()
			writeJSON(w, tunnel)
			return
		}
		if tunnelID, found := strings.CutPrefix(path, "tunnels/"); found && tunnelID != "" {
			fixture.mu.Lock()
			tunnel, exists := fixture.tunnels[tunnelID]
			fixture.mu.Unlock()
			if !exists {
				writeAPIError(w, http.StatusNotFound, "not_found", "ssh tunnel not found")
				return
			}
			switch r.Method {
			case http.MethodPatch:
				var input SSHTunnelInput
				if err := json.Unmarshal([]byte(body), &input); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				tunnel.Name, tunnel.HostID, tunnel.Type = input.Name, input.HostID, input.Type
				tunnel.ListenHost, tunnel.ListenPort = input.ListenHost, input.ListenPort
				tunnel.TargetHost, tunnel.TargetPort = input.TargetHost, input.TargetPort
				tunnel.Enabled, tunnel.AutoStart, tunnel.Version = input.Enabled, input.AutoStart, tunnel.Version+1
				fixture.mu.Lock()
				fixture.tunnels[tunnelID] = tunnel
				fixture.mu.Unlock()
				writeJSON(w, tunnel)
				return
			case http.MethodDelete:
				fixture.mu.Lock()
				delete(fixture.tunnels, tunnelID)
				fixture.mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

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
			settings := SSHConnectionSettings{}
			if input.Settings != nil {
				settings = *input.Settings
			}
			host := SSHHost{
				ID: id, Name: input.Name, Host: input.Host, Port: input.Port, Username: input.Username,
				HasPassword: input.Password != "", HasPrivateKey: input.PrivateKey != "",
				HasKeyPassphrase: input.KeyPassphrase != "",
				Enabled:          input.Enabled, Favorite: input.Favorite, GroupPath: input.GroupPath,
				Tags: append([]string(nil), input.Tags...), SortOrder: input.SortOrder,
				AuthMethod: input.AuthMethod, Settings: settings, Version: 1,
				CreatedAt: 1700000000000, UpdatedAt: 1700000000000,
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
		case suffix == "clone" && r.Method == http.MethodPost:
			var input struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			fixture.nextID++
			cloneID := fmt.Sprintf("host-%d", fixture.nextID)
			clone := host
			clone.ID, clone.Name, clone.Version = cloneID, input.Name, 1
			fixture.hosts[cloneID] = clone
			fixture.secrets[cloneID] = secret
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, clone)
		case suffix == "" && r.Method == http.MethodPatch:
			var input SSHHostInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			host.Name, host.Host, host.Port, host.Username = input.Name, input.Host, input.Port, input.Username
			if input.Settings != nil {
				host.Settings = *input.Settings
			}
			if input.ClearTerminalAppearance {
				host.Settings.TerminalAppearance = nil
			}
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
				TrustedFingerprint: host.TrustedFingerprint, AuthMethod: host.AuthMethod,
				Settings: host.Settings, Version: host.Version,
			})
		case suffix == "fingerprint" && r.Method == http.MethodPut:
			var input struct {
				Fingerprint string                   `json:"fingerprint"`
				Source      SSHHostFingerprintSource `json:"source"`
			}
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			fixture.nextFingerprintID++
			retiredAt := int64(1700000000000 + fixture.nextFingerprintID)
			history := fixture.fingerprints[hostID]
			for index := range history {
				if history[index].Active {
					history[index].Active = false
					history[index].RetiredAt = &retiredAt
				}
			}
			algorithm, _, _ := strings.Cut(input.Fingerprint, ":")
			if input.Source == "" {
				input.Source = SSHHostFingerprintTOFU
			}
			history = append(history, SSHHostFingerprint{
				ID: fmt.Sprintf("fingerprint-%d", fixture.nextFingerprintID), HostID: hostID,
				Algorithm: algorithm, Fingerprint: input.Fingerprint, Source: input.Source,
				Active: true, ObservedAt: retiredAt,
			})
			fixture.fingerprints[hostID] = history
			host.TrustedFingerprint = input.Fingerprint
			host.Version++
			fixture.hosts[hostID] = host
			fixture.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case suffix == "fingerprints" && r.Method == http.MethodGet:
			fixture.mu.Lock()
			history := append([]SSHHostFingerprint(nil), fixture.fingerprints[hostID]...)
			fixture.mu.Unlock()
			writeJSON(w, map[string]any{"fingerprints": history})
		case suffix == "fingerprint" && r.Method == http.MethodDelete:
			fixture.mu.Lock()
			retiredAt := int64(1700000000000 + fixture.nextFingerprintID + 1)
			history := fixture.fingerprints[hostID]
			for index := range history {
				if history[index].Active {
					history[index].Active = false
					history[index].RetiredAt = &retiredAt
				}
			}
			fixture.fingerprints[hostID] = history
			host.TrustedFingerprint = ""
			host.Version++
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

func TestSessionSSHHostWorkspaceMetadataAndClone(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	settings := &SSHConnectionSettings{
		TCPTimeoutMS:          15000,
		SSHHandshakeTimeoutMS: 12000,
		PTYTimeoutMS:          9000,
		KeepaliveIntervalMS:   30000,
		FailureCount:          5,
		IdleTimeoutMS:         600000,
		Compression:           true,
		StartupCommand:        "cd /srv/app",
		InitialDirectory:      "/srv/app",
		Environment:           map[string]string{"APP_ENV": "production"},
		AutoReconnect:         true,
	}
	created, err := session.CreateSSHHost(context.Background(), SSHHostInput{
		Name: "production", Host: "prod.example.com", Port: 2222, Username: "deploy",
		PrivateKey: "private-key", Enabled: true, Favorite: true, GroupPath: "vps/production",
		Tags: []string{"critical", "web"}, SortOrder: 7, AuthMethod: SSHAuthPrivateKey,
		Settings: settings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Enabled || !created.Favorite || created.GroupPath != "vps/production" ||
		created.AuthMethod != SSHAuthPrivateKey || created.Version != 1 {
		t.Fatalf("created metadata = %+v", created)
	}
	if len(created.Tags) != 2 || created.Settings.InitialDirectory != "/srv/app" ||
		!created.Settings.Compression || !created.Settings.AutoReconnect {
		t.Fatalf("created settings = %+v", created)
	}
	if !strings.Contains(fixture.lastBody(), `"authMethod":"private_key"`) ||
		!strings.Contains(fixture.lastBody(), `"settings"`) {
		t.Fatalf("create body = %s", fixture.lastBody())
	}

	credentials, err := session.SSHHostCredentials(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if credentials.PrivateKey != "private-key" || credentials.AuthMethod != SSHAuthPrivateKey || credentials.Version != created.Version ||
		credentials.Settings.Environment["APP_ENV"] != "production" {
		t.Fatalf("credentials = %+v", credentials)
	}

	clone, err := session.CloneSSHHost(context.Background(), created.ID, "production copy")
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID == created.ID || clone.Name != "production copy" || !clone.Favorite ||
		clone.GroupPath != created.GroupPath || clone.AuthMethod != SSHAuthPrivateKey {
		t.Fatalf("clone = %+v", clone)
	}
	cloneCredentials, err := session.SSHHostCredentials(context.Background(), clone.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cloneCredentials.PrivateKey != "private-key" ||
		cloneCredentials.Settings.StartupCommand != "cd /srv/app" {
		t.Fatalf("clone credentials = %+v", cloneCredentials)
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
