package gui

import (
	"errors"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/x/explorer"
)

type nativeSFTPFileDialog struct {
	explorer *explorer.Explorer
}

type sftpFileDialogEventListener interface {
	ListenEvents(event.Event)
}

func newNativeSFTPFileDialog(window *app.Window) sftpFileDialog {
	if window == nil {
		return nil
	}
	return &nativeSFTPFileDialog{explorer: explorer.NewExplorer(window)}
}

func (dialog *nativeSFTPFileDialog) ListenEvents(current event.Event) {
	if dialog == nil || dialog.explorer == nil {
		return
	}
	dialog.explorer.ListenEvents(current)
}

func (dialog *nativeSFTPFileDialog) ChooseUploadPaths() ([]string, error) {
	if dialog == nil || dialog.explorer == nil {
		return nil, errors.New("native file dialog is unavailable")
	}
	readers, err := dialog.explorer.ChooseFiles()
	if err != nil {
		return nil, normalizeSFTPExplorerError(err)
	}
	return sftpUploadPathsFromReaders(readers)
}

func (dialog *nativeSFTPFileDialog) ChooseDownloadPath(name string) (string, error) {
	if dialog == nil || dialog.explorer == nil {
		return "", errors.New("native file dialog is unavailable")
	}
	writer, err := dialog.explorer.CreateFile(name)
	if err != nil {
		return "", normalizeSFTPExplorerError(err)
	}
	return sftpDownloadPathFromWriter(writer)
}

func (dialog *nativeSFTPFileDialog) ChooseImportPath() (string, error) {
	if dialog == nil || dialog.explorer == nil {
		return "", errors.New("native file dialog is unavailable")
	}
	reader, err := dialog.explorer.ChooseFile(".s12ryt")
	if err != nil {
		return "", normalizeWorkspaceExplorerError(err)
	}
	return workspaceImportPathFromReader(reader)
}

func (dialog *nativeSFTPFileDialog) ChooseExportPath() (string, error) {
	if dialog == nil || dialog.explorer == nil {
		return "", errors.New("native file dialog is unavailable")
	}
	writer, err := dialog.explorer.CreateFile("workspace.s12ryt")
	if err != nil {
		return "", normalizeWorkspaceExplorerError(err)
	}
	return workspaceExportPathFromWriter(writer)
}

func normalizeSFTPExplorerError(err error) error {
	if errors.Is(err, explorer.ErrUserDecline) {
		return errSFTPFileDialogCancelled
	}
	return err
}

func normalizeWorkspaceExplorerError(err error) error {
	if errors.Is(err, explorer.ErrUserDecline) {
		return errWorkspaceFileDialogCancelled
	}
	return err
}

var _ sftpFileDialog = (*nativeSFTPFileDialog)(nil)
var _ workspaceFileDialog = (*nativeSFTPFileDialog)(nil)
var _ sftpFileDialogEventListener = (*nativeSFTPFileDialog)(nil)
