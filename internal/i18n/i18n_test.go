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
		"The password is saved only when Remember password is enabled.",
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

func TestRemoteRememberPasswordStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Remember password",
		"The password is protected by Windows and kept after sign-out.",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Fatalf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestSSHWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"SSH terminal workspace",
		"Select an SSH host to open a terminal tab.",
		"Close",
		"Connecting",
		"Connected",
		"Connection failed",
		"Use Retry to try this host again, or Close to remove this tab.",
		"Connecting to SSH host...",
		"Edit",
		"New host",
		"New SSH host",
		"Edit SSH host",
		"Discard changes?",
		"This SSH host form has unsaved changes.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH workspace string %q was not translated", source)
		}
	}
}

func TestSSHWorkspaceHomeStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Hosts",
		"Port forwarding",
		"Command snippets",
		"Key management",
		"Host fingerprints",
		"Session history",
		"Search hosts",
		"Enter a host, IP address, or group",
		"Recent connections",
		"Groups",
		"Local terminal",
		"Refresh",
		"Clear",
		"Ungrouped",
		"SSH workspace",
		"No SSH hosts match this search.",
		"hosts",
		"Enabled",
		"Disabled",
		"Host is disabled.",
		"This workspace module is not available yet.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH workspace home string %q was not translated", source)
		}
	}
}

func TestSSHWorkspaceTabActionStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Duplicate",
		"Reconnect",
		"Rename",
		"Pin",
		"Unpin",
		"Close others",
		"Close all",
		"Tab name",
		"Save name",
		"Rename terminal tab",
		"Tab name is required",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH tab action string %q was not translated", source)
		}
	}
}

func TestSSHTerminalClipboardStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Copy",
		"Paste",
		"Paste multiple lines?",
		"Pasting multiple lines may execute several commands.",
		"Paste anyway",
		"No terminal text is selected.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH terminal clipboard string %q was not translated", source)
		}
	}
}

func TestSSHCommandSnippetWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Execute",
		"Search command snippets",
		"No command snippets yet.",
		"Run command snippet",
		"Variable value",
		"Secret values are loaded only when executing.",
		"Snippet is disabled.",
		"No terminal tab is connected.",
	}
	for _, source := range sources {
		translated := Text(TraditionalChinese, source)
		if translated == source {
			t.Fatalf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestSSHCommandSnippetManagementStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"New snippet",
		"Edit snippet",
		"Snippet name",
		"Command",
		"Variables (comma-separated)",
		"Secret values (NAME=value)",
		"Saved secret names",
		"Clear saved secrets",
		"Save snippet",
		"Delete snippet?",
		"This snippet will be permanently deleted.",
		"Saving SSH snippet...",
		"Deleting SSH snippet...",
		"Snippet name is required.",
		"Command is required.",
		"Secret entry must use NAME=value.",
		"Duplicate secret name.",
	} {
		translated := Text(TraditionalChinese, source)
		if translated == source || translated == "" {
			t.Fatalf("source %q has no Traditional Chinese translation", source)
		}
	}
}

func TestSSHTunnelWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"No tunnels configured.",
		"New tunnel",
		"Refresh tunnels",
		"Start",
		"Stop",
		"Local",
		"Remote",
		"Dynamic SOCKS",
		"Stopped",
		"Starting",
		"Running",
		"Failed",
		"Listen",
		"Target",
		"Traffic",
		"Up",
		"Down",
		"Tunnel name",
		"Tunnel host",
		"Listen host",
		"Listen port",
		"Target host",
		"Target port",
		"Save tunnel",
		"Delete tunnel",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH tunnel string %q was not translated", source)
		}
	}
}

func TestSSHTunnelFormValidationStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Tunnel name is required.",
		"Tunnel host is required.",
		"Tunnel type is invalid.",
		"Listen host is required.",
		"Listen port must be between 1 and 65535.",
		"Target host is required.",
		"Target port must be between 1 and 65535.",
		"Target port must be between 0 and 65535.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestSSHTunnelFormActionStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Edit tunnel",
		"Tunnel type",
		"Auto-start",
		"Delete tunnel?",
		"This tunnel will be permanently deleted.",
		"Deleting SSH tunnel...",
		"Saving SSH tunnel...",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestSFTPWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Terminal",
		"SFTP",
		"Remote path",
		"Parent folder",
		"Refresh files",
		"New folder",
		"Rename item",
		"Delete selected",
		"File information",
		"Create symbolic link",
		"No files in this folder.",
		"Loading remote files...",
		"Opening SFTP...",
		"SFTP is not available for local terminals.",
		"Delete remote items?",
		"This will permanently delete the selected remote items.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SFTP workspace string %q was not translated", source)
		}
	}
}

func TestSFTPOperationDialogStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Folder name",
		"New name",
		"Target path",
		"Link name",
		"Create",
		"Folder name is required.",
		"New name is required.",
		"Target path is required.",
		"Link name is required.",
		"Select at least one item.",
		"Select exactly one item.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SFTP operation string %q was not translated", source)
		}
	}
}

func TestSFTPInfoStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Type",
		"file",
		"directory",
		"symbolic link",
		"Size",
		"Mode",
		"Modified",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SFTP info source %q was not translated", source)
		}
	}
}

func TestSSHKeyIdentityWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Search key identities",
		"Refresh keys",
		"No key identities yet.",
		"No key identities match this search.",
		"New key",
		"Edit key",
		"Delete key?",
		"Key name",
		"Public key",
		"Fingerprint",
		"Private key material",
		"Key passphrase",
		"Saved key has a passphrase.",
		"Clear saved key material",
		"Save key",
		"This key identity will be permanently deleted.",
		"Saving SSH key identity...",
		"Deleting SSH key identity...",
		"Key name is required.",
		"Private key is required.",
		"Key identity is disabled.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH key identity string %q was not translated", source)
		}
	}
}

func TestSSHHostFingerprintWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Search host fingerprints",
		"Refresh fingerprints",
		"No host fingerprints yet.",
		"No host fingerprints match this search.",
		"Current",
		"Retired",
		"TOFU",
		"Manual",
		"Algorithm",
		"Observed",
		"Retired at",
		"Clear trust",
		"Trust manual fingerprint",
		"Copy fingerprint",
		"Manual host fingerprint",
		"Fingerprint must include an algorithm and digest.",
		"Clear trusted fingerprint?",
		"This host will require TOFU confirmation on the next connection.",
		"Saving host fingerprint...",
		"Clearing host fingerprint...",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH host fingerprint source %q was not translated", source)
		}
	}
}

func TestSSHSessionHistoryWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Search session history",
		"Refresh history",
		"No session history yet.",
		"No session history matches this search.",
		"Connecting",
		"Connected",
		"Failed",
		"Closed",
		"Latency",
		"Started",
		"Ended",
		"Error details",
		"Loading SSH session history...",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("SSH session history source %q was not translated", source)
		}
	}
}

func TestSFTPTransferStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Transfers",
		"Upload",
		"Download",
		"Upload files",
		"Download selected",
		"Hide transfers",
		"Show transfers",
		"Pause",
		"Resume",
		"Retry transfer",
		"queued",
		"running",
		"paused",
		"completed",
		"failed",
		"No transfers.",
	}
	for _, source := range sources {
		if got := Text(TraditionalChinese, source); got == source {
			t.Errorf("%q did not translate", source)
		}
	}
}

func TestSFTPUploadConflictStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Resolve upload conflict",
		"A remote file with this name already exists.",
		"Overwrite",
		"Skip",
		"Keep both",
	}
	for _, source := range sources {
		translated := Text(TraditionalChinese, source)
		if translated == source {
			t.Errorf("Traditional Chinese translation missing for %q", source)
		}
	}
}

func TestSSHWorkspaceImportExportStringsTranslateToTraditionalChinese(t *testing.T) {
	sources := []string{
		"Export workspace",
		"Import workspace",
		"Include secrets",
		"Export password",
		"Export workspace package",
		"Import workspace package",
		"Preview import",
		"Apply import",
		"Import password",
		"No import conflicts.",
		"Resolve import conflicts",
		"Import package is required.",
		"Import password is required for encrypted packages.",
		"Import conflict decisions are incomplete.",
		"Exporting SSH workspace...",
		"Loading SSH workspace import preview...",
		"Applying SSH workspace import...",
	}
	for _, source := range sources {
		if translated := Text(TraditionalChinese, source); translated == source {
			t.Errorf("Traditional Chinese translation missing for %q", source)
		}
	}
}

func TestTerminalAppearanceWorkspaceStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Terminal appearance", "Account default", "Override for this host", "Use account default",
		"Builtin monospace", "System monospace", "Font size", "Foreground", "Background",
		"Save appearance", "Terminal appearance is invalid.", "Saving terminal appearance...",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Fatalf("missing Traditional Chinese translation for %q", source)
		}
	}
}

func TestSSHWorkspaceExportValidationStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Export password is required when secrets are included.",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Fatalf("source %q has no Traditional Chinese translation", source)
		}
	}
}

func TestSFTPDropStringsTranslateToTraditionalChinese(t *testing.T) {
	for _, source := range []string{
		"Dropped files are invalid.",
		"Dropped file data is too large.",
	} {
		if got := Text(TraditionalChinese, source); got == source {
			t.Fatalf("source %q has no Traditional Chinese translation", source)
		}
	}
}
