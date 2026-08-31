package gui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

type transferBufferWriter struct {
	*bytes.Buffer
}

func (*transferBufferWriter) Close() error { return nil }

type transferStart struct {
	id     string
	offset int64
}

type transferSFTPClient struct {
	*testSFTPClient
	remoteData        map[string][]byte
	written           map[string]*bytes.Buffer
	readerPath        string
	readerOffset      int64
	writerPath        string
	writerOffset      int64
	writerShouldClear bool
}

type transferSFTPTransport struct {
	*testSSHTransport
	client *transferSFTPClient
}

func (transport *transferSFTPTransport) OpenSFTP() (sshclient.SFTPClient, error) {
	return transport.client, nil
}

func newTransferSFTPClient() *transferSFTPClient {
	return &transferSFTPClient{
		testSFTPClient: &testSFTPClient{},
		remoteData:     make(map[string][]byte),
		written:        make(map[string]*bytes.Buffer),
	}
}

func (client *transferSFTPClient) OpenReader(remotePath string, offset int64) (io.ReadCloser, error) {
	client.readerPath = remotePath
	client.readerOffset = offset
	data := client.remoteData[remotePath]
	if offset < 0 || offset > int64(len(data)) {
		return nil, errors.New("invalid fake reader offset")
	}
	return io.NopCloser(bytes.NewReader(data[offset:])), nil
}

func (client *transferSFTPClient) OpenWriter(remotePath string, offset int64, truncate bool) (io.WriteCloser, error) {
	client.writerPath = remotePath
	client.writerOffset = offset
	client.writerShouldClear = truncate
	buffer := client.written[remotePath]
	if buffer == nil || truncate {
		buffer = &bytes.Buffer{}
		client.written[remotePath] = buffer
	}
	return &transferBufferWriter{Buffer: buffer}, nil
}

func TestRunSFTPTransferResumesAtOffsetAndReportsProgress(t *testing.T) {
	item := transferItem{ID: "transfer-1", Size: 10, Transferred: 4, Status: transferRunning}
	destination := &transferBufferWriter{Buffer: bytes.NewBufferString("0123")}
	var sourceOffset int64 = -1
	var destinationOffset int64 = -1
	var truncate bool
	var progress []int64

	err := runSFTPTransfer(
		context.Background(),
		item,
		func(offset int64) (io.ReadCloser, error) {
			sourceOffset = offset
			return io.NopCloser(bytes.NewReader([]byte("456789"))), nil
		},
		func(offset int64, shouldTruncate bool) (io.WriteCloser, error) {
			destinationOffset = offset
			truncate = shouldTruncate
			return destination, nil
		},
		func(transferred int64) { progress = append(progress, transferred) },
	)
	if err != nil {
		t.Fatalf("run resumed transfer: %v", err)
	}
	if sourceOffset != 4 || destinationOffset != 4 || truncate {
		t.Fatalf("resume open arguments = source %d, destination %d, truncate %v", sourceOffset, destinationOffset, truncate)
	}
	if got := destination.String(); got != "0123456789" {
		t.Fatalf("resumed destination = %q", got)
	}
	if len(progress) == 0 || progress[len(progress)-1] != 10 {
		t.Fatalf("progress = %v, want final 10", progress)
	}
}

func TestRunSFTPTransferStopsBeforeOpeningStreamsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opened := false
	err := runSFTPTransfer(
		ctx,
		transferItem{ID: "transfer-1", Size: 10, Status: transferRunning},
		func(int64) (io.ReadCloser, error) {
			opened = true
			return nil, nil
		},
		func(int64, bool) (io.WriteCloser, error) {
			opened = true
			return nil, nil
		},
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled transfer error = %v", err)
	}
	if opened {
		t.Fatal("cancelled transfer opened a stream")
	}
}

