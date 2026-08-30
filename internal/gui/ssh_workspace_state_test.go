package gui

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"s12ryt-ssh/internal/remote"
)

type testSSHCloser struct {
	closed bool
}

func (c *testSSHCloser) Close() error {
	c.closed = true
	return nil
}

func (c *testSSHCloser) Read([]byte) (int, error) { return 0, io.EOF }

func (c *testSSHCloser) Write(p []byte) (int, error) { return len(p), nil }

type testSSHWrites struct {
	writes []string
}

func (w *testSSHWrites) Close() error { return nil }

func (w *testSSHWrites) Read([]byte) (int, error) { return 0, io.EOF }

func (w *testSSHWrites) Write(p []byte) (int, error) {
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func testSSHHost(id, name string) remote.SSHHost {
	return remote.SSHHost{ID: id, Name: name, Host: name + ".example.com", Port: 22, Username: "deploy"}
}

func TestSSHTabStoreAllowsIndependentDuplicateHostConnections(t *testing.T) {
	var store sshTabStore
	host := testSSHHost("host-1", "web")
	first := store.open(host)
	second := store.open(host)

	if first.ID == second.ID {
		t.Fatal("duplicate host connections must receive different tab IDs")
	}
	if len(store.tabs) != 2 || store.activeID != second.ID {
		t.Fatalf("tabs = %d active = %q, want 2 and %q", len(store.tabs), store.activeID, second.ID)
	}
	if first.HostID != second.HostID || first.HostName != second.HostName {
		t.Fatalf("duplicate tabs lost host identity: first=%+v second=%+v", first, second)
	}

	first.Output = "first session output"
	second.Output = "second session output"
	if store.activate(first.ID) == false {
		t.Fatal("first tab should be selectable")
	}
	if store.active().ID != first.ID || store.active().Output != "first session output" {
		t.Fatalf("active first tab = %+v", store.active())
	}
	if store.get(second.ID).Output != "second session output" {
		t.Fatal("switching tabs must not share terminal output")
	}
}

func TestSSHTabStoreCloseOnlyRemovesRequestedTab(t *testing.T) {
	var store sshTabStore
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-2", "db"))
	store.activate(first.ID)

	closed := store.close(first.ID)
	if closed != first {
		t.Fatal("close must return the requested tab")
	}
	if store.get(first.ID) != nil || store.get(second.ID) == nil {
		t.Fatal("closing one tab must preserve the other tab")
	}
	if store.activeID != second.ID {
		t.Fatalf("active tab after close = %q, want %q", store.activeID, second.ID)
	}
	if store.close("missing") != nil {
		t.Fatal("closing a missing tab must be a no-op")
	}
}

func TestSSHTabStoreAllowsConcurrentLookupAndClose(t *testing.T) {
	var store sshTabStore
	tab := store.open(testSSHHost("host-1", "web"))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			_ = store.get(tab.ID)
		}
	}()
	go func() {
		defer wg.Done()
		store.close(tab.ID)
	}()
	wg.Wait()
}

func TestSSHTabStoreRetainsFailedTabForRetry(t *testing.T) {
	var store sshTabStore
	tab := store.open(testSSHHost("host-1", "web"))

	failure := errors.New("connection refused")
	if !store.fail(tab.ID, failure) {
		t.Fatal("fail must update an existing tab")
	}
	if got := store.get(tab.ID); got.State != sshTabError || got.Error != failure.Error() {
		t.Fatalf("failed tab = %+v", got)
	}
	if !store.retry(tab.ID) {
		t.Fatal("retry must reuse the failed tab")
	}
	if got := store.get(tab.ID); got.State != sshTabConnecting || got.Error != "" {
		t.Fatalf("retried tab = %+v", got)
	}
}

func TestSSHTabStoreEndsOnlyTheCurrentSession(t *testing.T) {
	var store sshTabStore
	tab := store.open(testSSHHost("host-1", "web"))
	stale := &sshTabSession{pty: &testSSHCloser{}}
	currentPTY := &testSSHCloser{}
	current := &sshTabSession{pty: currentPTY}
	tab.session = current
	tab.State = sshTabConnected

	if store.endSession(tab.ID, stale, io.EOF) {
		t.Fatal("a stale reader must not end the replacement session")
	}
	if tab.State != sshTabConnected || tab.session != current {
		t.Fatalf("tab changed after stale session ended: %+v", tab)
	}

	if !store.endSession(tab.ID, current, io.EOF) {
		t.Fatal("the current session ending must update its tab")
	}
	if tab.State != sshTabError || tab.Error != "SSH terminal closed." || tab.session != nil {
		t.Fatalf("ended tab = %+v", tab)
	}
	if !currentPTY.closed {
		t.Fatal("ending a session must release its PTY")
	}
}

