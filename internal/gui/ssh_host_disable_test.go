package gui

import (
	"context"
	"strings"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

func TestSSHTabStoreCloseHostReleasesOnlyMatchingSessions(t *testing.T) {
	store := sshTabStore{}
	first := store.open(testSSHHost("host-1", "web"))
	second := store.open(testSSHHost("host-1", "web"))
	keep := store.open(testSSHHost("host-2", "database"))
	firstPTY := &testSSHCloser{}
	secondPTY := &testSSHCloser{}
	keepPTY := &testSSHCloser{}
	first.session = &sshTabSession{pty: firstPTY}
	second.session = &sshTabSession{pty: secondPTY}
	keep.session = &sshTabSession{pty: keepPTY}

	closed := store.closeHost("host-1")

	if len(closed) != 2 || len(store.tabs) != 1 || store.tabs[0] != keep {
		t.Fatalf("close host = %d closed, %d remaining", len(closed), len(store.tabs))
	}
	if !firstPTY.closed || !secondPTY.closed || keepPTY.closed {
		t.Fatalf("PTY close states = first %v, second %v, keep %v", firstPTY.closed, secondPTY.closed, keepPTY.closed)
	}
}

func TestSSHTunnelStoreStopHostReleasesOnlyMatchingRuntimes(t *testing.T) {
	store := newSSHTunnelStore()
	firstRule := testSSHTunnelRule("tunnel-1", "web")
	secondRule := testSSHTunnelRule("tunnel-2", "database")
	secondRule.HostID = "host-2"
	store.replace([]remote.SSHTunnelRule{firstRule, secondRule})
	firstRuntime := &testSSHTunnelRuntime{}
	secondRuntime := &testSSHTunnelRuntime{}
	store.attachRuntime(firstRule.ID, firstRuntime)
	store.attachRuntime(secondRule.ID, secondRuntime)

	if stopped := store.stopHost("host-1"); stopped != 1 {
		t.Fatalf("stopped tunnels = %d, want 1", stopped)
	}
	first, _ := store.get(firstRule.ID)
	second, _ := store.get(secondRule.ID)
	if first.Runtime != nil || first.Starting || first.Rule.Running {
		t.Fatalf("disabled host tunnel still running: %+v", first)
	}
	if second.Runtime != secondRuntime || !second.Rule.Running {
		t.Fatalf("other host tunnel was changed: %+v", second)
	}
	if firstRuntime.closed != 1 || secondRuntime.closed != 0 {
		t.Fatalf("runtime close counts = %d/%d", firstRuntime.closed, secondRuntime.closed)
	}
}

func TestSSHTunnelStoreRejectsRuntimeThatArrivesAfterHostStop(t *testing.T) {
	store := newSSHTunnelStore()
	rule := testSSHTunnelRule("tunnel-1", "web")
	store.replace([]remote.SSHTunnelRule{rule})
	if !store.setStarting(rule.ID) {
		t.Fatal("tunnel did not enter starting state")
	}
	if stopped := store.stopHost(rule.HostID); stopped != 1 {
		t.Fatalf("stopped tunnels = %d, want 1", stopped)
	}
	lateRuntime := &testSSHTunnelRuntime{}
	if store.attachStartingRuntime(rule.ID, lateRuntime) {
		t.Fatal("late runtime attached after host stop")
	}
	entry, _ := store.get(rule.ID)
	if lateRuntime.closed != 1 || entry.Runtime != nil || entry.Starting || entry.Rule.Running {
		t.Fatalf("late runtime state = closed %d, entry %+v", lateRuntime.closed, entry)
	}
}

func TestSSHConnectionPoolDisablesOnlyRequestedHost(t *testing.T) {
	pool := newSSHConnectionPool()
	firstTransport := &testSSHTransport{}
	secondTransport := &testSSHTransport{}
	firstLease, err := pool.acquire(sshConnectionKey{HostID: "host-1", Version: 1}, func() (sshTransport, error) {
		return firstTransport, nil
	})
	if err != nil {
		t.Fatalf("acquire first host: %v", err)
	}
	secondLease, err := pool.acquire(sshConnectionKey{HostID: "host-2", Version: 1}, func() (sshTransport, error) {
		return secondTransport, nil
	})
	if err != nil {
		t.Fatalf("acquire second host: %v", err)
	}

	pool.setHostEnabled("host-1", false)
	if firstTransport.closed != 1 || secondTransport.closed != 0 {
		t.Fatalf("transport close counts = %d/%d", firstTransport.closed, secondTransport.closed)
	}
	if _, err := pool.acquire(sshConnectionKey{HostID: "host-1", Version: 2}, func() (sshTransport, error) {
		return &testSSHTransport{}, nil
	}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled host acquisition error = %v", err)
	}

	pool.setHostEnabled("host-1", true)
	reenabledLease, err := pool.acquire(sshConnectionKey{HostID: "host-1", Version: 2}, func() (sshTransport, error) {
		return &testSSHTransport{}, nil
	})
	if err != nil {
		t.Fatalf("re-enabled host acquisition: %v", err)
	}
	_ = firstLease.Close()
	_ = secondLease.Close()
	_ = reenabledLease.Close()
	if firstTransport.closed != 1 {
		t.Fatalf("disabled transport closed %d times, want once", firstTransport.closed)
	}
}

func TestTransferManagerDisablesHostAndRejectsRetryUntilEnabled(t *testing.T) {
	started := make(chan string, 2)
	manager := newTransferManager(2, func(ctx context.Context, item transferItem, _ func(int64)) error {
		started <- item.ID
		<-ctx.Done()
		return ctx.Err()
	})
	defer manager.close()
	first := manager.enqueue(transferDownload, "host-1", "/one", "one", 10)
	second := manager.enqueue(transferDownload, "host-2", "/two", "two", 10)
	if first == nil || second == nil {
		t.Fatal("failed to enqueue transfer fixtures")
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("transfer worker did not start")
		}
	}

	if failed := manager.setHostEnabled("host-1", false, "Host is disabled."); failed != 1 {
		t.Fatalf("failed transfers = %d, want 1", failed)
	}
	firstState, _ := manager.item(first.ID)
	secondState, _ := manager.item(second.ID)
	if firstState.Status != transferFailed || firstState.Error != "Host is disabled." {
		t.Fatalf("disabled transfer state = %+v", firstState)
	}
	if secondState.Status != transferRunning {
		t.Fatalf("other host transfer state = %+v", secondState)
	}
	if manager.retry(first.ID) {
		t.Fatal("disabled host transfer must not retry")
	}
	if item := manager.enqueue(transferUpload, "host-1", "local", "/remote", 1); item != nil {
		t.Fatalf("disabled host accepted new transfer: %+v", item)
	}

	manager.setHostEnabled("host-1", true, "")
	if !manager.retry(first.ID) {
		t.Fatal("re-enabled host transfer should be retryable")
	}
}