func TestTransferProgressReportsRateAndRemainingTime(t *testing.T) {
	started := time.Unix(100, 0)
	item := transferItem{
		Size:              100,
		StartedAt:         started,
		LastProgressAt:    started,
		LastProgressBytes: 0,
	}
	if !applyTransferProgress(&item, 40, started.Add(10*time.Second)) {
		t.Fatal("apply transfer progress")
	}
	metrics := calculateTransferMetrics(item)
	if metrics.BytesPerSecond != 4 || metrics.RemainingSeconds != 15 || !metrics.HasETA {
		t.Fatalf("transfer metrics = %+v, want 4 bytes/s and 15 seconds", metrics)
	}

	if !applyTransferProgress(&item, 100, started.Add(25*time.Second)) {
		t.Fatal("apply completed transfer progress")
	}
	if metrics := calculateTransferMetrics(item); metrics.HasETA || metrics.RemainingSeconds != 0 {
		t.Fatalf("completed transfer metrics = %+v, want no ETA", metrics)
	}
}

func TestRunSFTPTransferVerifiesExpectedSHA256(t *testing.T) {
	payload := []byte("integrity-payload")
	digest := sha256.Sum256(payload)
	item := transferItem{
		Size:           int64(len(payload)),
		Status:         transferRunning,
		ExpectedSHA256: hex.EncodeToString(digest[:]),
	}
	openSource := func(int64) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	openDestination := func(int64, bool) (io.WriteCloser, error) {
		return &transferBufferWriter{Buffer: new(bytes.Buffer)}, nil
	}
	if err := runSFTPTransfer(context.Background(), item, openSource, openDestination, nil); err != nil {
		t.Fatalf("matching transfer checksum: %v", err)
	}

	item.ExpectedSHA256 = strings.Repeat("0", sha256.Size*2)
	if err := runSFTPTransfer(context.Background(), item, openSource, openDestination, nil); !errors.Is(err, errSFTPTransferIntegrity) {
		t.Fatalf("mismatched transfer checksum error = %v, want integrity error", err)
	}
}

type checksumSSHTransport struct {
	*testSSHTransport
	command string
	output  string
	err     error
}

func (transport *checksumSSHTransport) ExecContext(_ context.Context, command string) (string, error) {
	transport.command = command
	return transport.output, transport.err
}

type checksumSFTPTransport struct {
	*transferSFTPTransport
	command string
	output  string
}

func (transport *checksumSFTPTransport) ExecContext(_ context.Context, command string) (string, error) {
	transport.command = command
	return transport.output, nil
}

func TestRemoteFileSHA256QuotesPathAndParsesDigest(t *testing.T) {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	transport := &checksumSSHTransport{
		testSSHTransport: &testSSHTransport{},
		output:           digest + "  /srv/a user's report.txt\n",
	}
	credentials := remote.SSHHostCredentials{ID: "host-1", Version: 3}
	got, err := remoteFileSHA256(context.Background(), newSSHConnectionPool(), credentials, "/srv/a user's report.txt", func(remote.SSHHostCredentials) (sshTransport, error) {
		return transport, nil
	})
	if err != nil || got != digest {
		t.Fatalf("remote checksum = %q, error %v", got, err)
	}
	if transport.command != "sha256sum -- '/srv/a user'\"'\"'s report.txt'" {
		t.Fatalf("checksum command = %q", transport.command)
	}
	if transport.closed != 1 {
		t.Fatalf("transport close count = %d, want 1", transport.closed)
	}
}

