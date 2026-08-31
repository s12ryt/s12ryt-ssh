package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/x/explorer"

	"s12ryt-ssh/internal/remote"
)

type workspaceImportRemoteSession struct {
	fakeRemoteSession
	exportPackage string
	preview       remote.SSHWorkspaceImportPreview
	applyResult   remote.SSHWorkspaceImportResult
	exportRequest remote.SSHWorkspaceExportRequest
	previewInput  string
	previewPass   string
	applyRequest  remote.SSHWorkspaceImportRequest
}

func (session *workspaceImportRemoteSession) ExportSSHWorkspace(_ context.Context, request remote.SSHWorkspaceExportRequest) (string, error) {
	session.exportRequest = request
	return session.exportPackage, nil
}

func (session *workspaceImportRemoteSession) PreviewSSHWorkspaceImport(_ context.Context, encoded, password string) (remote.SSHWorkspaceImportPreview, error) {
	session.previewInput = encoded
	session.previewPass = password
	return session.preview, nil
}

func (session *workspaceImportRemoteSession) ApplySSHWorkspaceImport(_ context.Context, request remote.SSHWorkspaceImportRequest) (remote.SSHWorkspaceImportResult, error) {
	session.applyRequest = request
	return session.applyResult, nil
}

type fakeWorkspaceFileDialog struct {
	importPath    string
	importErr     error
	exportPath    string
	exportErr     error
	importStarted chan struct{}
	release       chan struct{}
	importCalls   int
	exportCalls   int
}

func (dialog *fakeWorkspaceFileDialog) ChooseImportPath() (string, error) {
	dialog.importCalls++
	if dialog.importStarted != nil {
		close(dialog.importStarted)
	}
	if dialog.release != nil {
		<-dialog.release
	}
	return dialog.importPath, dialog.importErr
}

func (dialog *fakeWorkspaceFileDialog) ChooseExportPath() (string, error) {
	dialog.exportCalls++
	return dialog.exportPath, dialog.exportErr
}

func waitForWorkspaceFileDialogEvent(t *testing.T, ui *Window) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("workspace file dialog did not queue a UI event")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRequestSSHWorkspaceImportPreventsConcurrentDialogsAndStoresSelectedPath(t *testing.T) {
	dialog := &fakeWorkspaceFileDialog{
		importPath:    "workspace.s12ryt",
		importStarted: make(chan struct{}),
		release:       make(chan struct{}),
	}
	ui := NewWindow(nil)
	ui.workspaceFiles = dialog

	if !ui.requestSSHWorkspaceImport() {
		t.Fatal("first import dialog request was rejected")
	}
	select {
	case <-dialog.importStarted:
	case <-time.After(time.Second):
		t.Fatal("import dialog did not start")
	}
	if ui.requestSSHWorkspaceImport() {
		t.Fatal("concurrent import dialog request was accepted")
	}
	if ui.requestSSHWorkspaceExport() {
		t.Fatal("export dialog must share the import dialog busy guard")
	}

	close(dialog.release)
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()

	if dialog.importCalls != 1 || ui.workspaceFileDialogBusy {
		t.Fatalf("dialog calls=%d busy=%v", dialog.importCalls, ui.workspaceFileDialogBusy)
	}
	if ui.workspaceImportPath != "workspace.s12ryt" {
		t.Fatalf("workspace import path = %q", ui.workspaceImportPath)
	}
	if !ui.workspaceImportOpen {
		t.Fatal("selected workspace import did not open the import form")
	}
}

func TestRequestSSHWorkspaceExportStoresSelectedPath(t *testing.T) {
	ui := NewWindow(nil)
	ui.workspaceFiles = &fakeWorkspaceFileDialog{exportPath: "workspace.s12ryt"}

	if !ui.requestSSHWorkspaceExport() {
		t.Fatal("export dialog request was rejected")
	}
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()

	if ui.workspaceFileDialogBusy {
		t.Fatal("export dialog busy flag was not cleared")
	}
	if ui.workspaceExportPath != "workspace.s12ryt" {
		t.Fatalf("workspace export path = %q", ui.workspaceExportPath)
	}
}

func TestSSHWorkspaceFileDialogCancellationIsSilent(t *testing.T) {
	ui := NewWindow(nil)
	ui.workspaceFiles = &fakeWorkspaceFileDialog{importErr: errWorkspaceFileDialogCancelled}

	if !ui.requestSSHWorkspaceImport() {
		t.Fatal("cancelled import dialog request was rejected")
	}
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()

	if ui.workspaceFileDialogBusy {
		t.Fatal("cancelled dialog busy flag was not cleared")
	}
	if ui.workspaceImportPath != "" {
		t.Fatalf("cancelled import path = %q", ui.workspaceImportPath)
	}
	if ui.model.Error != "" {
		t.Fatalf("cancelled dialog set error: %q", ui.model.Error)
	}
}

