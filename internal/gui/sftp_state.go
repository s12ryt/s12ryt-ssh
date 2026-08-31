package gui

import (
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	sshclient "s12ryt-ssh/internal/ssh"
)

type sftpEntry struct {
	Name      string
	Path      string
	Size      int64
	Mode      os.FileMode
	Modified  time.Time
	Directory bool
	Symlink   bool
}

func sftpActionSources() []string {
	return []string{
		"Upload files",
		"Download selected",
		"New folder",
		"Rename item",
		"Delete selected",
		"File information",
		"Create symbolic link",
	}
}

func sftpActionSelectionValid(action string, selected int) bool {
	if selected < 0 {
		return false
	}
	switch action {
	case "Rename item", "File information":
		return selected == 1
	case "Download selected", "Delete selected":
		return selected > 0
	default:
		return true
	}
}

type sftpOperationSpec struct {
	fieldSources []string
	submitSource string
}

func sftpOperationDialogSpec(action string) (sftpOperationSpec, bool) {
	switch action {
	case "New folder":
		return sftpOperationSpec{fieldSources: []string{"Folder name"}, submitSource: "Create"}, true
	case "Rename item":
		return sftpOperationSpec{fieldSources: []string{"New name"}, submitSource: "Save name"}, true
	case "Create symbolic link":
		return sftpOperationSpec{fieldSources: []string{"Target path", "Link name"}, submitSource: "Create"}, true
	default:
		return sftpOperationSpec{}, false
	}
}

func sftpInfoLines(info string) []string {
	info = strings.TrimSpace(strings.ReplaceAll(info, "\r\n", "\n"))
	if info == "" {
		return nil
	}
	return strings.Split(info, "\n")
}

func sftpEntriesFromTransport(entries []sshclient.SFTPEntry) []sftpEntry {
	mapped := make([]sftpEntry, 0, len(entries))
	for _, entry := range entries {
		mapped = append(mapped, sftpEntry{
			Name:      entry.Name,
			Path:      entry.Path,
			Size:      entry.Size,
			Mode:      entry.Mode,
			Modified:  entry.Modified,
			Directory: entry.Directory,
			Symlink:   entry.Symlink,
		})
	}
	return mapped
}

type sftpBrowserState struct {
	Path       string
	Entries    []sftpEntry
	selections map[string]bool
}

func newSFTPBrowserState(remotePath string) *sftpBrowserState {
	return &sftpBrowserState{
		Path:       cleanRemotePath(remotePath),
		selections: make(map[string]bool),
	}
}

func cleanRemotePath(remotePath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(remotePath))
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	return cleaned
}

func sftpChildPath(parent, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return "", false
	}
	return path.Join(cleanRemotePath(parent), name), true
}

func (browser *sftpBrowserState) applyEntries(entries []sftpEntry) {
	if browser == nil {
		return
	}
	browser.Path = cleanRemotePath(browser.Path)
	browser.Entries = append(browser.Entries[:0], entries...)
	for index := range browser.Entries {
		browser.Entries[index].Name = strings.TrimSpace(browser.Entries[index].Name)
		browser.Entries[index].Path = cleanRemotePath(browser.Entries[index].Path)
	}
	sort.SliceStable(browser.Entries, func(left, right int) bool {
		if browser.Entries[left].Directory != browser.Entries[right].Directory {
			return browser.Entries[left].Directory
		}
		return strings.ToLower(browser.Entries[left].Name) < strings.ToLower(browser.Entries[right].Name)
	})
	available := make(map[string]bool, len(browser.Entries))
	for _, entry := range browser.Entries {
		available[entry.Path] = true
	}
	for selected := range browser.selections {
		if !available[selected] {
			delete(browser.selections, selected)
		}
	}
}

func (browser *sftpBrowserState) enter(name string) bool {
	if browser == nil {
		return false
	}
	for _, entry := range browser.Entries {
		if entry.Name != name || !entry.Directory {
			continue
		}
		browser.Path = cleanRemotePath(entry.Path)
		browser.selections = make(map[string]bool)
		return true
	}
	return false
}

func (browser *sftpBrowserState) parent() bool {
	if browser == nil || browser.Path == "/" {
		return false
	}
	browser.Path = cleanRemotePath(path.Dir(browser.Path))
	browser.selections = make(map[string]bool)
	return true
}

