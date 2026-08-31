package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

// sshTabState describes the user-visible lifecycle of one terminal tab.
type sshTabState string

type sshTabView string

const (
	sshTabConnecting sshTabState = "connecting"
	sshTabConnected  sshTabState = "connected"
	sshTabError      sshTabState = "error"
	sshTabClosed     sshTabState = "closed"
)

const (
	sshTabViewTerminal sshTabView = "terminal"
	sshTabViewSFTP     sshTabView = "sftp"
)

// sshTab is the state boundary for one independent SSH/PTY session. Gio
// widgets and live resources are kept here so switching tabs never shares
// input, output, or cancellation state with another connection.
type sshTab struct {
	ID                   string
	HostID               string
	HostName             string
	Endpoint             string
	Local                bool
	Title                string
	Pinned               bool
	View                 sshTabView
	State                sshTabState
	Error                string
	Output               string
	input                widget.Editor
	outputList           layout.List
	tabButton            widget.Clickable
	closeButton          widget.Clickable
	retryButton          widget.Clickable
	sendButton           widget.Clickable
	copyButton           widget.Clickable
	pasteButton          widget.Clickable
	terminalViewButton   widget.Clickable
	sftpViewButton       widget.Clickable
	sftpParentButton     widget.Clickable
	sftpRefreshButton    widget.Clickable
	sftpActionList       layout.List
	sftpActionButtons    []widget.Clickable
	sftpList             layout.List
	dragTag              int
	terminalTag          int
	clipboardTag         int
	sftpDropTag          int
	selection            terminalSelection
	selectionAnchor      image.Point
	selectionPointerID   uint16
	selecting            bool
	size                 image.Point
	emulator             terminalEmulator
	sftpBrowser          *sftpBrowserState
	sftpLoading          bool
	sftpError            string
	sftpInfo             string
	sftpSelectionWidgets []widget.Bool
	sftpOpenButtons      []widget.Clickable
	sftpEntryPaths       []string
	outputMu             sync.RWMutex
	session              *sshTabSession
	history              *sshSessionHistoryTracker
	historyAttempt       *sshSessionHistoryAttempt
}

type terminalSelection struct {
	active bool
	start  image.Point
	end    image.Point
}

func (tab *sshTab) setTerminalSelection(start, end image.Point) {
	if tab == nil {
		return
	}
	tab.selection = terminalSelection{active: true, start: start, end: end}
}

func (tab *sshTab) clearTerminalSelection() {
	if tab == nil {
		return
	}
	tab.selection = terminalSelection{}
}

func (tab *sshTab) selectedTerminalText() string {
	if tab == nil || !tab.selection.active || tab.emulator == nil {
		return ""
	}
	return terminalSelectionText(tab.emulator.Frame(), tab.selection.start, tab.selection.end)
}

func (tab *sshTab) syncSFTPEntryWidgets() {
	if tab == nil || tab.sftpBrowser == nil {
		return
	}
	entries := tab.sftpBrowser.Entries
	paths := make([]string, len(entries))
	pathsMatch := len(paths) == len(tab.sftpEntryPaths)
	for index, entry := range entries {
		paths[index] = entry.Path
		if pathsMatch && tab.sftpEntryPaths[index] != entry.Path {
			pathsMatch = false
		}
	}
	if pathsMatch {
		return
	}

	tab.sftpSelectionWidgets = make([]widget.Bool, len(entries))
	tab.sftpOpenButtons = make([]widget.Clickable, len(entries))
	tab.sftpEntryPaths = paths
	for index, entry := range entries {
		tab.sftpSelectionWidgets[index].Value = tab.sftpBrowser.selections[entry.Path]
	}
}

type sshTabSession struct {
	pty       ptyTerminal
	sftp      sshclient.SFTPClient
	client    interface{ Close() error }
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func (s *sshTabSession) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.sftp != nil {
			_ = s.sftp.Close()
		}
		if s.pty != nil {
			_ = s.pty.Close()
		}
		if s.client != nil {
			_ = s.client.Close()
		}
	})
}

// sshTabStore owns tab ordering and selection. It deliberately has no network
// or widget concerns, which makes the lifecycle rules deterministic to test.
type sshTabStore struct {
	mu       sync.RWMutex
	tabs     []*sshTab
	activeID string
	nextID   int
}

type sshTabDragState struct {
	active     bool
	tabID      string
	startIndex int
	startX     float32
	pointerID  uint16
}

func (s *sshTabDragState) reset() {
	*s = sshTabDragState{}
}

func (s *sshTabStore) open(host remote.SSHHost) *sshTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	tab := &sshTab{
		ID:       fmt.Sprintf("ssh-tab-%d", s.nextID),
		HostID:   host.ID,
		HostName: host.Name,
		Endpoint: fmt.Sprintf("%s:%d", host.Host, host.Port),
		State:    sshTabConnecting,
		View:     sshTabViewTerminal,
		emulator: newTerminalEmulator(100, 30),
		history:  newSSHSessionHistoryTracker(),
	}
	s.tabs = append(s.tabs, tab)
	s.activeID = tab.ID
	return tab
}

