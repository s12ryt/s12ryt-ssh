package gui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"

	"gioui.org/layout"
	"gioui.org/widget"
)

type transferReaderOpener func(offset int64) (io.ReadCloser, error)

type transferWriterOpener func(offset int64, truncate bool) (io.WriteCloser, error)

type sftpDownloadTarget struct {
	RemotePath string
	LocalPath  string
}

type sftpUploadCandidate struct {
	LocalPath  string
	RemotePath string
	Size       int64
	Conflict   bool
}

type sftpUploadConflict struct {
	TabID     string
	Candidate sftpUploadCandidate
}

var errSFTPTransferIntegrity = errors.New("SFTP transfer integrity check failed")
var errSFTPChecksumUnavailable = errors.New("SFTP remote checksum is unavailable")

type transferMetrics struct {
	BytesPerSecond   float64
	RemainingSeconds int64
	HasETA           bool
}

func applyTransferProgress(item *transferItem, transferred int64, at time.Time) bool {
	if item == nil {
		return false
	}
	if transferred < 0 {
		transferred = 0
	}
	if item.Size >= 0 && transferred > item.Size {
		transferred = item.Size
	}
	if !at.IsZero() && !item.LastProgressAt.IsZero() {
		elapsed := at.Sub(item.LastProgressAt).Seconds()
		advanced := transferred - item.LastProgressBytes
		if elapsed > 0 && advanced > 0 {
			item.BytesPerSecond = float64(advanced) / elapsed
		}
	}
	item.Transferred = transferred
	item.LastProgressBytes = transferred
	if !at.IsZero() {
		item.LastProgressAt = at
	}
	return true
}

func calculateTransferMetrics(item transferItem) transferMetrics {
	metrics := transferMetrics{BytesPerSecond: item.BytesPerSecond}
	if item.Size <= item.Transferred || item.BytesPerSecond <= 0 {
		return metrics
	}
	remaining := float64(item.Size - item.Transferred)
	metrics.RemainingSeconds = int64(math.Ceil(remaining / item.BytesPerSecond))
	metrics.HasETA = true
	return metrics
}

func remoteFileSHA256(
	ctx context.Context,
	pool *sshConnectionPool,
	credentials remote.SSHHostCredentials,
	remotePath string,
	factory sshTransportFactory,
) (string, error) {
	if ctx == nil || pool == nil || credentials.ID == "" || credentials.Version < 1 || factory == nil {
		return "", errSFTPChecksumUnavailable
	}
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "" {
		return "", errSFTPChecksumUnavailable
	}
	lease, err := pool.acquire(sshConnectionKey{HostID: credentials.ID, Version: credentials.Version}, func() (sshTransport, error) {
		return factory(credentials)
	})
	if err != nil {
		return "", err
	}
	defer lease.Close()
	transport, ok := lease.transport().(sshChecksumTransport)
	if !ok {
		return "", errSFTPChecksumUnavailable
	}
	output, err := transport.ExecContext(ctx, "sha256sum -- "+quotePOSIXShellArgument(remotePath))
	if err != nil {
		return "", errors.Join(errSFTPChecksumUnavailable, err)
	}
	fields := strings.Fields(output)
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return "", errSFTPChecksumUnavailable
	}
	decoded, err := hex.DecodeString(fields[0])
	if err != nil || len(decoded) != sha256.Size {
		return "", errSFTPChecksumUnavailable
	}
	return strings.ToLower(fields[0]), nil
}

func quotePOSIXShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func verifyLocalFileSHA256(localPath, expected string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(expected))
	if err != nil || len(decoded) != sha256.Size {
		return errSFTPTransferIntegrity
	}
	if !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), strings.TrimSpace(expected)) {
		return errSFTPTransferIntegrity
	}
	return nil
}

