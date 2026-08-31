package gui

import (
	"context"
	"errors"
	"image"
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

func TestSSHTabStoreDuplicateStartsFreshSession(t *testing.T) {
	var store sshTabStore
	source := store.open(testSSHHost("host-1", "web"))
	source.State = sshTabConnected
	source.Output = "existing output"
	source.input.SetText("existing input")
	source.Title = "production"

	duplicate := store.duplicate(source.ID)
	if duplicate == nil || duplicate.ID == source.ID {
		t.Fatal("duplicate must create a new tab")
	}
	if duplicate.HostID != source.HostID || duplicate.HostName != source.HostName || duplicate.Endpoint != source.Endpoint {
		t.Fatalf("duplicate lost host identity: %+v", duplicate)
	}
	if duplicate.State != sshTabConnecting || duplicate.Output != "" || duplicate.input.Text() != "" {
		t.Fatalf("duplicate copied live session state: %+v output=%q input=%q", duplicate, duplicate.Output, duplicate.input.Text())
	}
	if duplicate.Title != "" || duplicate.Pinned {
		t.Fatalf("duplicate copied session-only tab decorations: %+v", duplicate)
	}
}

func TestSSHTabStoreSupportsRenamePinAndReorder(t *testing.T) {
	var store sshTabStore
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-2", "db"))
	third := store.open(testSSHHost("host-3", "cache"))

	if !store.rename(second.ID, "database") || second.Title != "database" {
		t.Fatalf("rename result = %q", second.Title)
	}
	if !store.setPinned(third.ID, true) || !third.Pinned {
		t.Fatal("pinning a tab must update its pinned state")
	}
	if store.tabs[0] != third {
		t.Fatalf("pinned tab order = [%s, ...], want %s first", store.tabs[0].ID, third.ID)
	}
	if !store.move(third.ID, len(store.tabs)-1) {
		t.Fatal("tab should be movable by order index")
	}
	if store.tabs[len(store.tabs)-1] != third {
		t.Fatalf("tab order after move does not place %s last", third.ID)
	}
	if store.tabs[0] != first || store.tabs[1] != second {
		t.Fatalf("tab order after move = %s, %s, %s", store.tabs[0].ID, store.tabs[1].ID, store.tabs[2].ID)
	}
}

func TestSSHTabStoreCloseOthersAndAllReleaseSessions(t *testing.T) {
	var store sshTabStore
	keep := store.open(testSSHHost("host-1", "web"))
	other := store.open(testSSHHost("host-2", "db"))
	last := store.open(testSSHHost("host-3", "cache"))
	otherPTY := &testSSHCloser{}
	lastPTY := &testSSHCloser{}
	other.session = &sshTabSession{pty: otherPTY}
	last.session = &sshTabSession{pty: lastPTY}

	closed := store.closeOthers(keep.ID)
	if len(closed) != 2 || len(store.tabs) != 1 || store.activeID != keep.ID {
		t.Fatalf("close others = closed %d tabs, remaining %d, active %q", len(closed), len(store.tabs), store.activeID)
	}
	if !otherPTY.closed || !lastPTY.closed {
		t.Fatal("close others must release every removed session")
	}

	keepPTY := &testSSHCloser{}
	keep.session = &sshTabSession{pty: keepPTY}
	all := store.closeAll()
	if len(all) != 1 || len(store.tabs) != 0 || store.activeID != "" || !keepPTY.closed {
		t.Fatalf("close all = %+v, remaining %d, active %q, closed=%v", all, len(store.tabs), store.activeID, keepPTY.closed)
	}
}

func TestSSHTabStoreReconnectReusesTabAndReleasesOldSession(t *testing.T) {
	var store sshTabStore
	tab := store.open(testSSHHost("host-1", "web"))
	oldPTY := &testSSHCloser{}
	tab.session = &sshTabSession{pty: oldPTY}
	tab.State = sshTabConnected
	tab.Error = "stale error"

	if !store.reconnect(tab.ID) {
		t.Fatal("connected tab should support reconnect")
	}
	if tab.State != sshTabConnecting || tab.Error != "" || tab.session != nil || store.activeID != tab.ID {
		t.Fatalf("reconnected tab = %+v", tab)
	}
	if !oldPTY.closed {
		t.Fatal("reconnect must release the old PTY")
	}
}

func TestSSHTabStoreSwitchesRemoteTabsBetweenTerminalAndSFTP(t *testing.T) {
	store := sshTabStore{}
	remoteTab := store.open(testSSHHost("host-1", "Remote"))
	localTab := store.openLocal("Local terminal")

	if remoteTab.View != sshTabViewTerminal || localTab.View != sshTabViewTerminal {
		t.Fatal("new tabs must start in the terminal view")
	}
	if !store.setView(remoteTab.ID, sshTabViewSFTP) || remoteTab.View != sshTabViewSFTP {
		t.Fatal("remote tab should switch to SFTP")
	}
	if store.setView(localTab.ID, sshTabViewSFTP) || localTab.View != sshTabViewTerminal {
		t.Fatal("local shell tabs must reject SFTP")
	}
}

func TestSSHTabSessionClosesItsSFTPResource(t *testing.T) {
	sftp := &testSFTPClient{}
	pty := &testSSHCloser{}
	session := &sshTabSession{pty: pty, sftp: sftp}

	session.close()
	session.close()

	if sftp.closed != 1 || !pty.closed {
		t.Fatalf("session close = SFTP %d, PTY closed %v", sftp.closed, pty.closed)
	}
}

func TestSSHTabDisplayNamePrefersSessionRename(t *testing.T) {
	tab := &sshTab{HostName: "web"}
	if got := sshTabDisplayName(tab); got != "web" {
		t.Fatalf("default tab title = %q, want web", got)
	}
	tab.Title = "release shell"
	if got := sshTabDisplayName(tab); got != "release shell" {
		t.Fatalf("renamed tab title = %q, want release shell", got)
	}
}

func TestSSHTabActionSourcesExposeEveryTabOperation(t *testing.T) {
	want := []string{
		"Duplicate",
		"Reconnect",
		"Rename",
		"Pin",
		"Close others",
		"Close all",
	}
	got := sshTabActionSources()
	if len(got) != len(want) {
		t.Fatalf("tab action sources = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("tab action source %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestSSHTabDragTargetCrossesItemCentersAndClampsToBounds(t *testing.T) {
	tests := []struct {
		name       string
		startIndex int
		deltaX     float32
		itemExtent float32
		count      int
		want       int
	}{
		{name: "small movement stays put", startIndex: 2, deltaX: 39, itemExtent: 100, count: 5, want: 2},
		{name: "half item moves right", startIndex: 2, deltaX: 50, itemExtent: 100, count: 5, want: 3},
		{name: "half item moves left", startIndex: 2, deltaX: -50, itemExtent: 100, count: 5, want: 1},
		{name: "multiple items move right", startIndex: 1, deltaX: 240, itemExtent: 100, count: 5, want: 3},
		{name: "left edge clamps", startIndex: 1, deltaX: -500, itemExtent: 100, count: 5, want: 0},
		{name: "right edge clamps", startIndex: 3, deltaX: 500, itemExtent: 100, count: 5, want: 4},
		{name: "invalid extent stays put", startIndex: 2, deltaX: 500, itemExtent: 0, count: 5, want: 2},
		{name: "empty list has no target", startIndex: 0, deltaX: 500, itemExtent: 100, count: 0, want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sshTabDragTarget(test.startIndex, test.deltaX, test.itemExtent, test.count); got != test.want {
				t.Fatalf("drag target = %d, want %d", got, test.want)
			}
		})
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

func TestPasteSSHTabTextRequiresConfirmationForMultipleLines(t *testing.T) {
	ui := NewWindow(nil)
	first := ui.sshTabs.open(testSSHHost("host-1", "web"))
	second := ui.sshTabs.open(testSSHHost("host-2", "db"))
	firstPTY := &testSSHWrites{}
	secondPTY := &testSSHWrites{}
	first.session = &sshTabSession{pty: firstPTY}
	second.session = &sshTabSession{pty: secondPTY}
	first.State = sshTabConnected
	second.State = sshTabConnected

	if !ui.pasteSSHTabText(first.ID, "pwd") {
		t.Fatal("single-line terminal paste should be handled")
	}
	if len(firstPTY.writes) != 1 || firstPTY.writes[0] != "pwd" {
		t.Fatalf("single-line PTY writes = %q, want pwd", firstPTY.writes)
	}
	if len(secondPTY.writes) != 0 {
		t.Fatalf("second PTY writes = %q, want none", secondPTY.writes)
	}

	if !ui.pasteSSHTabText(first.ID, "pwd\r\nwhoami") {
		t.Fatal("multi-line terminal paste should open confirmation")
	}
	if !ui.confirm.active {
		t.Fatal("multi-line paste must require confirmation")
	}
	if len(firstPTY.writes) != 1 {
		t.Fatalf("multi-line paste wrote before confirmation: %q", firstPTY.writes)
	}

	ui.confirm.accept()
	if len(firstPTY.writes) != 2 || firstPTY.writes[1] != "pwd\rwhoami" {
		t.Fatalf("confirmed multi-line PTY writes = %q", firstPTY.writes)
	}
}

func TestSSHTabsKeepIndependentTerminalSelections(t *testing.T) {
	var store sshTabStore
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-2", "db"))
	if err := first.emulator.Feed([]byte("hello\r\nworld")); err != nil {
		t.Fatalf("feed first terminal: %v", err)
	}
	if err := second.emulator.Feed([]byte("private")); err != nil {
		t.Fatalf("feed second terminal: %v", err)
	}

	first.setTerminalSelection(image.Pt(1, 0), image.Pt(3, 1))
	if got := first.selectedTerminalText(); got != "ello\nwor" {
		t.Fatalf("first selected text = %q, want %q", got, "ello\nwor")
	}
	if got := second.selectedTerminalText(); got != "" {
		t.Fatalf("second selected text = %q, want empty", got)
	}

	first.clearTerminalSelection()
	if got := first.selectedTerminalText(); got != "" {
		t.Fatalf("cleared selected text = %q, want empty", got)
	}
}