func TestVerifyLocalFileSHA256ChecksEntireResumedFile(t *testing.T) {
	payload := []byte("complete resumed payload")
	path := filepath.Join(t.TempDir(), "download.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if err := verifyLocalFileSHA256(path, hex.EncodeToString(digest[:])); err != nil {
		t.Fatalf("matching checksum failed: %v", err)
	}
	if err := verifyLocalFileSHA256(path, strings.Repeat("0", 64)); !errors.Is(err, errSFTPTransferIntegrity) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestWindowDownloadFetchesRemoteChecksumAndVerifiesEntireResumedFile(t *testing.T) {
	payload := []byte("complete resumed download")
	digest := sha256.Sum256(payload)
	localPath := filepath.Join(t.TempDir(), "download.bin")
	if err := os.WriteFile(localPath, payload[:9], 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := remote.SSHHostCredentials{ID: "host-1", Version: 5}
	remoteClient := newTransferSFTPClient()
	remoteClient.remoteData["/srv/download.bin"] = payload
	baseTransport := &transferSFTPTransport{testSSHTransport: &testSSHTransport{}, client: remoteClient}
	transport := &checksumSFTPTransport{
		transferSFTPTransport: baseTransport,
		output:                hex.EncodeToString(digest[:]) + "  /srv/download.bin\n",
	}
	ui := NewWindow(nil)
	ui.model.RemoteSession = &sftpRemoteSession{fakeRemoteSession: fakeRemoteSession{}, credentials: credentials}
	ui.sshTransportFactory = func(remote.SSHHostCredentials) (sshTransport, error) { return transport, nil }

	err := ui.executeSFTPTransfer(context.Background(), transferItem{
		ID:          "transfer-1",
		Direction:   transferDownload,
		HostID:      credentials.ID,
		Source:      "/srv/download.bin",
		Destination: localPath,
		Size:        int64(len(payload)),
		Transferred: 9,
	}, nil)
	if err != nil {
		t.Fatalf("execute resumed verified download: %v", err)
	}
	if transport.command != "sha256sum -- '/srv/download.bin'" {
		t.Fatalf("checksum command = %q", transport.command)
	}
	if got, err := os.ReadFile(localPath); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("downloaded payload = %q, error %v", got, err)
	}
}

func TestTransferProgressTextIncludesRateAndETA(t *testing.T) {
	item := transferItem{
		Size:           100,
		Transferred:    40,
		Status:         transferRunning,
		BytesPerSecond: 4,
	}
	got := transferProgressText(item)
	if !strings.Contains(got, "4.0 B/s") || !strings.Contains(got, "ETA 15s") {
		t.Fatalf("transfer progress text = %q, want rate and ETA", got)
	}
}

func TestParseDroppedFilePathsAcceptsURIListAndWindowsPaths(t *testing.T) {
	payload := "# dragged files\r\n\r\nfile:///C:/Users/deploy/report%20one.txt\r\nC:\\Users\\deploy\\notes.txt\r\nfile://server/share/archive.zip\r\n"

	paths, err := parseDroppedFilePaths(payload)
	if err != nil {
		t.Fatalf("parse dropped files: %v", err)
	}
	want := []string{
		`C:\Users\deploy\report one.txt`,
		`C:\Users\deploy\notes.txt`,
		`\\server\share\archive.zip`,
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("dropped paths = %#v, want %#v", paths, want)
	}
}

func TestParseDroppedFilePathsRejectsUnsupportedOrMalformedURIs(t *testing.T) {
	for _, payload := range []string{
		"https://example.com/file.txt\n",
		"file:///%ZZ\n",
		"relative/path.txt\n",
	} {
		if _, err := parseDroppedFilePaths(payload); err == nil {
			t.Errorf("parse dropped payload %q succeeded, want error", payload)
		}
	}
}

func TestParseDroppedFilePathsIgnoresCommentsAndEmptyInput(t *testing.T) {
	paths, err := parseDroppedFilePaths("# comment\n\n")
	if err != nil {
		t.Fatalf("parse empty dropped payload: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("empty dropped paths = %#v, want empty", paths)
	}
}

func TestReadSFTPDropPayloadEnforcesSizeLimitAndParsesPaths(t *testing.T) {
	valid := "C:\\Users\\deploy\\one.txt\nC:\\Users\\deploy\\two.txt\n"
	paths, err := readSFTPDropPayload(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("read valid drop payload: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{`C:\Users\deploy\one.txt`, `C:\Users\deploy\two.txt`}) {
		t.Fatalf("drop payload paths = %#v", paths)
	}

	oversized := strings.NewReader(strings.Repeat("x", sftpDropMaxBytes+1))
	if _, err := readSFTPDropPayload(oversized); !errors.Is(err, errSFTPDropTooLarge) {
		t.Fatalf("oversized drop payload error = %v, want size error", err)
	}
}

func TestReadSFTPDropDataClosesReaderOnSuccessAndFailure(t *testing.T) {
	valid := &namedDialogReader{Reader: bytes.NewReader([]byte(`C:\Users\deploy\one.txt`))}
	if _, err := readSFTPDropData(valid); err != nil {
		t.Fatalf("read valid drop data: %v", err)
	}
	if !valid.closed {
		t.Fatal("valid drop reader was not closed")
	}

	invalid := &namedDialogReader{Reader: bytes.NewReader([]byte("relative.txt"))}
	if _, err := readSFTPDropData(invalid); err == nil {
		t.Fatal("invalid drop data succeeded")
	}
	if !invalid.closed {
		t.Fatal("invalid drop reader was not closed")
	}
}

func TestHandleSFTPDropDataQueuesParsedFilesAndReportsInputErrors(t *testing.T) {
	first, err := os.CreateTemp("", "s12ryt-drop-one-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	firstPath := first.Name()
	if _, err := first.WriteString("one"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(firstPath)

	second, err := os.CreateTemp("", "s12ryt-drop-two-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	secondPath := second.Name()
	if _, err := second.WriteString("two"); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(secondPath)

	ui := NewWindow(nil)
	tab := ui.sshTabs.open(remote.SSHHost{ID: "host-1", Host: "example.test", Port: 22})
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	ui.transfers = newTransferManager(3, func(context.Context, transferItem, func(int64)) error {
		return nil
	})
	defer ui.transfers.close()

	reader := &namedDialogReader{Reader: bytes.NewReader([]byte(firstPath + "\n" + secondPath))}
	if !ui.handleSFTPDropData(tab.ID, reader) {
		t.Fatal("handle valid SFTP drop")
	}
	if !reader.closed {
		t.Fatal("valid drop reader was not closed")
	}
	items := ui.transfers.items()
	if len(items) != 2 || items[0].Source != firstPath || items[1].Source != secondPath {
		t.Fatalf("queued drop items = %+v", items)
	}

	ui.model.Error = ""
	invalid := &namedDialogReader{Reader: bytes.NewReader([]byte("relative.txt"))}
	if !ui.handleSFTPDropData(tab.ID, invalid) {
		t.Fatal("handle invalid SFTP drop event")
	}
	if !invalid.closed || ui.model.Error != ui.text("Dropped files are invalid.") {
		t.Fatalf("invalid drop state: closed=%v error=%q", invalid.closed, ui.model.Error)
	}

	ui.model.Error = ""
	oversized := &namedDialogReader{Reader: bytes.NewReader(bytes.Repeat([]byte("x"), sftpDropMaxBytes+1))}
	if !ui.handleSFTPDropData(tab.ID, oversized) {
		t.Fatal("handle oversized SFTP drop event")
	}
	if !oversized.closed || ui.model.Error != ui.text("Dropped file data is too large.") {
		t.Fatalf("oversized drop state: closed=%v error=%q", oversized.closed, ui.model.Error)
	}
}

func TestTransferManagerPausesResumesFromProgressAndReleasesConcurrency(t *testing.T) {
	starts := make(chan transferStart, 4)
	finishSecond := make(chan struct{})
	manager := newTransferManager(1, func(ctx context.Context, item transferItem, progress func(int64)) error {
		starts <- transferStart{id: item.ID, offset: item.Transferred}
		switch item.Source {
		case "first":
			if item.Transferred == 0 {
				progress(4)
				<-ctx.Done()
				return ctx.Err()
			}
			progress(item.Size)
			return nil
		case "second":
			<-finishSecond
			progress(item.Size)
			return nil
		default:
			return errors.New("unexpected transfer")
		}
	})
	defer manager.close()

	first := manager.enqueue(transferUpload, "host-1", "first", "/first", 10)
	second := manager.enqueue(transferUpload, "host-1", "second", "/second", 6)
	if got := waitTransferStart(t, starts); got.id != first.ID || got.offset != 0 {
		t.Fatalf("first start = %+v", got)
	}
	waitTransferStatus(t, manager, first.ID, transferRunning, 4)

	if !manager.pause(first.ID) {
		t.Fatal("pause running transfer")
	}
	waitTransferStatus(t, manager, first.ID, transferPaused, 4)
	if got := waitTransferStart(t, starts); got.id != second.ID || got.offset != 0 {
		t.Fatalf("second start after pause = %+v", got)
	}

	if !manager.resume(first.ID) {
		t.Fatal("resume paused transfer")
	}
	if snapshot, ok := manager.item(first.ID); !ok || snapshot.Status != transferQueued {
		t.Fatalf("resumed transfer before slot opens = %+v, %v", snapshot, ok)
	}
	close(finishSecond)
	waitTransferStatus(t, manager, second.ID, transferCompleted, second.Size)
	if got := waitTransferStart(t, starts); got.id != first.ID || got.offset != 4 {
		t.Fatalf("resumed start = %+v", got)
	}
	waitTransferStatus(t, manager, first.ID, transferCompleted, first.Size)
}

func TestRunSFTPTransferDirectionMapsUploadAndDownloadStreams(t *testing.T) {
	localDirectory := t.TempDir()
	uploadSource := filepath.Join(localDirectory, "upload.txt")
	if err := os.WriteFile(uploadSource, []byte("upload-data"), 0o600); err != nil {
		t.Fatalf("write upload fixture: %v", err)
	}
	client := newTransferSFTPClient()
	if err := runSFTPTransferDirection(context.Background(), transferItem{
		Direction:   transferUpload,
		Source:      uploadSource,
		Destination: "/upload.txt",
		Size:        11,
	}, client, nil); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if client.writerPath != "/upload.txt" || client.writerOffset != 0 || !client.writerShouldClear {
		t.Fatalf("upload writer = %q offset %d truncate %v", client.writerPath, client.writerOffset, client.writerShouldClear)
	}
	if got := client.written["/upload.txt"].String(); got != "upload-data" {
		t.Fatalf("remote upload = %q", got)
	}

	downloadDestination := filepath.Join(localDirectory, "download.txt")
	if err := os.WriteFile(downloadDestination, []byte("stale-content"), 0o600); err != nil {
		t.Fatalf("write download fixture: %v", err)
	}
	client.remoteData["/download.txt"] = []byte("download-data")
	if err := runSFTPTransferDirection(context.Background(), transferItem{
		Direction:   transferDownload,
		Source:      "/download.txt",
		Destination: downloadDestination,
		Size:        13,
	}, client, nil); err != nil {
		t.Fatalf("download: %v", err)
	}
	if client.readerPath != "/download.txt" || client.readerOffset != 0 {
		t.Fatalf("download reader = %q offset %d", client.readerPath, client.readerOffset)
	}
	if got, err := os.ReadFile(downloadDestination); err != nil || string(got) != "download-data" {
		t.Fatalf("local download = %q, %v", got, err)
	}
}

func TestWindowSFTPTransferUsesVersionedCredentialsAndIndependentPoolLease(t *testing.T) {
	localPath := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localPath, []byte("pooled"), 0o600); err != nil {
		t.Fatalf("write upload fixture: %v", err)
	}
	credentials := remote.SSHHostCredentials{
		ID:       "host-1",
		Host:     "server.example",
		Port:     22,
		Username: "deploy",
		Password: "secret",
		Version:  7,
	}
	ui := NewWindow(nil)
	ui.model.RemoteSession = &sftpRemoteSession{fakeRemoteSession: fakeRemoteSession{}, credentials: credentials}
	remoteClient := newTransferSFTPClient()
	transport := &transferSFTPTransport{testSSHTransport: &testSSHTransport{}, client: remoteClient}
	factoryCalls := 0
	ui.sshTransportFactory = func(got remote.SSHHostCredentials) (sshTransport, error) {
		factoryCalls++
		if got.ID != credentials.ID || got.Version != credentials.Version {
			t.Fatalf("factory credentials = %+v", got)
		}
		return transport, nil
	}
	var progress int64
	err := ui.executeSFTPTransfer(context.Background(), transferItem{
		ID:          "transfer-1",
		Direction:   transferUpload,
		HostID:      credentials.ID,
		Source:      localPath,
		Destination: "/upload.txt",
		Size:        6,
	}, func(transferred int64) { progress = transferred })
	if err != nil {
		t.Fatalf("execute pooled upload: %v", err)
	}
	if factoryCalls != 1 || transport.closed != 1 || remoteClient.closed != 1 {
		t.Fatalf("pool lifecycle = factory %d transport close %d SFTP close %d", factoryCalls, transport.closed, remoteClient.closed)
	}
	if got := remoteClient.written["/upload.txt"].String(); got != "pooled" || progress != 6 {
		t.Fatalf("pooled upload = %q progress %d", got, progress)
	}
}

func TestWindowOwnsAndClosesItsTransferManager(t *testing.T) {
	ui := NewWindow(nil)
	if ui.transfers == nil {
		t.Fatal("new window did not initialize its transfer manager")
	}
	if err := ui.Close(); err != nil {
		t.Fatalf("close window: %v", err)
	}
	if item := ui.transfers.enqueue(transferUpload, "host-1", "source", "destination", 1); item != nil {
		t.Fatalf("closed window accepted transfer %+v", item)
	}
}

func TestTransferPanelSnapshotsItemsAndExposesStatusActions(t *testing.T) {
	manager := newTransferManager(1, func(ctx context.Context, _ transferItem, _ func(int64)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	defer manager.close()
	first := manager.enqueue(transferUpload, "host-1", "first", "/first", 10)
	second := manager.enqueue(transferDownload, "host-1", "/second", "second", 20)
	items := manager.items()
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("transfer snapshots = %+v", items)
	}
	items[0].Source = "mutated"
	if original, _ := manager.item(first.ID); original.Source != "first" {
		t.Fatal("transfer panel snapshot mutated queue state")
	}

	cases := map[transferStatus]string{
		transferRunning:   "Pause",
		transferQueued:    "Pause",
		transferPaused:    "Resume",
		transferFailed:    "Retry transfer",
		transferCompleted: "",
	}
	for status, want := range cases {
		if got := transferActionSource(status); got != want {
			t.Errorf("action for %s = %q, want %q", status, got, want)
		}
	}
}

func TestEnqueueSFTPTransfersMapsCurrentBrowserPathsAndFileSizes(t *testing.T) {
	ui := NewWindow(nil)
	ui.transfers.close()
	ui.transfers = newTransferManager(1, func(ctx context.Context, _ transferItem, _ func(int64)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	defer ui.transfers.close()
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv/data")
	tab.sftpBrowser.applyEntries([]sftpEntry{
		{Name: "remote.txt", Path: "/srv/data/remote.txt", Size: 12},
		{Name: "folder", Path: "/srv/data/folder", Directory: true},
	})

	localUpload := filepath.Join(t.TempDir(), "local.txt")
	if err := os.WriteFile(localUpload, []byte("upload"), 0o600); err != nil {
		t.Fatalf("write local upload: %v", err)
	}
	if count := ui.enqueueSFTPUploads(tab.ID, []string{localUpload}); count != 1 {
		t.Fatalf("enqueued uploads = %d", count)
	}
	downloadPath := filepath.Join(t.TempDir(), "remote.txt")
	if count := ui.enqueueSFTPDownloads(tab.ID, []sftpDownloadTarget{
		{RemotePath: "/srv/data/remote.txt", LocalPath: downloadPath},
		{RemotePath: "/srv/data/folder", LocalPath: filepath.Join(t.TempDir(), "folder")},
	}); count != 1 {
		t.Fatalf("enqueued downloads = %d", count)
	}

	items := ui.transfers.items()
	if len(items) != 2 {
		t.Fatalf("transfer items = %+v", items)
	}
	if upload := items[0]; upload.Direction != transferUpload || upload.HostID != tab.HostID || upload.Source != localUpload || upload.Destination != "/srv/data/local.txt" || upload.Size != 6 {
		t.Fatalf("upload mapping = %+v", upload)
	}
	if download := items[1]; download.Direction != transferDownload || download.Source != "/srv/data/remote.txt" || download.Destination != downloadPath || download.Size != 12 {
		t.Fatalf("download mapping = %+v", download)
	}
}

func TestBuildSFTPUploadCandidatesDetectsConflictsAndKeepsBothNamesUnique(t *testing.T) {
	localDirectory := t.TempDir()
	first := filepath.Join(localDirectory, "report.txt")
	second := filepath.Join(localDirectory, "notes.txt")
	for path, content := range map[string]string{first: "report", second: "notes"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write upload candidate: %v", err)
		}
	}
	browser := newSFTPBrowserState("/srv")
	browser.applyEntries([]sftpEntry{
		{Name: "report.txt", Path: "/srv/report.txt", Size: 20},
		{Name: "report (2).txt", Path: "/srv/report (2).txt", Size: 21},
	})

	candidates := buildSFTPUploadCandidates(browser, []string{first, second})
	if len(candidates) != 2 {
		t.Fatalf("upload candidates = %+v", candidates)
	}
	if !candidates[0].Conflict || candidates[0].RemotePath != "/srv/report.txt" || candidates[0].Size != 6 {
		t.Fatalf("conflicting upload = %+v", candidates[0])
	}
	if candidates[1].Conflict || candidates[1].RemotePath != "/srv/notes.txt" || candidates[1].Size != 5 {
		t.Fatalf("non-conflicting upload = %+v", candidates[1])
	}
	existing := map[string]bool{"report.txt": true, "report (2).txt": true}
	if got := sftpKeepBothName("report.txt", existing); got != "report (3).txt" {
		t.Fatalf("keep-both name = %q, want report (3).txt", got)
	}
}

func TestPrepareSFTPUploadsQueuesConflictsAndResolvesEveryDecision(t *testing.T) {
	ui := NewWindow(nil)
	ui.transfers.close()
	ui.transfers = newTransferManager(1, func(ctx context.Context, _ transferItem, _ func(int64)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	defer ui.transfers.close()
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	tab.sftpBrowser.applyEntries([]sftpEntry{
		{Name: "report.txt", Path: "/srv/report.txt"},
		{Name: "report (2).txt", Path: "/srv/report (2).txt"},
	})

	firstDirectory := t.TempDir()
	secondDirectory := t.TempDir()
	firstReport := filepath.Join(firstDirectory, "report.txt")
	secondReport := filepath.Join(secondDirectory, "report.txt")
	notes := filepath.Join(firstDirectory, "notes.txt")
	for file, content := range map[string]string{firstReport: "one", secondReport: "two", notes: "notes"} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatalf("write conflict fixture: %v", err)
		}
	}

	if count := ui.prepareSFTPUploads(tab.ID, []string{firstReport, notes, secondReport}); count != 1 {
		t.Fatalf("immediate non-conflicting uploads = %d, want 1", count)
	}
	if !ui.sftpUploadConflictOpen || len(ui.sftpUploadConflicts) != 2 {
		t.Fatalf("upload conflict queue = open %v, entries %+v", ui.sftpUploadConflictOpen, ui.sftpUploadConflicts)
	}
	if !ui.resolveSFTPUploadConflict("Keep both") {
		t.Fatal("keep-both decision was rejected")
	}
	if !ui.sftpUploadConflictOpen || len(ui.sftpUploadConflicts) != 1 {
		t.Fatalf("remaining conflicts = open %v, entries %+v", ui.sftpUploadConflictOpen, ui.sftpUploadConflicts)
	}
	if !ui.resolveSFTPUploadConflict("Skip") {
		t.Fatal("skip decision was rejected")
	}
	if ui.sftpUploadConflictOpen || len(ui.sftpUploadConflicts) != 0 {
		t.Fatalf("resolved conflict queue = open %v, entries %+v", ui.sftpUploadConflictOpen, ui.sftpUploadConflicts)
	}

	items := ui.transfers.items()
	if len(items) != 2 {
		t.Fatalf("resolved transfer items = %+v", items)
	}
	if items[0].Destination != "/srv/notes.txt" || items[1].Destination != "/srv/report (3).txt" {
		t.Fatalf("resolved destinations = %q, %q", items[0].Destination, items[1].Destination)
	}
}

var _ sshclient.SFTPClient = (*transferSFTPClient)(nil)
var _ sshTransport = (*transferSFTPTransport)(nil)
var _ sshSFTPTransport = (*transferSFTPTransport)(nil)

func waitTransferStart(t *testing.T, starts <-chan transferStart) transferStart {
	t.Helper()
	select {
	case started := <-starts:
		return started
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for transfer worker")
		return transferStart{}
	}
}

func waitTransferStatus(t *testing.T, manager *transferManager, id string, status transferStatus, transferred int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		item, ok := manager.item(id)
		if ok && item.Status == status && item.Transferred == transferred {
			return
		}
		time.Sleep(time.Millisecond)
	}
	item, ok := manager.item(id)
	t.Fatalf("transfer %s = %+v, %v; want status %s progress %d", id, item, ok, status, transferred)
}