func buildSFTPUploadCandidates(browser *sftpBrowserState, localPaths []string) []sftpUploadCandidate {
	if browser == nil {
		return nil
	}
	existing := make(map[string]bool, len(browser.Entries))
	for _, entry := range browser.Entries {
		existing[cleanRemotePath(entry.Path)] = true
	}
	candidates := make([]sftpUploadCandidate, 0, len(localPaths))
	for _, localPath := range localPaths {
		info, err := os.Stat(localPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		remotePath, ok := sftpChildPath(browser.Path, filepath.Base(localPath))
		if !ok {
			continue
		}
		candidates = append(candidates, sftpUploadCandidate{
			LocalPath:  localPath,
			RemotePath: remotePath,
			Size:       info.Size(),
			Conflict:   existing[remotePath],
		})
		existing[remotePath] = true
	}
	return candidates
}

func sftpKeepBothName(name string, existing map[string]bool) string {
	name = filepath.Base(name)
	extension := path.Ext(name)
	stem := name[:len(name)-len(extension)]
	for suffix := 2; ; suffix++ {
		candidate := stem + " (" + strconv.Itoa(suffix) + ")" + extension
		if !existing[candidate] {
			return candidate
		}
	}
}

func (ui *Window) prepareSFTPUploads(tabID string, localPaths []string) int {
	tab := ui.transferSFTPTab(tabID)
	if tab == nil || ui.transfers == nil {
		return 0
	}
	candidates := buildSFTPUploadCandidates(tab.sftpBrowser, localPaths)
	count := 0
	for _, candidate := range candidates {
		if candidate.Conflict {
			ui.sftpUploadConflicts = append(ui.sftpUploadConflicts, sftpUploadConflict{
				TabID:     tabID,
				Candidate: candidate,
			})
			continue
		}
		if ui.enqueueSFTPUploadCandidate(tab, candidate, candidate.RemotePath) {
			count++
		}
	}
	ui.sftpUploadConflictOpen = len(ui.sftpUploadConflicts) > 0
	return count
}

func (ui *Window) resolveSFTPUploadConflict(decision string) bool {
	if ui == nil || len(ui.sftpUploadConflicts) == 0 {
		return false
	}
	conflict := ui.sftpUploadConflicts[0]
	tab := ui.transferSFTPTab(conflict.TabID)
	resolved := false
	switch decision {
	case "Overwrite":
		resolved = true
		if tab != nil {
			ui.enqueueSFTPUploadCandidate(tab, conflict.Candidate, conflict.Candidate.RemotePath)
		}
	case "Skip":
		resolved = true
	case "Keep both":
		resolved = true
		if tab != nil {
			existing := ui.sftpUploadDestinationNames(tab)
			name := sftpKeepBothName(path.Base(conflict.Candidate.RemotePath), existing)
			destination, ok := sftpChildPath(tab.sftpBrowser.Path, name)
			if ok {
				ui.enqueueSFTPUploadCandidate(tab, conflict.Candidate, destination)
			}
		}
	default:
		return false
	}
	if resolved {
		ui.sftpUploadConflicts = ui.sftpUploadConflicts[1:]
		if len(ui.sftpUploadConflicts) == 0 {
			ui.closeSFTPUploadConflicts()
		} else {
			ui.sftpUploadConflictOpen = true
		}
	}
	return resolved
}

func (ui *Window) handleSFTPUploadConflict(gtx layout.Context) {
	if ui == nil || !ui.sftpUploadConflictOpen {
		return
	}
	if ui.sftpUploadConflictScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSFTPUploadConflicts()
		return
	}
	if ui.sftpUploadOverwrite.Clicked(gtx) {
		ui.resolveSFTPUploadConflict("Overwrite")
		return
	}
	if ui.sftpUploadSkip.Clicked(gtx) {
		ui.resolveSFTPUploadConflict("Skip")
		return
	}
	if ui.sftpUploadKeepBoth.Clicked(gtx) {
		ui.resolveSFTPUploadConflict("Keep both")
	}
}

func (ui *Window) enqueueSFTPUploadCandidate(tab *sshTab, candidate sftpUploadCandidate, destination string) bool {
	if ui == nil || tab == nil || ui.transfers == nil || strings.TrimSpace(destination) == "" {
		return false
	}
	item := ui.transfers.enqueue(
		transferUpload,
		tab.HostID,
		candidate.LocalPath,
		cleanRemotePath(destination),
		candidate.Size,
	)
	if item == nil {
		return false
	}
	ui.transferPanelOpen = true
	return true
}

func (ui *Window) sftpUploadDestinationNames(tab *sshTab) map[string]bool {
	existing := make(map[string]bool)
	if tab == nil || tab.sftpBrowser == nil {
		return existing
	}
	for _, entry := range tab.sftpBrowser.Entries {
		existing[path.Base(entry.Path)] = true
	}
	if ui.transfers == nil {
		return existing
	}
	parent := cleanRemotePath(tab.sftpBrowser.Path)
	for _, item := range ui.transfers.items() {
		if item.HostID == tab.HostID && item.Direction == transferUpload && path.Dir(cleanRemotePath(item.Destination)) == parent {
			existing[path.Base(item.Destination)] = true
		}
	}
	return existing
}

