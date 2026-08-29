package i18n

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageDefaultsToEnglishAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")

	prefs, err := LoadPreferences(path)
	if err != nil {
		t.Fatalf("LoadPreferences missing file: %v", err)
	}
	if prefs.Language != English {
		t.Fatalf("default language = %q, want %q", prefs.Language, English)
	}

	prefs.Language = TraditionalChinese
	if err := SavePreferences(path, prefs); err != nil {
		t.Fatalf("SavePreferences: %v", err)
	}
	loaded, err := LoadPreferences(path)
	if err != nil {
		t.Fatalf("LoadPreferences saved file: %v", err)
	}
	if loaded.Language != TraditionalChinese {
		t.Fatalf("loaded language = %q, want %q", loaded.Language, TraditionalChinese)
	}
}

func TestInvalidPreferencesFallBackToEnglishWithoutSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"language":"xx","password":"must-not-be-used"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prefs, err := LoadPreferences(path)
	if err != nil {
		t.Fatalf("LoadPreferences invalid file: %v", err)
	}
	if prefs.Language != English {
		t.Fatalf("invalid language = %q, want %q", prefs.Language, English)
	}
}

func TestTranslationsExistInBothLanguages(t *testing.T) {
	for _, key := range Keys() {
		for _, language := range []Language{English, TraditionalChinese} {
			value := T(language, key)
			if value == "" || value == string(key) {
				t.Fatalf("missing translation for %q in %q: %q", key, language, value)
			}
		}
	}
	if got := T(TraditionalChinese, KeyLanguageToggle); got != "EN" {
		t.Fatalf("Chinese toggle label = %q, want EN", got)
	}
	if got := T(English, KeyLanguageToggle); got != "中" {
		t.Fatalf("English toggle label = %q, want 中", got)
	}
	if strings.Contains(T(English, KeyStatusWorking), "status.") {
		t.Error("translation unexpectedly exposes an internal key")
	}
}

func TestGUIStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Secure remote workspace",
		"Log out",
		"Working...",
		"Operation failed.",
		"Connect",
		"SSH hosts",
		"No SSH hosts yet.",
		"New host",
		"SSH host details",
		"Save host",
		"Delete host",
		"Delete SSH host?",
		"Trust this host key?",
		"Close terminal",
		"Terminal input",
		"Send",
		"Terminal output will appear here",
		"SSH terminal is not connected",
		"SSH terminal is already connected",
		"Select or save an SSH host first",
		"Loading SSH hosts...",
		"Saving SSH host...",
		"Deleting SSH host...",
		"Trusting host key...",
		"Connecting to SSH host...",
		"SSH terminal connected.",
		"SSH terminal closed.",
		"Name",
		"Host",
		"Port",
		"Username",
		"Password",
		"Private key",
		"Key passphrase",
		"Host fingerprint",
		"name is required",
		"host is required",
		"username is required",
		"password or private key is required",
		"port must be between 1 and 65535",
		"Sign in with authentication service",
		"Use a complete HTTP or HTTPS URL. The password is never saved.",
		"Authentication service URL",
		"Account",
		"Sign in remotely",
		"Restore saved session",
		"Remote workspace",
		"Remote account: ",
		"Remote sign-in URL, account, and password are required",
		"Signing in to authentication service...",
		"Restoring remote session...",
		"Signing out...",
		"Remote workspace ready.",
		"Sign in to the remote authentication service.",
		"Remote authentication service is unavailable",
		"SSH access is not enabled for this account.",
		"Cancel",
		"Confirm",
		"Show",
		"Hide",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("GUI string %q was not translated", source)
		}
	}
	if got := Text(TraditionalChinese, "Could not save language preference: permission denied"); got != "無法儲存語言偏好：permission denied" {
		t.Fatalf("preference save error = %q", got)
	}
	if got := Text(TraditionalChinese, "remote service: AccessDenied"); got != "remote service: AccessDenied" {
		t.Fatalf("external error was translated: %q", got)
	}
}