func (browser *sftpBrowserState) toggleSelection(remotePath string) bool {
	if browser == nil {
		return false
	}
	remotePath = cleanRemotePath(remotePath)
	found := false
	for _, entry := range browser.Entries {
		if entry.Path == remotePath {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	if browser.selections[remotePath] {
		delete(browser.selections, remotePath)
	} else {
		browser.selections[remotePath] = true
	}
	return true
}

func (browser *sftpBrowserState) selectedPaths() []string {
	if browser == nil {
		return nil
	}
	selected := make([]string, 0, len(browser.selections))
	for _, entry := range browser.Entries {
		if browser.selections[entry.Path] {
			selected = append(selected, entry.Path)
		}
	}
	return selected
}

type transferDirection string

const (
	transferUpload   transferDirection = "upload"
	transferDownload transferDirection = "download"
)

type transferStatus string

const (
	transferQueued    transferStatus = "queued"
	transferRunning   transferStatus = "running"
	transferPaused    transferStatus = "paused"
	transferCompleted transferStatus = "completed"
	transferFailed    transferStatus = "failed"
)

type transferItem struct {
	ID                string
	Direction         transferDirection
	HostID            string
	Source            string
	Destination       string
	Size              int64
	Transferred       int64
	Status            transferStatus
	Error             string
	Attempts          int
	StartedAt         time.Time
	LastProgressAt    time.Time
	LastProgressBytes int64
	BytesPerSecond    float64
	ExpectedSHA256    string
}

type transferQueue struct {
	mu            sync.Mutex
	maxConcurrent int
	nextID        int
	items         []*transferItem
}

func newTransferQueue(maxConcurrent int) *transferQueue {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &transferQueue{maxConcurrent: maxConcurrent}
}

func (queue *transferQueue) enqueue(direction transferDirection, hostID, source, destination string, size int64) *transferItem {
	if queue == nil {
		return nil
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if size < 0 {
		size = 0
	}
	queue.nextID++
	now := time.Now()
	item := &transferItem{
		ID:             "transfer-" + strconv.Itoa(queue.nextID),
		Direction:      direction,
		HostID:         hostID,
		Source:         source,
		Destination:    destination,
		Size:           size,
		Status:         transferQueued,
		Attempts:       1,
		StartedAt:      now,
		LastProgressAt: now,
	}
	queue.items = append(queue.items, item)
	queue.scheduleUnlocked()
	return item
}

func (queue *transferQueue) pause(id string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || (item.Status != transferRunning && item.Status != transferQueued) {
		return false
	}
	item.Status = transferPaused
	queue.scheduleUnlocked()
	return true
}

func (queue *transferQueue) resume(id string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || item.Status != transferPaused {
		return false
	}
	item.Status = transferQueued
	queue.scheduleUnlocked()
	return true
}

func (queue *transferQueue) updateProgress(id string, transferred int64) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || item.Status != transferRunning {
		return false
	}
	return applyTransferProgress(item, transferred, time.Now())
}

func (queue *transferQueue) complete(id string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || item.Status != transferRunning {
		return false
	}
	item.Transferred = item.Size
	item.Status = transferCompleted
	item.Error = ""
	queue.scheduleUnlocked()
	return true
}

func (queue *transferQueue) fail(id, message string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || item.Status != transferRunning {
		return false
	}
	item.Status = transferFailed
	item.Error = strings.TrimSpace(message)
	queue.scheduleUnlocked()
	return true
}

func (queue *transferQueue) retry(id string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	item := queue.findUnlocked(id)
	if item == nil || item.Status != transferFailed {
		return false
	}
	item.Attempts++
	item.Error = ""
	item.Status = transferQueued
	item.BytesPerSecond = 0
	item.LastProgressAt = time.Now()
	item.LastProgressBytes = item.Transferred
	queue.scheduleUnlocked()
	return true
}

func (queue *transferQueue) disableHost(hostID, message string) []string {
	if queue == nil || strings.TrimSpace(hostID) == "" {
		return nil
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	failed := make([]string, 0)
	for _, item := range queue.items {
		if item.HostID != hostID || (item.Status != transferQueued && item.Status != transferRunning && item.Status != transferPaused) {
			continue
		}
		item.Status = transferFailed
		item.Error = strings.TrimSpace(message)
		failed = append(failed, item.ID)
	}
	queue.scheduleUnlocked()
	return failed
}

func (queue *transferQueue) findUnlocked(id string) *transferItem {
	for _, item := range queue.items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func (queue *transferQueue) scheduleUnlocked() {
	running := 0
	for _, item := range queue.items {
		if item.Status == transferRunning {
			running++
		}
	}
	for _, item := range queue.items {
		if running >= queue.maxConcurrent {
			return
		}
		if item.Status != transferQueued {
			continue
		}
		item.Status = transferRunning
		running++
	}
}
