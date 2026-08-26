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
	if strings.Contains(T(English, KeyStatusReady), "status.") {
		t.Error("translation unexpectedly exposes an internal key")
	}
}

func TestBytesTranslationUsesHumanReadableEnglishLabel(t *testing.T) {
	if got := T(English, KeyBytes); got != "Bytes" {
		t.Fatalf("English bytes label = %q, want %q", got, "Bytes")
	}
	if got := Text(TraditionalChinese, "Bytes"); got != "位元組" {
		t.Fatalf("Chinese bytes label = %q, want %q", got, "位元組")
	}
}

func TestGUIStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Save this recovery key before continuing.",
		"Rotating recovery credentials...",
		"Connecting to SSH host...",
		"Unlocking encrypted vault...",
		"PostgreSQL SSL mode (default require)",
		"Bytes",
		"vault name and password are required",
		"S3 endpoint, bucket, access key, and secret key are required",
		"SQL type, host, port, user, password, and database are required",
		"recovery key, new vault name, and new vault password are required",
		"SSH profiles",
		"Storage profiles",
		"Database profiles",
		"New",
		"New profile",
		"Name",
		"Key path",
		"Key passphrase",
		"Host fingerprint",
		"Vault bucket",
		" to ",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("GUI string %q was not translated", source)
		}
	}
	if got := Text(TraditionalChinese, "Could not save language preference: permission denied"); got != "無法保存語言偏好：permission denied" {
		t.Fatalf("preference save error = %q", got)
	}
	if got := Text(TraditionalChinese, "remote service: AccessDenied"); got != "remote service: AccessDenied" {
		t.Fatalf("external error was translated: %q", got)
	}
}
