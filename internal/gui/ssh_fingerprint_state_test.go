package gui

import (
	"context"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

type fingerprintRemoteSession struct {
	*fakeRemoteSession
	history   map[string][]remote.SSHHostFingerprint
	setCalls  []fingerprintSetCall
	clearCall []string
}

type fingerprintSetCall struct {
	hostID      string
	fingerprint string
	source      remote.SSHHostFingerprintSource
}

func (s *fingerprintRemoteSession) SSHHostFingerprints(_ context.Context, hostID string) ([]remote.SSHHostFingerprint, error) {
	return append([]remote.SSHHostFingerprint(nil), s.history[hostID]...), nil
}

func (s *fingerprintRemoteSession) SetSSHHostFingerprintWithSource(_ context.Context, hostID, fingerprint string, source remote.SSHHostFingerprintSource) error {
	s.setCalls = append(s.setCalls, fingerprintSetCall{hostID: hostID, fingerprint: fingerprint, source: source})
	return nil
}

func (s *fingerprintRemoteSession) ClearSSHHostFingerprint(_ context.Context, hostID string) error {
	s.clearCall = append(s.clearCall, hostID)
	return nil
}

func testSSHHostFingerprintEntry(id, name, fingerprint string, active bool) sshHostFingerprintEntry {
	host := remote.SSHHost{ID: id, Name: name, Host: id + ".example.com", TrustedFingerprint: fingerprint}
	return sshHostFingerprintEntry{
		Host: host,
		History: []remote.SSHHostFingerprint{{
			ID: "fingerprint-" + id, HostID: id, Algorithm: "SHA256",
			Fingerprint: fingerprint, Source: remote.SSHHostFingerprintTOFU, Active: active,
		}},
	}
}

func TestSSHHostFingerprintStoreRefreshCopiesHistoryAndClearsRemoved(t *testing.T) {
	store := newSSHHostFingerprintStore()
	entries := []sshHostFingerprintEntry{
		testSSHHostFingerprintEntry("one", "Production", "SHA256:one", true),
		testSSHHostFingerprintEntry("two", "Staging", "SHA256:two", false),
	}
	store.replace(entries)
	entries[0].Host.Name = "mutated"
	entries[0].History[0].Fingerprint = "SHA256:mutated"

	snapshot := store.snapshot()
	if len(snapshot) != 2 || snapshot[0].Host.Name != "Production" ||
		snapshot[0].History[0].Fingerprint != "SHA256:one" {
		t.Fatalf("fingerprint snapshot = %+v", snapshot)
	}
	store.replace(entries[:1])
	if got := store.snapshot(); len(got) != 1 || got[0].Host.ID != "one" {
		t.Fatalf("fingerprint store after replace = %+v", got)
	}
}

func TestFilterSSHHostFingerprintsMatchesHostAddressAlgorithmAndDigest(t *testing.T) {
	entries := []sshHostFingerprintEntry{
		testSSHHostFingerprintEntry("one", "Production", "SHA256:one", true),
		testSSHHostFingerprintEntry("two", "Staging", "MD5:aa:bb", true),
	}
	entries[1].History[0].Algorithm = "MD5"
	entries[1].History[0].Fingerprint = "MD5:aa:bb"

	for query, expectedID := range map[string]string{
		"production":  "one",
		"TWO.EXAMPLE": "two",
		"md5":         "two",
		"SHA256:ONE":  "one",
	} {
		filtered := filterSSHHostFingerprintEntries(entries, query)
		if len(filtered) != 1 || filtered[0].Host.ID != expectedID {
			t.Fatalf("filter %q = %+v", query, filtered)
		}
	}
}

func TestValidateManualSSHHostFingerprintRequiresAlgorithmAndDigest(t *testing.T) {
	if got, err := validateManualSSHHostFingerprint(" sha256:abc123 "); err != nil || got != "SHA256:abc123" {
		t.Fatalf("valid manual fingerprint = %q, %v", got, err)
	}
	for _, invalid := range []string{"", "abc123", ":abc", "SHA256:"} {
		if _, err := validateManualSSHHostFingerprint(invalid); err == nil {
			t.Fatalf("manual fingerprint %q unexpectedly accepted", invalid)
		}
	}
}

func TestRefreshSSHHostFingerprintsLoadsHistoryForCurrentHosts(t *testing.T) {
	ui := NewWindow(nil)
	session := &fingerprintRemoteSession{
		fakeRemoteSession: &fakeRemoteSession{},
		history: map[string][]remote.SSHHostFingerprint{
			"host-1": {{ID: "fingerprint-1", HostID: "host-1", Algorithm: "SHA256", Fingerprint: "SHA256:first", Active: true}},
			"host-2": {{ID: "fingerprint-2", HostID: "host-2", Algorithm: "MD5", Fingerprint: "MD5:aa:bb", Active: false}},
		},
	}
	ui.model.SetRemoteSession(session, true)
	ui.sshHosts = []remote.SSHHost{
		{ID: "host-1", Name: "Production", Host: "one.example.com"},
		{ID: "host-2", Name: "Staging", Host: "two.example.com"},
	}

	if !ui.refreshSSHHostFingerprints() {
		t.Fatal("fingerprint refresh did not start")
	}
	if !waitForFingerprintUIEvent(ui, 2*time.Second) {
		t.Fatal("fingerprint refresh did not complete")
	}
	entries := ui.sshFingerprints.snapshot()
	if len(entries) != 2 || entries[0].Host.ID != "host-1" || entries[1].Host.ID != "host-2" {
		t.Fatalf("fingerprint entries = %+v", entries)
	}
	if len(entries[0].History) != 1 || entries[0].History[0].Fingerprint != "SHA256:first" {
		t.Fatalf("first fingerprint history = %+v", entries[0].History)
	}
}

func TestClearSSHHostFingerprintViewDropsPreviousAccountMetadata(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshFingerprints.replace([]sshHostFingerprintEntry{testSSHHostFingerprintEntry("one", "Production", "SHA256:one", true)})

	ui.clearSSHHostFingerprintView()

	if entries := ui.sshFingerprints.snapshot(); len(entries) != 0 {
		t.Fatalf("fingerprint entries after clear = %+v", entries)
	}
}

func TestTrustManualSSHHostFingerprintUsesManualSourceAndRefreshesHistory(t *testing.T) {
	ui := NewWindow(nil)
	session := &fingerprintRemoteSession{fakeRemoteSession: &fakeRemoteSession{}, history: map[string][]remote.SSHHostFingerprint{"host-1": {}}}
	ui.model.SetRemoteSession(session, true)
	ui.sshHosts = []remote.SSHHost{{ID: "host-1", Name: "Production", Host: "one.example.com"}}
	ui.sshFingerprints.replace([]sshHostFingerprintEntry{{Host: ui.sshHosts[0]}})

	if !ui.openManualSSHHostFingerprint("host-1") {
		t.Fatal("manual fingerprint dialog did not open")
	}
	ui.sshFingerprintManualEditor.SetText(" sha256:new-digest ")
	if !ui.submitManualSSHHostFingerprint() {
		t.Fatal("manual fingerprint submission did not start")
	}
	if !waitForFingerprintUIEvent(ui, 2*time.Second) {
		t.Fatal("manual fingerprint submission did not complete")
	}
	if len(session.setCalls) != 1 || session.setCalls[0] != (fingerprintSetCall{
		hostID: "host-1", fingerprint: "SHA256:new-digest", source: remote.SSHHostFingerprintManual,
	}) {
		t.Fatalf("manual fingerprint calls = %+v", session.setCalls)
	}
	if ui.sshFingerprintManualOpen {
		t.Fatal("manual fingerprint dialog remained open")
	}
}

func TestClearSSHHostFingerprintRequiresConfirmation(t *testing.T) {
	ui := NewWindow(nil)
	session := &fingerprintRemoteSession{fakeRemoteSession: &fakeRemoteSession{}, history: map[string][]remote.SSHHostFingerprint{"host-1": {}}}
	ui.model.SetRemoteSession(session, true)
	ui.sshHosts = []remote.SSHHost{{ID: "host-1", Name: "Production", Host: "one.example.com"}}
	ui.sshFingerprints.replace([]sshHostFingerprintEntry{testSSHHostFingerprintEntry("host-1", "Production", "SHA256:one", true)})

	if !ui.clearTrustedSSHHostFingerprint("host-1") {
		t.Fatal("clear fingerprint confirmation did not open")
	}
	if len(session.clearCall) != 0 || !ui.confirm.active {
		t.Fatalf("clear before confirmation = calls %v, confirmation %+v", session.clearCall, ui.confirm)
	}
	ui.confirm.accept()
	if !waitForFingerprintUIEvent(ui, 2*time.Second) {
		t.Fatal("clear fingerprint did not complete")
	}
	if len(session.clearCall) != 1 || session.clearCall[0] != "host-1" {
		t.Fatalf("clear fingerprint calls = %v", session.clearCall)
	}
}

func TestSyncSSHHostFingerprintButtonsFollowsVisibleHostsAndHistory(t *testing.T) {
	ui := NewWindow(nil)
	entries := []sshHostFingerprintEntry{
		testSSHHostFingerprintEntry("one", "Production", "SHA256:one", true),
		testSSHHostFingerprintEntry("two", "Staging", "MD5:aa:bb", false),
	}
	entries[0].History = append(entries[0].History, remote.SSHHostFingerprint{
		ID: "fingerprint-old", HostID: "one", Algorithm: "MD5", Fingerprint: "MD5:old", Active: false,
	})

	ui.syncSSHHostFingerprintButtons(entries)

	if got := ui.sshFingerprintVisibleIDs; len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("visible fingerprint host IDs = %v", got)
	}
	if len(ui.sshFingerprintManualBtns) != 2 || len(ui.sshFingerprintClearBtns) != 2 {
		t.Fatalf("host action buttons = manual:%d clear:%d", len(ui.sshFingerprintManualBtns), len(ui.sshFingerprintClearBtns))
	}
	if len(ui.sshFingerprintCopyBtns) != 3 || len(ui.sshFingerprintCopyValues) != 3 || ui.sshFingerprintCopyValues[1] != "MD5:old" {
		t.Fatalf("copy actions = buttons:%d values:%v", len(ui.sshFingerprintCopyBtns), ui.sshFingerprintCopyValues)
	}

	ui.syncSSHHostFingerprintButtons(entries[1:])
	if len(ui.sshFingerprintVisibleIDs) != 1 || ui.sshFingerprintVisibleIDs[0] != "two" ||
		len(ui.sshFingerprintCopyValues) != 1 || ui.sshFingerprintCopyValues[0] != "MD5:aa:bb" {
		t.Fatalf("filtered fingerprint actions = IDs:%v values:%v", ui.sshFingerprintVisibleIDs, ui.sshFingerprintCopyValues)
	}
}

func waitForFingerprintUIEvent(ui *Window, timeout time.Duration) bool {
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
