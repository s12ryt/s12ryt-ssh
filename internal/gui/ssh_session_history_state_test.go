package gui

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

type sessionHistoryRemoteSession struct {
	*fakeRemoteSession
	history []remote.SSHSessionHistory
}

type lifecycleSessionHistoryRemoteSession struct {
	*fakeRemoteSession
	mu      sync.Mutex
	created []remote.SSHSessionHistoryInput
	updates []remote.SSHSessionHistoryUpdate
}

func (s *lifecycleSessionHistoryRemoteSession) SSHSessionHistory(context.Context) ([]remote.SSHSessionHistory, error) {
	return nil, nil
}

func (s *lifecycleSessionHistoryRemoteSession) CreateSSHSessionHistory(_ context.Context, input remote.SSHSessionHistoryInput) (remote.SSHSessionHistory, error) {
	s.mu.Lock()
	s.created = append(s.created, input)
	id := "history-1"
	s.mu.Unlock()
	return remote.SSHSessionHistory{ID: id, HostID: input.HostID, Status: input.Status}, nil
}

func (s *lifecycleSessionHistoryRemoteSession) UpdateSSHSessionHistory(_ context.Context, _ string, input remote.SSHSessionHistoryUpdate) (remote.SSHSessionHistory, error) {
	s.mu.Lock()
	s.updates = append(s.updates, input)
	s.mu.Unlock()
	return remote.SSHSessionHistory{Status: input.Status}, nil
}

func (s *lifecycleSessionHistoryRemoteSession) updateSnapshot() []remote.SSHSessionHistoryUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]remote.SSHSessionHistoryUpdate(nil), s.updates...)
}

func (s *sessionHistoryRemoteSession) SSHSessionHistory(context.Context) ([]remote.SSHSessionHistory, error) {
	return cloneSSHSessionHistory(s.history), nil
}

func testSSHSessionHistory(id, hostName string, status remote.SSHSessionHistoryStatus) remote.SSHSessionHistory {
	return remote.SSHSessionHistory{
		ID: id, HostID: "host-" + id, HostName: hostName, Status: status,
		LatencyMS: 42, StartedAt: 1700000000000,
	}
}

func TestSSHSessionHistoryStoreRefreshCopiesRecordsAndClearsRemoved(t *testing.T) {
	store := newSSHSessionHistoryStore()
	endedAt := int64(1700000001000)
	records := []remote.SSHSessionHistory{
		testSSHSessionHistory("one", "Production", remote.SSHSessionConnected),
		{
			ID: "two", HostID: "host-two", HostName: "Staging", Status: remote.SSHSessionClosed,
			StartedAt: 1699999999000, EndedAt: &endedAt,
		},
	}
	store.replace(records)
	records[0].HostName = "mutated"
	*records[1].EndedAt = 0

	snapshot := store.snapshot()
	if len(snapshot) != 2 || snapshot[0].HostName != "Production" {
		t.Fatalf("session history snapshot = %+v", snapshot)
	}
	if snapshot[1].EndedAt == nil || *snapshot[1].EndedAt != 1700000001000 {
		t.Fatalf("session history endedAt = %+v", snapshot[1].EndedAt)
	}
	store.replace(records[:1])
	if got := store.snapshot(); len(got) != 1 || got[0].ID != "one" {
		t.Fatalf("session history after replace = %+v", got)
	}
}

func TestFilterSSHSessionHistoryMatchesHostStatusAndError(t *testing.T) {
	records := []remote.SSHSessionHistory{
		testSSHSessionHistory("one", "Production", remote.SSHSessionConnected),
		testSSHSessionHistory("two", "Staging", remote.SSHSessionFailed),
	}
	records[1].ErrorMessage = "connection refused"

	for query, expectedID := range map[string]string{
		"production": "one",
		"FAILED":     "two",
		"REFUSED":    "two",
	} {
		filtered := filterSSHSessionHistory(records, query)
		if len(filtered) != 1 || filtered[0].ID != expectedID {
			t.Fatalf("filter %q = %+v", query, filtered)
		}
	}
	if got := filterSSHSessionHistory(records, ""); len(got) != 2 || got[0].ID != "one" || got[1].ID != "two" {
		t.Fatalf("empty session history filter = %+v", got)
	}
}

func TestSSHSessionHistoryStatusSourcesAreStable(t *testing.T) {
	tests := map[remote.SSHSessionHistoryStatus]string{
		remote.SSHSessionConnecting: "Connecting",
		remote.SSHSessionConnected:  "Connected",
		remote.SSHSessionFailed:     "Failed",
		remote.SSHSessionClosed:     "Closed",
	}
	for status, want := range tests {
		if got := sshSessionHistoryStatusSource(status); got != want {
			t.Fatalf("status source for %q = %q, want %q", status, got, want)
		}
	}
}

func TestSSHSessionHistoryTrackerRejectsStaleAttemptsAndFlushesLateCreation(t *testing.T) {
	tracker := newSSHSessionHistoryTracker()
	first := tracker.begin("host-one", "Production", 1000)
	if _, ok := tracker.markCreated(first, "history-one"); !ok {
		t.Fatal("first history attempt was not created")
	}

	second := tracker.begin("host-one", "Production", 2000)
	if _, ok := tracker.finish(first, remote.SSHSessionFailed, 10, "stale failure", 2010); ok {
		t.Fatal("stale history attempt updated the current attempt")
	}
	if _, ok := tracker.markCreated(second, "history-two"); !ok {
		t.Fatal("second history attempt was not created")
	}
	update, ok := tracker.finish(second, remote.SSHSessionConnected, 42, "", 2042)
	if !ok || update.ID != "history-two" || update.Status != remote.SSHSessionConnected || update.LatencyMS != 42 {
		t.Fatalf("current history update = %+v, ok %v", update, ok)
	}

	third := tracker.begin("host-one", "Production", 3000)
	if update, ok := tracker.finish(third, remote.SSHSessionClosed, 0, "", 0); ok {
		t.Fatalf("history update was emitted before the server created the record: %+v", update)
	}
	late, ok := tracker.markCreated(third, "history-three")
	if !ok || late.ID != "history-three" || late.Status != remote.SSHSessionClosed {
		t.Fatalf("late creation update = %+v, ok %v", late, ok)
	}
}

