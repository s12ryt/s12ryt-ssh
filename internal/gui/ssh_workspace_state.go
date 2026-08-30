package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
)

// sshTabState describes the user-visible lifecycle of one terminal tab.
type sshTabState string

const (
	sshTabConnecting sshTabState = "connecting"
	sshTabConnected  sshTabState = "connected"
	sshTabError      sshTabState = "error"
	sshTabClosed     sshTabState = "closed"
)

// sshTab is the state boundary for one independent SSH/PTY session. Gio
// widgets and live resources are kept here so switching tabs never shares
// input, output, or cancellation state with another connection.
type sshTab struct {
	ID          string
	HostID      string
	HostName    string
	Endpoint    string
	State       sshTabState
	Error       string
	Output      string
	input       widget.Editor
	outputList  layout.List
	tabButton   widget.Clickable
	closeButton widget.Clickable
	retryButton widget.Clickable
	sendButton  widget.Clickable
	size        image.Point
	outputMu    sync.RWMutex
	session     *sshTabSession
}

type sshTabSession struct {
	pty       ptyTerminal
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
	}
	s.tabs = append(s.tabs, tab)
	s.activeID = tab.ID
	return tab
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

const sshHostStripBelowDp = 900

func useSSHHostStrip(widthDp int) bool {
	return widthDp < sshHostStripBelowDp
}