func TestWorkspaceFileDialogPathsCloseHandlesAndRejectUnnamedFiles(t *testing.T) {
	reader := &namedDialogReader{name: `C:\workspace.s12ryt`}
	if got, err := workspaceImportPathFromReader(reader); err != nil || got != reader.name {
		t.Fatalf("import path = %q, error %v", got, err)
	}
	if !reader.closed {
		t.Fatal("import reader was not closed")
	}

	unnamedReader := &namedDialogReader{}
	if _, err := workspaceImportPathFromReader(unnamedReader); err == nil || !unnamedReader.closed {
		t.Fatalf("unnamed import reader = error %v, closed %v", err, unnamedReader.closed)
	}

	writer := &namedDialogWriter{name: `C:\workspace.s12ryt`}
	if got, err := workspaceExportPathFromWriter(writer); err != nil || got != writer.name {
		t.Fatalf("export path = %q, error %v", got, err)
	}
	if !writer.closed {
		t.Fatal("export writer was not closed")
	}

	unnamedWriter := &namedDialogWriter{}
	if _, err := workspaceExportPathFromWriter(unnamedWriter); err == nil || !unnamedWriter.closed {
		t.Fatalf("unnamed export writer = error %v, closed %v", err, unnamedWriter.closed)
	}
}

func TestNormalizeWorkspaceExplorerCancellation(t *testing.T) {
	if !errors.Is(normalizeWorkspaceExplorerError(explorer.ErrUserDecline), errWorkspaceFileDialogCancelled) {
		t.Fatal("user cancellation was not mapped to the workspace cancellation sentinel")
	}
	original := errors.New("dialog failed")
	if !errors.Is(normalizeWorkspaceExplorerError(original), original) {
		t.Fatal("non-cancellation error was not preserved")
	}
}

func TestExportSSHWorkspaceWritesRemotePackageToSelectedPath(t *testing.T) {
	session := &workspaceImportRemoteSession{exportPackage: "opaque-export"}
	password := "correct horse battery staple"
	ui := NewWindow(nil)
	ui.model.SetRemoteSession(session, true)
	ui.workspaceExportPath = filepath.Join(t.TempDir(), "workspace.s12ryt")
	ui.workspaceExport = sshWorkspaceExportState{
		IncludeSecrets: true,
		Password:       password,
	}

	if !ui.submitSSHWorkspaceExport() {
		t.Fatal("workspace export did not start")
	}
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()

	content, err := os.ReadFile(ui.workspaceExportPath)
	if err != nil {
		t.Fatalf("read exported package: %v", err)
	}
	if string(content) != session.exportPackage {
		t.Fatalf("exported package = %q, want %q", content, session.exportPackage)
	}
	if !session.exportRequest.IncludeSecrets || session.exportRequest.Password != password {
		t.Fatalf("export request = %+v", session.exportRequest)
	}
}

func TestPreviewAndApplySSHWorkspaceImportUsesOpaquePackageAndResolutions(t *testing.T) {
	packagePath := filepath.Join(t.TempDir(), "workspace.s12ryt")
	if err := os.WriteFile(packagePath, []byte("opaque-import"), 0o600); err != nil {
		t.Fatalf("write import package: %v", err)
	}
	session := &workspaceImportRemoteSession{
		preview: remote.SSHWorkspaceImportPreview{
			IncludesSecrets: true,
			Counts:          remote.SSHWorkspaceResourceCounts{Hosts: 1},
			Conflicts: []remote.SSHWorkspaceImportConflict{
				{Kind: remote.SSHWorkspaceImportHost, Name: "web", Conflict: true},
			},
		},
		applyResult: remote.SSHWorkspaceImportResult{
			IncludesSecrets: true,
			Counts:          remote.SSHWorkspaceImportApplyCounts{Copied: 1},
		},
	}
	ui := NewWindow(nil)
	ui.model.SetRemoteSession(session, true)
	ui.workspaceImportPath = packagePath
	password := "correct horse battery staple"
	ui.workspaceImportPassword = password

	if !ui.previewSSHWorkspaceImport() {
		t.Fatal("workspace import preview did not start")
	}
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()
	if session.previewInput != "opaque-import" || session.previewPass != password {
		t.Fatalf("preview request package=%q password=%q", session.previewInput, session.previewPass)
	}
	if ui.workspaceImport == nil || !ui.workspaceImport.IncludesSecrets || len(ui.workspaceImport.Conflicts) != 1 {
		t.Fatalf("import state = %+v", ui.workspaceImport)
	}
	if !ui.workspaceImport.setResolution(remote.SSHWorkspaceImportHost, "web", remote.SSHWorkspaceImportCopy) {
		t.Fatal("host conflict resolution was rejected")
	}

	if !ui.applySSHWorkspaceImport() {
		t.Fatal("workspace import apply did not start")
	}
	waitForWorkspaceFileDialogEvent(t, ui)
	ui.pump()
	if session.applyRequest.Package != "opaque-import" || session.applyRequest.Password != password {
		t.Fatalf("apply request = %+v", session.applyRequest)
	}
	if len(session.applyRequest.Resolutions) != 1 || session.applyRequest.Resolutions[0].Action != remote.SSHWorkspaceImportCopy {
		t.Fatalf("apply resolutions = %+v", session.applyRequest.Resolutions)
	}
	if ui.workspaceImport != nil {
		t.Fatal("import state was not cleared after apply")
	}
}

