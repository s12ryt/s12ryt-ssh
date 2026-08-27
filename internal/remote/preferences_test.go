package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewClientValidatesBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantURL string
		wantErr bool
	}{
		{name: "https", rawURL: "https://auth.example.com/", wantURL: "https://auth.example.com"},
		{name: "loopback http", rawURL: "http://127.0.0.1:8787", wantURL: "http://127.0.0.1:8787"},
		{name: "nested base path", rawURL: "https://example.com/auth/", wantURL: "https://example.com/auth"},
		{name: "missing scheme", rawURL: "auth.example.com", wantErr: true},
		{name: "unsupported scheme", rawURL: "ftp://auth.example.com", wantErr: true},
		{name: "missing host", rawURL: "https:///auth", wantErr: true},
		{name: "credentials", rawURL: "https://user:pass@auth.example.com", wantErr: true},
		{name: "query", rawURL: "https://auth.example.com?tenant=a", wantErr: true},
		{name: "fragment", rawURL: "https://auth.example.com/#section", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(test.rawURL, nil)
			if test.wantErr {
				if err == nil {
					t.Fatal("NewClient() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := client.BaseURL(); got != test.wantURL {
				t.Fatalf("BaseURL() = %q, want %q", got, test.wantURL)
			}
		})
	}
}

func TestPreferencesPersistOnlyNonSensitiveRemoteLoginFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote-preferences.json")
	want := Preferences{
		BaseURL:  "https://auth.example.com",
		Username: "alice",
		DeviceID: "desktop-123",
	}
	if err := SavePreferences(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("LoadPreferences() = %+v, want %+v", got, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"password", "accesstoken", "refreshtoken", "secret"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("preferences contain sensitive field %q: %s", forbidden, data)
		}
	}
}

func TestLoadPreferencesFallsBackForMissingOrInvalidFiles(t *testing.T) {
	missing, err := LoadPreferences(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if missing != (Preferences{}) {
		t.Fatalf("missing preferences = %+v", missing)
	}

	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"base_url":"https://example.com","username":"alice","device_id":"device"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if invalid != (Preferences{}) {
		t.Fatalf("invalid preferences = %+v", invalid)
	}
}