func (s *sshTabStore) openLocal(name string) *sshTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	tab := &sshTab{
		ID:       fmt.Sprintf("local-tab-%d", s.nextID),
		HostName: name,
		Endpoint: "Local shell",
		Local:    true,
		State:    sshTabConnecting,
		View:     sshTabViewTerminal,
		emulator: newTerminalEmulator(100, 30),
		history:  newSSHSessionHistoryTracker(),
	}
	s.tabs = append(s.tabs, tab)
	s.activeID = tab.ID
	return tab
}

func (s *sshTabStore) duplicate(id string) *sshTab {
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.getUnlocked(id)
	if source == nil {
		return nil
	}
	s.nextID++
	tabID := "ssh-tab-"
	if source.Local {
		tabID = "local-tab-"
	}
	duplicate := &sshTab{
		ID:       fmt.Sprintf("%s%d", tabID, s.nextID),
		HostID:   source.HostID,
		HostName: source.HostName,
		Endpoint: source.Endpoint,
		Local:    source.Local,
		State:    sshTabConnecting,
		View:     sshTabViewTerminal,
		emulator: newTerminalEmulator(100, 30),
		history:  newSSHSessionHistoryTracker(),
	}
	s.tabs = append(s.tabs, duplicate)
	s.activeID = duplicate.ID
	return duplicate
}

func (s *sshTabStore) setView(id string, view sshTabView) bool {
	if view != sshTabViewTerminal && view != sshTabViewSFTP {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tab := s.getUnlocked(id)
	if tab == nil || (tab.Local && view == sshTabViewSFTP) {
		return false
	}
	tab.View = view
	return true
}

func (s *sshTabStore) rename(id, title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tab := s.getUnlocked(id)
	if tab == nil {
		return false
	}
	tab.Title = title
	return true
}

func (s *sshTabStore) setPinned(id string, pinned bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tabIndex := -1
	for index, tab := range s.tabs {
		if tab.ID == id {
			tabIndex = index
			break
		}
	}
	if tabIndex < 0 {
		return false
	}
	tab := s.tabs[tabIndex]
	tab.Pinned = pinned
	if !pinned || tabIndex == 0 {
		return true
	}
	s.tabs = append(s.tabs[:tabIndex], s.tabs[tabIndex+1:]...)
	s.tabs = append(s.tabs, nil)
	copy(s.tabs[1:], s.tabs[:len(s.tabs)-1])
	s.tabs[0] = tab
	return true
}

func (s *sshTabStore) move(id string, target int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if target < 0 || target >= len(s.tabs) {
		return false
	}
	from := -1
	for index, tab := range s.tabs {
		if tab.ID == id {
			from = index
			break
		}
	}
	if from < 0 {
		return false
	}
	if from == target {
		return true
	}
	tab := s.tabs[from]
	if from < target {
		copy(s.tabs[from:target], s.tabs[from+1:target+1])
	} else {
		copy(s.tabs[target+1:from+1], s.tabs[target:from])
	}
	s.tabs[target] = tab
	return true
}

func (s *sshTabStore) get(id string) *sshTab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUnlocked(id)
}

func (s *sshTabStore) getUnlocked(id string) *sshTab {
	for _, tab := range s.tabs {
		if tab.ID == id {
			return tab
		}
	}
	return nil
}

func (s *sshTabStore) active() *sshTab {
	return s.get(s.activeID)
}

func (s *sshTabStore) activate(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getUnlocked(id) == nil {
		return false
	}
	s.activeID = id
	return true
}

func (s *sshTabStore) close(id string) *sshTab {
	s.mu.Lock()
	for index, tab := range s.tabs {
		if tab.ID != id {
			continue
		}
		s.tabs = append(s.tabs[:index], s.tabs[index+1:]...)
		tab.State = sshTabClosed
		if s.activeID == id {
			if len(s.tabs) == 0 {
				s.activeID = ""
			} else if index < len(s.tabs) {
				s.activeID = s.tabs[index].ID
			} else {
				s.activeID = s.tabs[len(s.tabs)-1].ID
			}
		}
		s.mu.Unlock()
		tab.session.close()
		return tab
	}
	s.mu.Unlock()
	return nil
}

func (s *sshTabStore) closeHost(hostID string) []*sshTab {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return nil
	}
	s.mu.Lock()
	kept := make([]*sshTab, 0, len(s.tabs))
	closed := make([]*sshTab, 0)
	for _, tab := range s.tabs {
		if tab.Local || tab.HostID != hostID {
			kept = append(kept, tab)
			continue
		}
		tab.State = sshTabClosed
		closed = append(closed, tab)
	}
	s.tabs = kept
	if s.getUnlocked(s.activeID) == nil {
		if len(s.tabs) == 0 {
			s.activeID = ""
		} else {
			s.activeID = s.tabs[len(s.tabs)-1].ID
		}
	}
	s.mu.Unlock()
	for _, tab := range closed {
		tab.session.close()
	}
	return closed
}