func TestSSHFormDirtyOnlyChangesWhenValuesDiffer(t *testing.T) {
	original := sshFormValues{HostID: "host-1", Name: "web", Host: "web.example.com", Port: "22", User: "deploy"}
	if sshFormDirty(original, original) {
		t.Fatal("unchanged form must not be dirty")
	}
	changed := original
	changed.User = "root"
	if !sshFormDirty(original, changed) {
		t.Fatal("changed form must be dirty")
	}
}

func TestSSHWorkspaceUsesHostStripBelowDesktopWidth(t *testing.T) {
	if !useSSHHostStrip(899) {
		t.Fatal("narrow workspace width must use the host strip")
	}
	if useSSHHostStrip(900) {
		t.Fatal("wide workspace width must keep the sidebar")
	}
}

func TestSSHTabStatusTextUsesStableTranslationSources(t *testing.T) {
	if got := sshTabStatusSource(sshTabConnecting); got != "Connecting" {
		t.Fatalf("connecting status source = %q", got)
	}
	if got := sshTabStatusSource(sshTabConnected); got != "Connected" {
		t.Fatalf("connected status source = %q", got)
	}
	if got := sshTabStatusSource(sshTabError); got != "Connection failed" {
		t.Fatalf("error status source = %q", got)
	}
}

func TestSSHFormCloseOnlyNeedsConfirmationWhenDirty(t *testing.T) {
	if sshFormCloseNeedsConfirmation(false) {
		t.Fatal("clean form should close immediately")
	}
	if !sshFormCloseNeedsConfirmation(true) {
		t.Fatal("dirty form should require discard confirmation")
	}
}

func TestSSHTabCloseReleasesOnlyItsSessionResources(t *testing.T) {
	var store sshTabStore
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-2", "db"))
	firstPTY := &testSSHCloser{}
	firstClient := &testSSHCloser{}
	secondPTY := &testSSHCloser{}
	secondClient := &testSSHCloser{}
	firstContext, firstCancel := context.WithCancel(context.Background())
	secondContext, secondCancel := context.WithCancel(context.Background())
	first.session = &sshTabSession{pty: firstPTY, client: firstClient, ctx: firstContext, cancel: firstCancel}
	second.session = &sshTabSession{pty: secondPTY, client: secondClient, ctx: secondContext, cancel: secondCancel}

	store.close(first.ID)
	if !firstPTY.closed || !firstClient.closed {
		t.Fatal("closing a tab must close its PTY and SSH client")
	}
	select {
	case <-firstContext.Done():
	default:
		t.Fatal("closing a tab must cancel its session context")
	}
	if secondPTY.closed || secondClient.closed {
		t.Fatal("closing one tab must not close another tab's resources")
	}
	select {
	case <-secondContext.Done():
		t.Fatal("closing one tab must not cancel another tab's context")
	default:
	}
}

func TestSSHTabsKeepIndependentInputEditors(t *testing.T) {
	var store sshTabStore
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-1", "web"))
	first.input.SetText("first command")
	second.input.SetText("second command")

	if first.input.Text() != "first command" || second.input.Text() != "second command" {
		t.Fatalf("tab input state = %q / %q", first.input.Text(), second.input.Text())
	}
}

func TestSendSSHTabInputWritesOnlyToRequestedTab(t *testing.T) {
	ui := NewWindow(nil)
	first := ui.sshTabs.open(testSSHHost("host-1", "web"))
	second := ui.sshTabs.open(testSSHHost("host-1", "web"))
	firstPTY := &testSSHWrites{}
	secondPTY := &testSSHWrites{}
	first.session = &sshTabSession{pty: firstPTY}
	second.session = &sshTabSession{pty: secondPTY}
	first.State = sshTabConnected
	second.State = sshTabConnected
	first.input.SetText("first command")
	second.input.SetText("second command")

	if !ui.sendSSHTabInput(first.ID) {
		t.Fatal("sending input to a connected tab should succeed")
	}
	if len(firstPTY.writes) != 1 || firstPTY.writes[0] != "first command\n" {
		t.Fatalf("first PTY writes = %q", firstPTY.writes)
	}
	if first.input.Text() != "" || second.input.Text() != "second command" {
		t.Fatalf("tab inputs after send = %q / %q", first.input.Text(), second.input.Text())
	}
	if len(secondPTY.writes) != 0 {
		t.Fatalf("second PTY writes = %q", secondPTY.writes)
	}
}