func TestSSHTabLifecycleWritesConnectedAndClosedHistory(t *testing.T) {
	ui := NewWindow(nil)
	session := &lifecycleSessionHistoryRemoteSession{fakeRemoteSession: &fakeRemoteSession{}}
	ui.model.SetRemoteSession(session, true)
	host := testSSHHost("host-one", "Production")
	tab := ui.sshTabs.open(host)

	if !ui.startSSHSessionHistory(tab) {
		t.Fatal("SSH session history did not start")
	}
	ui.attachSSHTab(tab.ID, nil, &testBlockingPTY{done: make(chan struct{})})
	if !waitForHistoryUpdateCount(session, 1, 2*time.Second) {
		t.Fatalf("history connected update did not arrive: %+v", session.updateSnapshot())
	}
	ui.closeSSHTab(tab.ID)
	if !waitForHistoryUpdateCount(session, 2, 2*time.Second) {
		t.Fatalf("history closed update did not arrive: %+v", session.updateSnapshot())
	}

	updates := session.updateSnapshot()
	if updates[0].Status != remote.SSHSessionConnected || updates[0].LatencyMS == nil || *updates[0].LatencyMS < 0 {
		t.Fatalf("connected history update = %+v", updates[0])
	}
	if updates[1].Status != remote.SSHSessionClosed || updates[1].ErrorMessage == nil || *updates[1].ErrorMessage != "" {
		t.Fatalf("closed history update = %+v", updates[1])
	}
}

type testBlockingPTY struct {
	done      chan struct{}
	closeOnce sync.Once
}

func (p *testBlockingPTY) Read([]byte) (int, error) {
	<-p.done
	return 0, io.EOF
}

func (p *testBlockingPTY) Write(data []byte) (int, error) { return len(data), nil }

func (p *testBlockingPTY) Close() error {
	p.closeOnce.Do(func() { close(p.done) })
	return nil
}

func waitForHistoryUpdateCount(session *lifecycleSessionHistoryRemoteSession, want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(session.updateSnapshot()) >= want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func TestSSHSessionHistoryDetailsExposeOnlyPersistedMetadata(t *testing.T) {
	endedAt := int64(1700000001000)
	record := remote.SSHSessionHistory{
		ID:           "history-one",
		HostID:       "host-one",
		HostName:     "Production",
		Status:       remote.SSHSessionFailed,
		LatencyMS:    42,
		ErrorMessage: "connection refused",
		StartedAt:    1700000000000,
		EndedAt:      &endedAt,
	}

	details := sshSessionHistoryDetails(record)
	wantSources := []string{"Started", "Latency", "Ended", "Error details"}
	if len(details) != len(wantSources) {
		t.Fatalf("session history details = %+v", details)
	}
	for index, source := range wantSources {
		if details[index].Source != source {
			t.Fatalf("detail %d source = %q, want %q", index, details[index].Source, source)
		}
	}
	if details[1].Value != "42 ms" {
		t.Fatalf("latency detail = %q, want 42 ms", details[1].Value)
	}
	if details[3].Value != "connection refused" || !details[3].Danger {
		t.Fatalf("error detail = %+v", details[3])
	}

	minimal := sshSessionHistoryDetails(remote.SSHSessionHistory{StartedAt: record.StartedAt})
	if len(minimal) != 1 || minimal[0].Source != "Started" {
		t.Fatalf("minimal session history details = %+v", minimal)
	}
}

func TestRefreshSSHSessionHistoryLoadsMetadataWithoutTerminalContent(t *testing.T) {
	ui := NewWindow(nil)
	session := &sessionHistoryRemoteSession{
		fakeRemoteSession: &fakeRemoteSession{},
		history: []remote.SSHSessionHistory{
			testSSHSessionHistory("one", "Production", remote.SSHSessionConnected),
		},
	}
	ui.model.SetRemoteSession(session, true)

	if !ui.refreshSSHSessionHistory() {
		t.Fatal("session history refresh did not start")
	}
	if !waitForSessionHistoryUIEvent(ui, 2*time.Second) {
		t.Fatal("session history refresh did not complete")
	}
	records := ui.sshHistory.snapshot()
	if len(records) != 1 || records[0].ID != "one" || records[0].HostName != "Production" {
		t.Fatalf("session history records = %+v", records)
	}
}

func TestClearSSHSessionHistoryViewDropsPreviousAccountMetadata(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshHistory.replace([]remote.SSHSessionHistory{
		testSSHSessionHistory("one", "Production", remote.SSHSessionConnected),
	})

	ui.clearSSHSessionHistoryView()

	if records := ui.sshHistory.snapshot(); len(records) != 0 {
		t.Fatalf("session history after clear = %+v", records)
	}
}

func waitForSessionHistoryUIEvent(ui *Window, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ui.pump()
		if !ui.busy {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