func (s *sshTabStore) closeOthers(id string) []*sshTab {
	s.mu.Lock()
	keep := s.getUnlocked(id)
	if keep == nil {
		s.mu.Unlock()
		return nil
	}
	closed := make([]*sshTab, 0, len(s.tabs)-1)
	for _, tab := range s.tabs {
		if tab == keep {
			continue
		}
		tab.State = sshTabClosed
		closed = append(closed, tab)
	}
	s.tabs = []*sshTab{keep}
	s.activeID = keep.ID
	s.mu.Unlock()
	for _, tab := range closed {
		tab.session.close()
	}
	return closed
}

func (s *sshTabStore) closeAll() []*sshTab {
	s.mu.Lock()
	closed := s.tabs
	for _, tab := range closed {
		tab.State = sshTabClosed
	}
	s.tabs = nil
	s.activeID = ""
	s.mu.Unlock()
	for _, tab := range closed {
		tab.session.close()
	}
	return closed
}

func (s *sshTabStore) fail(id string, err error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tab := s.getUnlocked(id)
	if tab == nil {
		return false
	}
	tab.State = sshTabError
	if err == nil {
		tab.Error = "SSH connection failed"
	} else {
		tab.Error = err.Error()
	}
	return true
}

func (s *sshTabStore) retry(id string) bool {
	s.mu.Lock()
	tab := s.getUnlocked(id)
	if tab == nil || tab.State != sshTabError {
		s.mu.Unlock()
		return false
	}
	session := tab.session
	tab.session = nil
	tab.State = sshTabConnecting
	tab.Error = ""
	s.activeID = id
	s.mu.Unlock()
	session.close()
	return true
}

func (s *sshTabStore) reconnect(id string) bool {
	s.mu.Lock()
	tab := s.getUnlocked(id)
	if tab == nil {
		s.mu.Unlock()
		return false
	}
	session := tab.session
	tab.session = nil
	tab.State = sshTabConnecting
	tab.Error = ""
	s.activeID = id
	s.mu.Unlock()
	session.close()
	return true
}

func (s *sshTabStore) endSession(id string, session *sshTabSession, err error) bool {
	s.mu.Lock()
	tab := s.getUnlocked(id)
	if tab == nil || tab.session != session {
		s.mu.Unlock()
		return false
	}
	tab.session = nil
	tab.State = sshTabError
	if err == nil || errors.Is(err, io.EOF) {
		tab.Error = "SSH terminal closed."
	} else {
		tab.Error = err.Error()
	}
	s.mu.Unlock()
	session.close()
	return true
}

func (t *sshTab) appendOutput(incoming string) {
	if t == nil {
		return
	}
	t.outputMu.Lock()
	t.Output = appendTerminalFilter(t.Output, incoming, terminalMaxRunes)
	if t.emulator != nil {
		_ = t.emulator.Feed([]byte(incoming))
	}
	t.outputMu.Unlock()
}

func (t *sshTab) outputSnapshot() string {
	if t == nil {
		return ""
	}
	t.outputMu.RLock()
	defer t.outputMu.RUnlock()
	return t.Output
}

// sshFormValues is the non-widget snapshot used to decide whether a modal
// contains unsaved changes.
type sshFormValues struct {
	HostID      string
	Name        string
	Host        string
	Port        string
	User        string
	Password    string
	PrivateKey  string
	KeyPass     string
	Fingerprint string
}

func sshFormDirty(original, current sshFormValues) bool {
	return original != current
}

func sshFormCloseNeedsConfirmation(dirty bool) bool {
	return dirty
}

func sshTabStatusSource(state sshTabState) string {
	switch state {
	case sshTabConnected:
		return "Connected"
	case sshTabError:
		return "Connection failed"
	default:
		return "Connecting"
	}
}

func sshTabDisplayName(tab *sshTab) string {
	if tab == nil {
		return ""
	}
	if tab.Title != "" {
		return tab.Title
	}
	return tab.HostName
}

func sshTabActionSources() []string {
	return []string{
		"Duplicate",
		"Reconnect",
		"Rename",
		"Pin",
		"Close others",
		"Close all",
	}
}

func sshTabDragTarget(startIndex int, deltaX, itemExtent float32, count int) int {
	if count < 1 {
		return -1
	}
	if startIndex < 0 {
		startIndex = 0
	} else if startIndex >= count {
		startIndex = count - 1
	}
	if itemExtent <= 0 {
		return startIndex
	}
	target := startIndex + int(math.Round(float64(deltaX/itemExtent)))
	if target < 0 {
		return 0
	}
	if target >= count {
		return count - 1
	}
	return target
}

const sshHostStripBelowDp = 900

func useSSHHostStrip(widthDp int) bool {
	return widthDp < sshHostStripBelowDp
}