func (ui *Window) closeSFTPUploadConflicts() {
	if ui == nil {
		return
	}
	ui.sftpUploadConflictOpen = false
	ui.sftpUploadConflicts = nil
	ui.sftpUploadOverwrite = widget.Clickable{}
	ui.sftpUploadSkip = widget.Clickable{}
	ui.sftpUploadKeepBoth = widget.Clickable{}
	ui.sftpUploadConflictScrim = widget.Clickable{}
}

type transferExecutor func(context.Context, transferItem, func(int64)) error

type transferManager struct {
	queue   *transferQueue
	execute transferExecutor

	mu            sync.Mutex
	workers       map[string]context.CancelFunc
	disabledHosts map[string]bool
	closed        bool
	onChange      func()
}

func newTransferManager(maxConcurrent int, execute transferExecutor) *transferManager {
	return &transferManager{
		queue:         newTransferQueue(maxConcurrent),
		execute:       execute,
		workers:       make(map[string]context.CancelFunc),
		disabledHosts: make(map[string]bool),
	}
}

func (manager *transferManager) enqueue(
	direction transferDirection,
	hostID string,
	source string,
	destination string,
	size int64,
) *transferItem {
	if manager == nil || manager.queue == nil || manager.execute == nil {
		return nil
	}
	manager.mu.Lock()
	if manager.closed || manager.disabledHosts[hostID] {
		manager.mu.Unlock()
		return nil
	}
	item := manager.queue.enqueue(direction, hostID, source, destination, size)
	manager.mu.Unlock()
	manager.reconcile()
	manager.notify()
	return item
}

func (manager *transferManager) item(id string) (transferItem, bool) {
	if manager == nil || manager.queue == nil {
		return transferItem{}, false
	}
	manager.queue.mu.Lock()
	defer manager.queue.mu.Unlock()
	item := manager.queue.findUnlocked(id)
	if item == nil {
		return transferItem{}, false
	}
	return *item, true
}

func (manager *transferManager) items() []transferItem {
	if manager == nil || manager.queue == nil {
		return nil
	}
	manager.queue.mu.Lock()
	defer manager.queue.mu.Unlock()
	items := make([]transferItem, 0, len(manager.queue.items))
	for _, item := range manager.queue.items {
		items = append(items, *item)
	}
	return items
}

func transferActionSource(status transferStatus) string {
	switch status {
	case transferRunning, transferQueued:
		return "Pause"
	case transferPaused:
		return "Resume"
	case transferFailed:
		return "Retry transfer"
	default:
		return ""
	}
}

func (manager *transferManager) pause(id string) bool {
	if manager == nil || manager.queue == nil || !manager.queue.pause(id) {
		return false
	}
	manager.mu.Lock()
	cancel := manager.workers[id]
	manager.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	manager.reconcile()
	manager.notify()
	return true
}

func (manager *transferManager) resume(id string) bool {
	if manager == nil || manager.queue == nil || !manager.queue.resume(id) {
		return false
	}
	manager.reconcile()
	manager.notify()
	return true
}

func (manager *transferManager) retry(id string) bool {
	if manager == nil || manager.queue == nil {
		return false
	}
	item, ok := manager.item(id)
	if !ok {
		return false
	}
	manager.mu.Lock()
	disabled := manager.disabledHosts[item.HostID]
	manager.mu.Unlock()
	if disabled || !manager.queue.retry(id) {
		return false
	}
	manager.reconcile()
	manager.notify()
	return true
}

