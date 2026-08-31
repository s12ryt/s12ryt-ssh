package gui

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type fakeSFTPFileDialog struct {
	mu                sync.Mutex
	uploadPaths       []string
	uploadErr         error
	downloadPaths     []string
	downloadErr       error
	downloadNames     []string
	uploadStarted     chan struct{}
	releaseUpload     chan struct{}
	uploadStartedOnce sync.Once
}

type namedDialogReader struct {
	*bytes.Reader
	name   string
	closed bool
}

func (reader *namedDialogReader) Name() string { return reader.name }

func (reader *namedDialogReader) Close() error {
	reader.closed = true
	return nil
}

type namedDialogWriter struct {
	bytes.Buffer
	name   string
	closed bool
}

func (writer *namedDialogWriter) Name() string { return writer.name }

func (writer *namedDialogWriter) Close() error {
	writer.closed = true
	return nil
}

func (dialog *fakeSFTPFileDialog) ChooseUploadPaths() ([]string, error) {
	if dialog.uploadStarted != nil {
		dialog.uploadStartedOnce.Do(func() { close(dialog.uploadStarted) })
	}
	if dialog.releaseUpload != nil {
		<-dialog.releaseUpload
	}
	dialog.mu.Lock()
	defer dialog.mu.Unlock()
	return append([]string(nil), dialog.uploadPaths...), dialog.uploadErr
}

func (dialog *fakeSFTPFileDialog) ChooseDownloadPath(name string) (string, error) {
	dialog.mu.Lock()
	defer dialog.mu.Unlock()
	dialog.downloadNames = append(dialog.downloadNames, name)
	if dialog.downloadErr != nil {
		return "", dialog.downloadErr
	}
	if len(dialog.downloadPaths) == 0 {
		return "", errSFTPFileDialogCancelled
	}
	result := dialog.downloadPaths[0]
	dialog.downloadPaths = dialog.downloadPaths[1:]
	return result, nil
}

func TestRequestSFTPUploadFilesQueuesNativeSelectionsAndPreventsConcurrentDialogs(t *testing.T) {
	ui, tab := newSFTPFileDialogTestWindow(t)
	localDirectory := t.TempDir()
	first := filepath.Join(localDirectory, "first.txt")
	second := filepath.Join(localDirectory, "second.txt")
	if err := os.WriteFile(first, []byte("first"), 0o600); err != nil {
		t.Fatalf("write first upload: %v", err)
	}
	if err := os.WriteFile(second, []byte("second"), 0o600); err != nil {
		t.Fatalf("write second upload: %v", err)
	}
	dialog := &fakeSFTPFileDialog{
		uploadPaths:   []string{first, second},
		uploadStarted: make(chan struct{}),
		releaseUpload: make(chan struct{}),
	}
	ui.sftpFiles = dialog

	if !ui.requestSFTPUploadFiles(tab.ID) {
		t.Fatal("first upload dialog did not start")
	}
	<-dialog.uploadStarted
	if ui.requestSFTPUploadFiles(tab.ID) {
		t.Fatal("second upload dialog started while one was already open")
	}
	close(dialog.releaseUpload)
	waitForSFTPUIEvent(t, ui)

	items := ui.transfers.items()
	if len(items) != 2 || items[0].Source != first || items[1].Source != second {
		t.Fatalf("queued upload selections = %+v", items)
	}
	if ui.sftpFileDialogBusy {
		t.Fatal("upload dialog stayed busy after applying its result")
	}
}

func TestRequestSFTPDownloadFilesPromptsForEachSelectedFile(t *testing.T) {
	ui, tab := newSFTPFileDialogTestWindow(t)
	tab.sftpBrowser.applyEntries([]sftpEntry{
		{Name: "first.txt", Path: "/srv/first.txt", Size: 5},
		{Name: "folder", Path: "/srv/folder", Directory: true},
		{Name: "second.txt", Path: "/srv/second.txt", Size: 6},
	})
	tab.sftpBrowser.toggleSelection("/srv/first.txt")
	tab.sftpBrowser.toggleSelection("/srv/folder")
	tab.sftpBrowser.toggleSelection("/srv/second.txt")
	firstDestination := filepath.Join(t.TempDir(), "first.txt")
	secondDestination := filepath.Join(t.TempDir(), "second.txt")
	dialog := &fakeSFTPFileDialog{downloadPaths: []string{firstDestination, secondDestination}}
	ui.sftpFiles = dialog

	if !ui.requestSFTPDownloadFiles(tab.ID) {
		t.Fatal("download dialogs did not start")
	}
	waitForSFTPUIEvent(t, ui)

	if want := []string{"first.txt", "second.txt"}; !reflect.DeepEqual(dialog.downloadNames, want) {
		t.Fatalf("download suggestions = %v, want %v", dialog.downloadNames, want)
	}
	items := ui.transfers.items()
	if len(items) != 2 || items[0].Destination != firstDestination || items[1].Destination != secondDestination {
		t.Fatalf("queued download selections = %+v", items)
	}
}

func TestSFTPFileDialogCancellationIsSilent(t *testing.T) {
	ui, tab := newSFTPFileDialogTestWindow(t)
	ui.sftpFiles = &fakeSFTPFileDialog{uploadErr: errSFTPFileDialogCancelled}
	ui.model.Error = ""

	if !ui.requestSFTPUploadFiles(tab.ID) {
		t.Fatal("cancelled upload dialog did not start")
	}
	waitForSFTPUIEvent(t, ui)
	if ui.model.Error != "" {
		t.Fatalf("cancelled dialog error = %q, want empty", ui.model.Error)
	}
	if len(ui.transfers.items()) != 0 {
		t.Fatal("cancelled dialog queued a transfer")
	}
}

func TestSFTPDialogPathsCloseNativeHandlesAndRejectUnnamedFiles(t *testing.T) {
	first := &namedDialogReader{Reader: bytes.NewReader(nil), name: `C:\first.txt`}
	second := &namedDialogReader{Reader: bytes.NewReader(nil), name: `C:\second.txt`}
	paths, err := sftpUploadPathsFromReaders([]io.ReadCloser{first, second})
	if err != nil {
		t.Fatalf("upload paths: %v", err)
	}
	if want := []string{`C:\first.txt`, `C:\second.txt`}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("upload paths = %v, want %v", paths, want)
	}
	if !first.closed || !second.closed {
		t.Fatal("upload dialog handles were not closed")
	}

	unnamed := &namedDialogReader{Reader: bytes.NewReader(nil)}
	if _, err := sftpUploadPathsFromReaders([]io.ReadCloser{unnamed}); err == nil || !unnamed.closed {
		t.Fatalf("unnamed upload = error %v, closed %v", err, unnamed.closed)
	}

	writer := &namedDialogWriter{name: `C:\download.txt`}
	path, err := sftpDownloadPathFromWriter(writer)
	if err != nil || path != writer.name || !writer.closed {
		t.Fatalf("download path = %q, error %v, closed %v", path, err, writer.closed)
	}
}

func newSFTPFileDialogTestWindow(t *testing.T) (*Window, *sshTab) {
	t.Helper()
	ui := NewWindow(nil)
	ui.transfers.close()
	ui.transfers = newTransferManager(1, func(ctx context.Context, _ transferItem, _ func(int64)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	t.Cleanup(ui.transfers.close)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	return ui, tab
}

var _ sftpFileDialog = (*fakeSFTPFileDialog)(nil)
