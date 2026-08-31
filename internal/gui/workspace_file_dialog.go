package gui

import (
	"errors"
	"io"
	"slices"
	"strings"

	"gioui.org/widget"
)

var errWorkspaceFileDialogCancelled = errors.New("workspace file dialog was cancelled")

type workspaceFileDialog interface {
	ChooseImportPath() (string, error)
	ChooseExportPath() (string, error)
}

func workspaceImportPathFromReader(reader io.ReadCloser) (string, error) {
	if reader == nil {
		return "", errors.New("workspace import file is unavailable")
	}
	name := ""
	if named, ok := reader.(namedDialogHandle); ok {
		name = strings.TrimSpace(named.Name())
	}
	closeErr := reader.Close()
	if name == "" {
		return "", errors.Join(errors.New("workspace import file does not expose a path"), closeErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	return name, nil
}

func workspaceExportPathFromWriter(writer io.WriteCloser) (string, error) {
	if writer == nil {
		return "", errors.New("workspace export file is unavailable")
	}
	name := ""
	if named, ok := writer.(namedDialogHandle); ok {
		name = strings.TrimSpace(named.Name())
	}
	closeErr := writer.Close()
	if name == "" {
		return "", errors.Join(errors.New("workspace export file does not expose a path"), closeErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	return name, nil
}

func (ui *Window) requestSSHWorkspaceImport() bool {
	if ui == nil || ui.workspaceFiles == nil || ui.workspaceFileDialogBusy {
		return false
	}
	dialog := ui.workspaceFiles
	ui.workspaceFileDialogBusy = true
	go func() {
		path, err := dialog.ChooseImportPath()
		ui.queueSSHTabApply(func() {
			ui.workspaceFileDialogBusy = false
			if errors.Is(err, errWorkspaceFileDialogCancelled) {
				return
			}
			if err != nil {
				ui.model.Error = err.Error()
				return
			}
			if strings.TrimSpace(path) == "" {
				return
			}
			ui.workspaceImportPath = path
			ui.openSSHWorkspaceImportForm()
		}, func() {
			ui.workspaceFileDialogBusy = false
		})
	}()
	return true
}

func (ui *Window) syncSSHWorkspaceImportConflictButtons() {
	if ui == nil || ui.workspaceImport == nil {
		if ui != nil {
			ui.workspaceImportConflictKeys = nil
			ui.workspaceImportConflictButtons = nil
		}
		return
	}
	keys := make([]string, 0, len(ui.workspaceImport.Conflicts))
	for _, conflict := range ui.workspaceImport.Conflicts {
		if conflict.Conflict {
			keys = append(keys, sshWorkspaceImportDecisionKey(conflict.Kind, conflict.Name))
		}
	}
	if slices.Equal(keys, ui.workspaceImportConflictKeys) && len(ui.workspaceImportConflictButtons) == len(keys) {
		return
	}
	ui.workspaceImportConflictKeys = keys
	ui.workspaceImportConflictButtons = make([][3]widget.Clickable, len(keys))
}

func (ui *Window) requestSSHWorkspaceExport() bool {
	if ui == nil || ui.workspaceFiles == nil || ui.workspaceFileDialogBusy {
		return false
	}
	dialog := ui.workspaceFiles
	ui.workspaceFileDialogBusy = true
	go func() {
		path, err := dialog.ChooseExportPath()
		ui.queueSSHTabApply(func() {
			ui.workspaceFileDialogBusy = false
			if errors.Is(err, errWorkspaceFileDialogCancelled) {
				return
			}
			if err != nil {
				ui.model.Error = err.Error()
				return
			}
			if strings.TrimSpace(path) == "" {
				return
			}
			ui.workspaceExportPath = path
			if ui.workspaceExportOpen {
				ui.submitSSHWorkspaceExport()
			}
		}, func() {
			ui.workspaceFileDialogBusy = false
		})
	}()
	return true
}