func (manager *transferManager) setHostEnabled(hostID string, enabled bool, message string) int {
	if manager == nil || manager.queue == nil || strings.TrimSpace(hostID) == "" {
		return 0
	}
	manager.mu.Lock()
	if manager.disabledHosts == nil {
		manager.disabledHosts = make(map[string]bool)
	}
	if enabled {
		delete(manager.disabledHosts, hostID)
		manager.mu.Unlock()
		return 0
	}
	manager.disabledHosts[hostID] = true
	failedIDs := manager.queue.disableHost(hostID, message)
	cancels := make([]context.CancelFunc, 0, len(failedIDs))
	for _, id := range failedIDs {
		if cancel := manager.workers[id]; cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	manager.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	manager.reconcile()
	manager.notify()
	return len(failedIDs)
}

func (manager *transferManager) close() {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return
	}
	manager.closed = true
	cancels := make([]context.CancelFunc, 0, len(manager.workers))
	for _, cancel := range manager.workers {
		cancels = append(cancels, cancel)
	}
	manager.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (manager *transferManager) reconcile() {
	if manager == nil || manager.queue == nil || manager.execute == nil {
		return
	}
	running := manager.runningItems()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed {
		return
	}
	for _, item := range running {
		if _, exists := manager.workers[item.ID]; exists {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		manager.workers[item.ID] = cancel
		go manager.run(ctx, item)
	}
}

func (manager *transferManager) runningItems() []transferItem {
	manager.queue.mu.Lock()
	defer manager.queue.mu.Unlock()
	items := make([]transferItem, 0, manager.queue.maxConcurrent)
	for _, item := range manager.queue.items {
		if item.Status == transferRunning {
			items = append(items, *item)
		}
	}
	return items
}

func (manager *transferManager) run(ctx context.Context, item transferItem) {
	err := manager.execute(ctx, item, func(transferred int64) {
		if manager.queue.updateProgress(item.ID, transferred) {
			manager.notify()
		}
	})

	manager.mu.Lock()
	closed := manager.closed
	manager.mu.Unlock()
	if closed {
		manager.mu.Lock()
		delete(manager.workers, item.ID)
		manager.mu.Unlock()
		return
	}
	current, exists := manager.item(item.ID)
	if !exists || current.Status != transferRunning {
		manager.mu.Lock()
		delete(manager.workers, item.ID)
		manager.mu.Unlock()
		manager.reconcile()
		return
	}
	if err == nil {
		manager.queue.complete(item.ID)
	} else if !errors.Is(err, context.Canceled) {
		manager.queue.fail(item.ID, err.Error())
	}
	manager.mu.Lock()
	delete(manager.workers, item.ID)
	manager.mu.Unlock()
	manager.reconcile()
	manager.notify()
}

func (manager *transferManager) notify() {
	if manager == nil {
		return
	}
	manager.mu.Lock()
	onChange := manager.onChange
	manager.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

func runSFTPTransfer(
	ctx context.Context,
	item transferItem,
	openSource transferReaderOpener,
	openDestination transferWriterOpener,
	onProgress func(int64),
) error {
	if ctx == nil {
		return errors.New("SFTP transfer requires a context")
	}
	if openSource == nil || openDestination == nil {
		return errors.New("SFTP transfer requires source and destination streams")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	offset := item.Transferred
	if offset < 0 {
		offset = 0
	}
	if item.Size >= 0 && offset > item.Size {
		offset = item.Size
	}
	if item.Size >= 0 && offset == item.Size {
		if onProgress != nil {
			onProgress(offset)
		}
		return nil
	}
	expected := strings.TrimSpace(item.ExpectedSHA256)
	if expected != "" {
		digest, decodeErr := hex.DecodeString(expected)
		if decodeErr != nil || len(digest) != sha256.Size {
			return errSFTPTransferIntegrity
		}
	}

	source, err := openSource(offset)
	if err != nil {
		return err
	}
	if source == nil {
		return errors.New("SFTP transfer source is unavailable")
	}
	destination, err := openDestination(offset, offset == 0)
	if err != nil {
		return errors.Join(err, source.Close())
	}
	if destination == nil {
		return errors.Join(errors.New("SFTP transfer destination is unavailable"), source.Close())
	}

	var digest hash.Hash
	if expected != "" {
		digest = sha256.New()
	}
	transferErr := copySFTPTransferWithDigest(ctx, source, destination, item.Size, offset, onProgress, digest)
	if transferErr == nil && digest != nil && !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), expected) {
		transferErr = errSFTPTransferIntegrity
	}
	return errors.Join(transferErr, destination.Close(), source.Close())
}

func runSFTPTransferDirection(
	ctx context.Context,
	item transferItem,
	client sshclient.SFTPClient,
	onProgress func(int64),
) error {
	if client == nil {
		return errors.New("SFTP transfer requires a remote session")
	}
	switch item.Direction {
	case transferUpload:
		return runSFTPTransfer(
			ctx,
			item,
			func(offset int64) (io.ReadCloser, error) {
				return openLocalTransferReader(item.Source, offset)
			},
			func(offset int64, truncate bool) (io.WriteCloser, error) {
				return client.OpenWriter(item.Destination, offset, truncate)
			},
			onProgress,
		)
	case transferDownload:
		return runSFTPTransfer(
			ctx,
			item,
			func(offset int64) (io.ReadCloser, error) {
				return client.OpenReader(item.Source, offset)
			},
			func(offset int64, truncate bool) (io.WriteCloser, error) {
				return openLocalTransferWriter(item.Destination, offset, truncate)
			},
			onProgress,
		)
	default:
		return errors.New("unsupported SFTP transfer direction")
	}
}

func (ui *Window) executeSFTPTransfer(
	ctx context.Context,
	item transferItem,
	onProgress func(int64),
) error {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return errors.New("SFTP transfer requires a remote session")
	}
	if ctx == nil {
		return errors.New("SFTP transfer requires a context")
	}
	credentials, err := ui.model.RemoteSession.SSHHostCredentials(ctx, item.HostID)
	if err != nil {
		return err
	}
	client, err := ui.openPooledSFTP(credentials)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("SFTP transfer session is unavailable")
	}
	expectedChecksum := strings.TrimSpace(item.ExpectedSHA256)
	if item.Direction == transferDownload && expectedChecksum == "" {
		checksum, checksumErr := remoteFileSHA256(ctx, ui.sshPool, credentials, item.Source, ui.sshTransportFactory)
		if checksumErr == nil {
			expectedChecksum = checksum
		} else if ctx.Err() != nil {
			return errors.Join(ctx.Err(), client.Close())
		} else if !errors.Is(checksumErr, errSFTPChecksumUnavailable) {
			return errors.Join(checksumErr, client.Close())
		}
	}
	transferItem := item
	if item.Direction == transferDownload && expectedChecksum != "" {
		transferItem.ExpectedSHA256 = ""
	}
	transferErr := runSFTPTransferDirection(ctx, transferItem, client, onProgress)
	if transferErr == nil && item.Direction == transferDownload && expectedChecksum != "" {
		transferErr = verifyLocalFileSHA256(item.Destination, expectedChecksum)
	}
	return errors.Join(transferErr, client.Close())
}

