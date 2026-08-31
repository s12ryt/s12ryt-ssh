package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"s12ryt-ssh/internal/remote"
)

func TestSSHWorkspaceImportFileReadsOpaquePackageAndRejectsOversizedInput(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "workspace.s12ryt")
	if err := os.WriteFile(path, []byte("opaque-package"), 0o600); err != nil {
		t.Fatalf("write package: %v", err)
	}
	if got, err := readSSHWorkspaceImportFile(path); err != nil || got != "opaque-package" {
		t.Fatalf("read package = %q, error %v", got, err)
	}

	oversized := filepath.Join(directory, "oversized.s12ryt")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatalf("create oversized package: %v", err)
	}
	if err := file.Truncate(16*1024*1024 + 1); err != nil {
		file.Close()
		t.Fatalf("truncate oversized package: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close oversized package: %v", err)
	}
	if _, err := readSSHWorkspaceImportFile(oversized); err == nil {
		t.Fatal("oversized package was accepted")
	}
}

func TestSSHWorkspaceExportFileWritesPrivateFileAndReplacesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.s12ryt")
	if err := writeSSHWorkspaceExportFile(path, "new-opaque-package"); err != nil {
		t.Fatalf("write package: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new-opaque-package" {
		t.Fatalf("export content = %q, error %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("export permissions = %o, want 600", info.Mode().Perm())
	}
	if err := writeSSHWorkspaceExportFile(path, "replaced-package"); err != nil {
		t.Fatalf("replace package: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "replaced-package" {
		t.Fatalf("replaced export content = %q, error %v", got, err)
	}
}

func TestSSHWorkspaceImportStateRequiresPackageAndEncryptedPassword(t *testing.T) {
	state := newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{})

	if got := validateSSHWorkspaceImportState(state); got != "Import package is required." {
		t.Fatalf("empty package error = %q", got)
	}

	state.Package = "opaque-package"
	if got := validateSSHWorkspaceImportState(state); got != "" {
		t.Fatalf("metadata-only package error = %q, want no error", got)
	}

	state.IncludesSecrets = true
	if got := validateSSHWorkspaceImportState(state); got != "Import password is required for encrypted packages." {
		t.Fatalf("encrypted package error = %q", got)
	}

	state.Password = "correct horse battery staple"
	if got := validateSSHWorkspaceImportState(state); got != "" {
		t.Fatalf("encrypted package with password error = %q", got)
	}
}

func TestSSHWorkspaceImportStateRequiresEveryRealConflictDecision(t *testing.T) {
	state := newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{
		IncludesSecrets: true,
		Conflicts: []remote.SSHWorkspaceImportConflict{
			{Kind: remote.SSHWorkspaceImportHost, Name: "web", Conflict: true},
			{Kind: remote.SSHWorkspaceImportTunnel, Name: "web-tunnel", Conflict: false},
			{Kind: remote.SSHWorkspaceImportKey, Name: "deploy", Conflict: true},
		},
	})
	state.Package = "opaque-package"
	state.Password = "correct horse battery staple"

	if got := validateSSHWorkspaceImportState(state); got != "Import conflict decisions are incomplete." {
		t.Fatalf("incomplete decisions error = %q", got)
	}
	if !state.setResolution(remote.SSHWorkspaceImportHost, "web", remote.SSHWorkspaceImportCopy) {
		t.Fatal("valid host resolution was rejected")
	}
	if got := validateSSHWorkspaceImportState(state); got != "Import conflict decisions are incomplete." {
		t.Fatalf("partially resolved error = %q", got)
	}
	if !state.setResolution(remote.SSHWorkspaceImportKey, "deploy", remote.SSHWorkspaceImportSkip) {
		t.Fatal("valid key resolution was rejected")
	}
	if got := validateSSHWorkspaceImportState(state); got != "" {
		t.Fatalf("complete decisions error = %q", got)
	}
}

func TestSSHWorkspaceImportStateRejectsInvalidOrUnusedResolutions(t *testing.T) {
	state := newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{
		Conflicts: []remote.SSHWorkspaceImportConflict{
			{Kind: remote.SSHWorkspaceImportHost, Name: "web", Conflict: true},
		},
	})

	if state.setResolution(remote.SSHWorkspaceImportHost, "missing", remote.SSHWorkspaceImportOverwrite) {
		t.Fatal("resolution for a missing conflict was accepted")
	}
	if state.setResolution(remote.SSHWorkspaceImportHost, "web", remote.SSHWorkspaceImportDecision("invalid")) {
		t.Fatal("invalid resolution action was accepted")
	}
	if !state.setResolution(remote.SSHWorkspaceImportHost, "web", remote.SSHWorkspaceImportOverwrite) {
		t.Fatal("valid resolution was rejected")
	}
}

func TestSSHWorkspaceImportStateEmitsResolutionsInPreviewOrder(t *testing.T) {
	state := newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{
		Conflicts: []remote.SSHWorkspaceImportConflict{
			{Kind: remote.SSHWorkspaceImportKey, Name: "deploy", Conflict: true},
			{Kind: remote.SSHWorkspaceImportHost, Name: "web", Conflict: true},
		},
	})
	state.setResolution(remote.SSHWorkspaceImportHost, "web", remote.SSHWorkspaceImportCopy)
	state.setResolution(remote.SSHWorkspaceImportKey, "deploy", remote.SSHWorkspaceImportSkip)

	got := state.resolutions()
	if len(got) != 2 {
		t.Fatalf("resolution count = %d, want 2", len(got))
	}
	if got[0].Kind != remote.SSHWorkspaceImportKey || got[0].Name != "deploy" || got[0].Action != remote.SSHWorkspaceImportSkip {
		t.Fatalf("first resolution = %+v", got[0])
	}
	if got[1].Kind != remote.SSHWorkspaceImportHost || got[1].Name != "web" || got[1].Action != remote.SSHWorkspaceImportCopy {
		t.Fatalf("second resolution = %+v", got[1])
	}
}

func TestSSHWorkspaceExportStateRequiresPasswordOnlyWhenSecretsAreIncluded(t *testing.T) {
	state := sshWorkspaceExportState{}
	if got := validateSSHWorkspaceExportState(state); got != "" {
		t.Fatalf("metadata-only export error = %q", got)
	}

	state.IncludeSecrets = true
	if got := validateSSHWorkspaceExportState(state); got != "Export password is required when secrets are included." {
		t.Fatalf("encrypted export error = %q", got)
	}

	state.Password = "correct horse battery staple"
	if got := validateSSHWorkspaceExportState(state); got != "" {
		t.Fatalf("encrypted export with password error = %q", got)
	}
}
