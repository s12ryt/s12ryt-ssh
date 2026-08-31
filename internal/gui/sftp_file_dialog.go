package gui

import (
	"errors"
	"io"
	"strings"
)

var errSFTPFileDialogCancelled = errors.New("SFTP file dialog was cancelled")

type sftpFileDialog interface {
	ChooseUploadPaths() ([]string, error)
	ChooseDownloadPath(name string) (string, error)
}

type namedDialogHandle interface {
	Name() string
}

func sftpUploadPathsFromReaders(readers []io.ReadCloser) ([]string, error) {
	paths := make([]string, 0, len(readers))
	var resultErr error
	for _, reader := range readers {
		if reader == nil {
			resultErr = errors.Join(resultErr, errors.New("selected upload file is unavailable"))
			continue
		}
		name := ""
		if named, ok := reader.(namedDialogHandle); ok {
			name = strings.TrimSpace(named.Name())
		}
		if name == "" {
			resultErr = errors.Join(resultErr, errors.New("selected upload file does not expose a local path"))
		} else {
			paths = append(paths, name)
		}
		resultErr = errors.Join(resultErr, reader.Close())
	}
	if resultErr != nil {
		return nil, resultErr
	}
	if len(paths) == 0 {
		return nil, errSFTPFileDialogCancelled
	}
	return paths, nil
}

func sftpDownloadPathFromWriter(writer io.WriteCloser) (string, error) {
	if writer == nil {
		return "", errors.New("download destination is unavailable")
	}
	name := ""
	if named, ok := writer.(namedDialogHandle); ok {
		name = strings.TrimSpace(named.Name())
	}
	closeErr := writer.Close()
	if name == "" {
		return "", errors.Join(errors.New("download destination does not expose a local path"), closeErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	return name, nil
}

func (ui *Window) requestSFTPUploadFiles(tabID string) bool {
	if ui == nil || ui.sftpFiles == nil || ui.sftpFileDialogBusy || ui.transferSFTPTab(tabID) == nil {
		return false
	}
	dialog := ui.sftpFiles
	ui.sftpFileDialogBusy = true
	go func() {
		paths, err := dialog.ChooseUploadPaths()
		ui.queueSSHTabApply(func() {
			ui.sftpFileDialogBusy = false
			if errors.Is(err, errSFTPFileDialogCancelled) {
				return
			}
			if err != nil {
				ui.model.Error = err.Error()
				return
			}
			ui.prepareSFTPUploads(tabID, paths)
		})
	}()
	return true
}

func (ui *Window) requestSFTPDownloadFiles(tabID string) bool {
	tab := ui.transferSFTPTab(tabID)
	if ui == nil || ui.sftpFiles == nil || ui.sftpFileDialogBusy || tab == nil {
		return false
	}
	selected := make([]sftpEntry, 0, len(tab.sftpBrowser.Entries))
	for _, entry := range tab.sftpBrowser.Entries {
		if !entry.Directory && tab.sftpBrowser.selections[entry.Path] {
			selected = append(selected, entry)
		}
	}
	if len(selected) == 0 {
		return false
	}
	dialog := ui.sftpFiles
	ui.sftpFileDialogBusy = true
	go func() {
		targets := make([]sftpDownloadTarget, 0, len(selected))
		var dialogErr error
		for _, entry := range selected {
			localPath, err := dialog.ChooseDownloadPath(entry.Name)
			if errors.Is(err, errSFTPFileDialogCancelled) {
				break
			}
			if err != nil {
				dialogErr = err
				break
			}
			if localPath != "" {
				targets = append(targets, sftpDownloadTarget{RemotePath: entry.Path, LocalPath: localPath})
			}
		}
		ui.queueSSHTabApply(func() {
			ui.sftpFileDialogBusy = false
			if dialogErr != nil {
				ui.model.Error = dialogErr.Error()
				return
			}
			ui.enqueueSFTPDownloads(tabID, targets)
		})
	}()
	return true
}