func TestApplySSHHostsCascadesDisabledHostResources(t *testing.T) {
	ui := NewWindow(nil)
	ui.transfers.close()
	started := make(chan string, 2)
	ui.transfers = newTransferManager(2, func(ctx context.Context, item transferItem, _ func(int64)) error {
		started <- item.ID
		<-ctx.Done()
		return ctx.Err()
	})
	defer ui.transfers.close()

	disabledHost := testSSHHost("host-1", "web")
	disabledHost.Enabled = true
	keepHost := testSSHHost("host-2", "database")
	keepHost.Enabled = true
	ui.applySSHHosts([]remote.SSHHost{disabledHost, keepHost})

	closedTab := ui.sshTabs.open(disabledHost)
	keepTab := ui.sshTabs.open(keepHost)
	closedPTY := &testSSHCloser{}
	keepPTY := &testSSHCloser{}
	closedTab.session = &sshTabSession{pty: closedPTY}
	keepTab.session = &sshTabSession{pty: keepPTY}
	ui.sftpOperationOpen = true
	ui.sftpOperationTabID = closedTab.ID
	ui.sftpUploadConflicts = []sftpUploadConflict{{TabID: closedTab.ID}}
	ui.sftpUploadConflictOpen = true

	tunnelRule := testSSHTunnelRule("tunnel-1", "web")
	ui.sshTunnels.replace([]remote.SSHTunnelRule{tunnelRule})
	tunnelRuntime := &testSSHTunnelRuntime{}
	ui.sshTunnels.attachRuntime(tunnelRule.ID, tunnelRuntime)

	transport := &testSSHTransport{}
	lease, err := ui.sshPool.acquire(sshConnectionKey{HostID: disabledHost.ID, Version: 1}, func() (sshTransport, error) {
		return transport, nil
	})
	if err != nil {
		t.Fatalf("acquire disabled host fixture: %v", err)
	}
	transfer := ui.transfers.enqueue(transferDownload, disabledHost.ID, "/remote", "local", 10)
	if transfer == nil {
		t.Fatal("failed to enqueue disabled host fixture")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("disabled host transfer did not start")
	}

	disabledHost.Enabled = false
	ui.applySSHHosts([]remote.SSHHost{disabledHost, keepHost})

	if ui.sshTabs.get(closedTab.ID) != nil || ui.sshTabs.get(keepTab.ID) == nil {
		t.Fatalf("tabs after disable = %+v", ui.sshTabs.tabs)
	}
	if !closedPTY.closed || keepPTY.closed {
		t.Fatalf("PTY close states = disabled %v, keep %v", closedPTY.closed, keepPTY.closed)
	}
	if tunnelRuntime.closed != 1 {
		t.Fatalf("tunnel runtime close count = %d", tunnelRuntime.closed)
	}
	if transport.closed != 1 {
		t.Fatalf("pooled transport close count = %d", transport.closed)
	}
	transferState, _ := ui.transfers.item(transfer.ID)
	if transferState.Status != transferFailed || transferState.Error != "Host is disabled." {
		t.Fatalf("disabled transfer state = %+v", transferState)
	}
	if ui.sftpOperationOpen || ui.sftpUploadConflictOpen || len(ui.sftpUploadConflicts) != 0 {
		t.Fatalf("disabled host SFTP overlays remain open: operation=%v conflict=%v entries=%d", ui.sftpOperationOpen, ui.sftpUploadConflictOpen, len(ui.sftpUploadConflicts))
	}
	_ = lease.Close()
	if transport.closed != 1 {
		t.Fatalf("pooled transport closed %d times", transport.closed)
	}
}

func TestApplySSHHostsDoesNotCascadeRemovedHostResources(t *testing.T) {
	ui := NewWindow(nil)
	host := testSSHHost("host-1", "web")
	host.Enabled = true
	ui.applySSHHosts([]remote.SSHHost{host})
	tab := ui.sshTabs.open(host)
	pty := &testSSHCloser{}
	tab.session = &sshTabSession{pty: pty}

	ui.applySSHHosts(nil)

	if ui.sshTabs.get(tab.ID) == nil || pty.closed {
		t.Fatal("removing host metadata must not close an existing tab")
	}
	ui.sshTabs.closeAll()
}