func (ui *Window) enqueueSFTPUploads(tabID string, localPaths []string) int {
	tab := ui.transferSFTPTab(tabID)
	if tab == nil || ui.transfers == nil {
		return 0
	}
	count := 0
	for _, localPath := range localPaths {
		info, err := os.Stat(localPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		remotePath, ok := sftpChildPath(tab.sftpBrowser.Path, filepath.Base(localPath))
		if !ok {
			continue
		}
		if ui.transfers.enqueue(transferUpload, tab.HostID, localPath, remotePath, info.Size()) != nil {
			count++
		}
	}
	if count > 0 {
		ui.transferPanelOpen = true
	}
	return count
}

func (ui *Window) enqueueSFTPDownloads(tabID string, targets []sftpDownloadTarget) int {
	tab := ui.transferSFTPTab(tabID)
	if tab == nil || ui.transfers == nil {
		return 0
	}
	entries := make(map[string]sftpEntry, len(tab.sftpBrowser.Entries))
	for _, entry := range tab.sftpBrowser.Entries {
		entries[entry.Path] = entry
	}
	count := 0
	for _, target := range targets {
		entry, ok := entries[cleanRemotePath(target.RemotePath)]
		if !ok || entry.Directory || filepath.Clean(target.LocalPath) == "." {
			continue
		}
		if ui.transfers.enqueue(transferDownload, tab.HostID, entry.Path, target.LocalPath, entry.Size) != nil {
			count++
		}
	}
	if count > 0 {
		ui.transferPanelOpen = true
	}
	return count
}

func (ui *Window) transferSFTPTab(tabID string) *sshTab {
	if ui == nil {
		return nil
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.View != sshTabViewSFTP || tab.sftpBrowser == nil {
		return nil
	}
	return tab
}

func openLocalTransferReader(localPath string, offset int64) (io.ReadCloser, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func openLocalTransferWriter(localPath string, offset int64, truncate bool) (io.WriteCloser, error) {
	flags := os.O_CREATE | os.O_WRONLY
	if truncate {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(localPath, flags, 0o600)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func copySFTPTransfer(
	ctx context.Context,
	source io.Reader,
	destination io.Writer,
	size int64,
	transferred int64,
	onProgress func(int64),
) error {
	return copySFTPTransferWithDigest(ctx, source, destination, size, transferred, onProgress, nil)
}

func copySFTPTransferWithDigest(
	ctx context.Context,
	source io.Reader,
	destination io.Writer,
	size int64,
	transferred int64,
	onProgress func(int64),
	digest hash.Hash,
) error {
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		readBuffer := buffer
		if size >= 0 {
			remaining := size - transferred
			if remaining <= 0 {
				return nil
			}
			if int64(len(readBuffer)) > remaining {
				readBuffer = readBuffer[:remaining]
			}
		}

		read, readErr := source.Read(readBuffer)
		if read > 0 {
			written, writeErr := destination.Write(readBuffer[:read])
			if digest != nil && written > 0 {
				_, _ = digest.Write(readBuffer[:written])
			}
			transferred += int64(written)
			if onProgress != nil {
				onProgress(transferred)
			}
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				if size >= 0 && transferred < size {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			return readErr
		}
		if read == 0 {
			return io.ErrNoProgress
		}
	}
}