func TestSSHWorkspaceImportExportFormsKeepSensitiveValuesInFormState(t *testing.T) {
	ui := NewWindow(nil)
	if !ui.openSSHWorkspaceExportForm() {
		t.Fatal("workspace export form did not open")
	}
	ui.workspaceExportIncludeSecrets.Value = true
	ui.workspaceExportPassword.SetText("correct horse battery staple")
	if got := ui.currentSSHWorkspaceExportForm(); !got.IncludeSecrets || got.Password != "correct horse battery staple" {
		t.Fatalf("export form = %+v", got)
	}
	ui.closeSSHWorkspaceExportForm()
	if ui.workspaceExportOpen || ui.workspaceExport.Password != "" {
		t.Fatal("export form retained sensitive state after close")
	}

	ui.workspaceImportPath = "workspace.s12ryt"
	if !ui.openSSHWorkspaceImportForm() {
		t.Fatal("workspace import form did not open")
	}
	ui.workspaceImportPasswordEditor.SetText("import password")
	ui.syncSSHWorkspaceImportPassword()
	if ui.workspaceImportPassword != "import password" {
		t.Fatalf("import password = %q", ui.workspaceImportPassword)
	}
	ui.closeSSHWorkspaceImportForm()
	if ui.workspaceImportOpen || ui.workspaceImport != nil || ui.workspaceImportPassword != "" {
		t.Fatal("import form retained sensitive state after close")
	}
	if len(ui.workspaceImportConflictKeys) != 0 || len(ui.workspaceImportConflictButtons) != 0 {
		t.Fatal("import form retained conflict controls after close")
	}
}

func TestSyncSSHWorkspaceImportConflictButtonsFollowsPreviewOrder(t *testing.T) {
	ui := NewWindow(nil)
	ui.workspaceImport = newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{
		Conflicts: []remote.SSHWorkspaceImportConflict{
			{Kind: remote.SSHWorkspaceImportHost, Name: "web", Conflict: true},
			{Kind: remote.SSHWorkspaceImportTunnel, Name: "proxy", Conflict: false},
			{Kind: remote.SSHWorkspaceImportSnippet, Name: "deploy", Conflict: true},
		},
	})

	ui.syncSSHWorkspaceImportConflictButtons()
	if len(ui.workspaceImportConflictKeys) != 2 || len(ui.workspaceImportConflictButtons) != 2 {
		t.Fatalf("conflict keys=%v buttons=%d", ui.workspaceImportConflictKeys, len(ui.workspaceImportConflictButtons))
	}
	if ui.workspaceImportConflictKeys[0] != sshWorkspaceImportDecisionKey(remote.SSHWorkspaceImportHost, "web") ||
		ui.workspaceImportConflictKeys[1] != sshWorkspaceImportDecisionKey(remote.SSHWorkspaceImportSnippet, "deploy") {
		t.Fatalf("conflict keys = %v", ui.workspaceImportConflictKeys)
	}

	ui.workspaceImport = newSSHWorkspaceImportState(remote.SSHWorkspaceImportPreview{
		Conflicts: []remote.SSHWorkspaceImportConflict{
			{Kind: remote.SSHWorkspaceImportKey, Name: "production", Conflict: true},
		},
	})
	ui.syncSSHWorkspaceImportConflictButtons()
	if len(ui.workspaceImportConflictKeys) != 1 || len(ui.workspaceImportConflictButtons) != 1 {
		t.Fatalf("updated conflict keys=%v buttons=%d", ui.workspaceImportConflictKeys, len(ui.workspaceImportConflictButtons))
	}
}
