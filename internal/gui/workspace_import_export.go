package gui

import (
	"context"
	"errors"
	"strings"

	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
)

type remoteWorkspaceImportExportSession interface {
	ExportSSHWorkspace(context.Context, remote.SSHWorkspaceExportRequest) (string, error)
	PreviewSSHWorkspaceImport(context.Context, string, string) (remote.SSHWorkspaceImportPreview, error)
	ApplySSHWorkspaceImport(context.Context, remote.SSHWorkspaceImportRequest) (remote.SSHWorkspaceImportResult, error)
}

func (ui *Window) optionalWorkspaceImportExportSession() (remoteWorkspaceImportExportSession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote workspace session is unavailable")
	}
	session, ok := ui.model.RemoteSession.(remoteWorkspaceImportExportSession)
	if !ok {
		return nil, errors.New("SSH workspace import and export service is unavailable")
	}
	return session, nil
}

func (ui *Window) closeSSHWorkspaceImportExport() {
	if ui == nil {
		return
	}
	ui.closeSSHWorkspaceExportForm()
	ui.closeSSHWorkspaceImportForm()
}

func (ui *Window) openSSHWorkspaceExportForm() bool {
	if ui == nil || ui.busy || ui.workspaceImportOpen {
		return false
	}
	ui.workspaceExportOpen = true
	ui.workspaceExportPath = ""
	ui.workspaceExport = sshWorkspaceExportState{}
	ui.workspaceExportIncludeSecrets.Value = false
	ui.workspaceExportPassword.SetText("")
	ui.workspaceExportPassword.SingleLine = true
	ui.workspaceExportPassword.Submit = true
	return true
}

func (ui *Window) currentSSHWorkspaceExportForm() sshWorkspaceExportState {
	if ui == nil {
		return sshWorkspaceExportState{}
	}
	ui.workspaceExport = sshWorkspaceExportState{
		IncludeSecrets: ui.workspaceExportIncludeSecrets.Value,
		Password:       ui.workspaceExportPassword.Text(),
	}
	return ui.workspaceExport
}

func (ui *Window) closeSSHWorkspaceExportForm() {
	if ui == nil {
		return
	}
	ui.workspaceExportOpen = false
	ui.workspaceExportPath = ""
	ui.workspaceExport = sshWorkspaceExportState{}
	ui.workspaceExportIncludeSecrets.Value = false
	ui.workspaceExportPassword.SetText("")
	ui.workspaceExportClose = widget.Clickable{}
	ui.workspaceExportCancel = widget.Clickable{}
	ui.workspaceExportSubmit = widget.Clickable{}
	ui.workspaceExportScrim = widget.Clickable{}
	delete(ui.reveals, &ui.workspaceExportPassword)
}

func (ui *Window) openSSHWorkspaceImportForm() bool {
	if ui == nil || ui.busy || ui.workspaceExportOpen || strings.TrimSpace(ui.workspaceImportPath) == "" {
		return false
	}
	ui.workspaceImportOpen = true
	ui.workspaceImport = nil
	ui.workspaceImportPassword = ""
	ui.workspaceImportPasswordEditor.SetText("")
	ui.workspaceImportPasswordEditor.SingleLine = true
	ui.workspaceImportPasswordEditor.Submit = true
	return true
}

func (ui *Window) syncSSHWorkspaceImportPassword() {
	if ui == nil {
		return
	}
	ui.workspaceImportPassword = ui.workspaceImportPasswordEditor.Text()
}

func (ui *Window) closeSSHWorkspaceImportForm() {
	if ui == nil {
		return
	}
	ui.workspaceImportOpen = false
	ui.workspaceImportPath = ""
	ui.workspaceImport = nil
	ui.workspaceImportPassword = ""
	ui.workspaceImportPasswordEditor.SetText("")
	ui.workspaceImportClose = widget.Clickable{}
	ui.workspaceImportCancel = widget.Clickable{}
	ui.workspaceImportPreview = widget.Clickable{}
	ui.workspaceImportApply = widget.Clickable{}
	ui.workspaceImportScrim = widget.Clickable{}
	ui.workspaceImportConflictKeys = nil
	ui.workspaceImportConflictButtons = nil
	delete(ui.reveals, &ui.workspaceImportPasswordEditor)
}

func (ui *Window) submitSSHWorkspaceExport() bool {
	if ui == nil || ui.model == nil || ui.busy {
		return false
	}
	if strings.TrimSpace(ui.workspaceExportPath) == "" {
		ui.model.Error = ui.text("Export workspace package")
		return false
	}
	if source := validateSSHWorkspaceExportState(ui.workspaceExport); source != "" {
		ui.model.Error = ui.text(source)
		return false
	}
	session, err := ui.optionalWorkspaceImportExportSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	request := remote.SSHWorkspaceExportRequest{
		IncludeSecrets: ui.workspaceExport.IncludeSecrets,
		Password:       ui.workspaceExport.Password,
	}
	path := ui.workspaceExportPath
	ui.workspaceExport.Password = ""
	ui.async(ui.text("Exporting SSH workspace..."), func(ctx context.Context) (func(), error) {
		encoded, err := session.ExportSSHWorkspace(ctx, request)
		if err != nil {
			return nil, err
		}
		if err := writeSSHWorkspaceExportFile(path, encoded); err != nil {
			return nil, err
		}
		return func() {
			if ui.workspaceExportOpen {
				ui.closeSSHWorkspaceExportForm()
			}
		}, nil
	})
	return true
}

func (ui *Window) previewSSHWorkspaceImport() bool {
	if ui == nil || ui.model == nil || ui.busy {
		return false
	}
	if strings.TrimSpace(ui.workspaceImportPath) == "" {
		ui.model.Error = ui.text("Import package is required.")
		return false
	}
	session, err := ui.optionalWorkspaceImportExportSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	path := ui.workspaceImportPath
	password := ui.workspaceImportPassword
	ui.async(ui.text("Loading SSH workspace import preview..."), func(ctx context.Context) (func(), error) {
		encoded, err := readSSHWorkspaceImportFile(path)
		if err != nil {
			return nil, err
		}
		preview, err := session.PreviewSSHWorkspaceImport(ctx, encoded, password)
		if err != nil {
			return nil, err
		}
		return func() {
			state := newSSHWorkspaceImportState(preview)
			state.Package = encoded
			state.Password = password
			ui.workspaceImport = state
		}, nil
	})
	return true
}

func (ui *Window) applySSHWorkspaceImport() bool {
	if ui == nil || ui.model == nil || ui.busy || ui.workspaceImport == nil {
		return false
	}
	if source := validateSSHWorkspaceImportState(ui.workspaceImport); source != "" {
		ui.model.Error = ui.text(source)
		return false
	}
	session, err := ui.optionalWorkspaceImportExportSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	state := ui.workspaceImport
	request := remote.SSHWorkspaceImportRequest{
		Package:     state.Package,
		Password:    state.Password,
		Resolutions: state.resolutions(),
	}
	ui.async(ui.text("Applying SSH workspace import..."), func(ctx context.Context) (func(), error) {
		if _, err := session.ApplySSHWorkspaceImport(ctx, request); err != nil {
			return nil, err
		}
		return func() {
			ui.closeSSHWorkspaceImportForm()
		}, nil
	})
	return true
}
