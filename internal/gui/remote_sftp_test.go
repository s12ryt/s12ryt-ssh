package gui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

type operationSFTPClient struct {
	entries    []sshclient.SFTPEntry
	readPaths  []string
	mkdirs     []string
	renames    [][2]string
	removes    []string
	removeDirs []string
	lstatPaths []string
	lstatEntry sshclient.SFTPEntry
	symlinks   [][2]string
}

func (client *operationSFTPClient) ReadDir(_ context.Context, remotePath string) ([]sshclient.SFTPEntry, error) {
	client.readPaths = append(client.readPaths, remotePath)
	return append([]sshclient.SFTPEntry(nil), client.entries...), nil
}

func (client *operationSFTPClient) Mkdir(remotePath string) error {
	client.mkdirs = append(client.mkdirs, remotePath)
	return nil
}
func (client *operationSFTPClient) Rename(oldPath, newPath string) error {
	client.renames = append(client.renames, [2]string{oldPath, newPath})
	return nil
}
func (client *operationSFTPClient) Remove(remotePath string) error {
	client.removes = append(client.removes, remotePath)
	return nil
}
func (client *operationSFTPClient) RemoveDirectory(remotePath string) error {
	client.removeDirs = append(client.removeDirs, remotePath)
	return nil
}
func (client *operationSFTPClient) Symlink(targetPath, linkPath string) error {
	client.symlinks = append(client.symlinks, [2]string{targetPath, linkPath})
	return nil
}
func (*operationSFTPClient) ReadLink(string) (string, error)                 { return "", nil }
func (*operationSFTPClient) OpenReader(string, int64) (io.ReadCloser, error) { return nil, nil }
func (*operationSFTPClient) OpenWriter(string, int64, bool) (io.WriteCloser, error) {
	return nil, nil
}
func (client *operationSFTPClient) Lstat(remotePath string) (sshclient.SFTPEntry, error) {
	client.lstatPaths = append(client.lstatPaths, remotePath)
	return client.lstatEntry, nil
}
func (*operationSFTPClient) Close() error { return nil }

type sftpRemoteSession struct {
	fakeRemoteSession
	credentials remote.SSHHostCredentials
}

func (session *sftpRemoteSession) SSHHostCredentials(context.Context, string) (remote.SSHHostCredentials, error) {
	return session.credentials, nil
}

func TestAttachSSHTabSFTPAppliesOnlyToCurrentTerminalSession(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	current := &sshTabSession{pty: &testSSHCloser{}}
	tab.session = current
	client := &testSFTPClient{}
	modified := time.Date(2026, time.August, 30, 11, 0, 0, 0, time.UTC)
	entries := []sshclient.SFTPEntry{
		{Name: "zeta.log", Path: "/zeta.log", Size: 42, Modified: modified},
		{Name: "apps", Path: "/apps", Directory: true, Modified: modified},
	}

	if !ui.attachSSHTabSFTP(tab.ID, current, client, "/", entries) {
		t.Fatal("current terminal session should accept SFTP")
	}
	if current.sftp != client || tab.sftpBrowser == nil || tab.View != sshTabViewSFTP {
		t.Fatalf("attached SFTP state = client %v, browser %#v, view %q", current.sftp == client, tab.sftpBrowser, tab.View)
	}
	if len(tab.sftpBrowser.Entries) != 2 || tab.sftpBrowser.Entries[0].Name != "apps" || tab.sftpBrowser.Entries[1].Size != 42 {
		t.Fatalf("mapped browser entries = %#v", tab.sftpBrowser.Entries)
	}

	staleClient := &testSFTPClient{}
	stale := &sshTabSession{pty: &testSSHCloser{}}
	if ui.attachSSHTabSFTP(tab.ID, stale, staleClient, "/old", nil) {
		t.Fatal("stale terminal session must not replace current SFTP state")
	}
	if staleClient.closed != 1 || current.sftp != client || tab.sftpBrowser.Path != "/" {
		t.Fatalf("stale attach cleanup = closed %d, path %q", staleClient.closed, tab.sftpBrowser.Path)
	}
}

func TestOpenSSHTabSFTPUsesVersionedPoolAndLoadsRootDirectory(t *testing.T) {
	ui := NewWindow(nil)
	ui.model.RemoteSession = &sftpRemoteSession{credentials: remote.SSHHostCredentials{
		ID: "host-1", Version: 4, Host: "web.example.com", Port: 22, Username: "deploy",
	}}
	modified := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	transport := &testSFTPTransport{
		testSSHTransport: &testSSHTransport{},
		entries: []sshclient.SFTPEntry{
			{Name: "notes.txt", Path: "/notes.txt", Size: 18, Modified: modified},
			{Name: "srv", Path: "/srv", Directory: true, Modified: modified},
		},
	}
	factoryCalls := 0
	ui.sshTransportFactory = func(credentials remote.SSHHostCredentials) (sshTransport, error) {
		factoryCalls++
		if credentials.ID != "host-1" || credentials.Version != 4 {
			t.Fatalf("factory credentials = %+v", credentials)
		}
		return transport, nil
	}
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	current := &sshTabSession{pty: &testSSHCloser{}}
	tab.session = current
	tab.State = sshTabConnected

	if !ui.openSSHTabSFTP(tab.ID) {
		t.Fatal("connected remote tab should start opening SFTP")
	}
	if !tab.sftpLoading || tab.View != sshTabViewSFTP {
		t.Fatalf("opening state = loading %v, view %q", tab.sftpLoading, tab.View)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("SFTP open did not queue its UI result")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()

	if factoryCalls != 1 || len(transport.sessions) != 1 {
		t.Fatalf("SFTP transport calls = factory %d, sessions %d", factoryCalls, len(transport.sessions))
	}
	if current.sftp == nil || tab.sftpLoading || tab.sftpError != "" || tab.sftpBrowser == nil {
		t.Fatalf("attached SFTP state = session %v, loading %v, error %q, browser %#v", current.sftp != nil, tab.sftpLoading, tab.sftpError, tab.sftpBrowser)
	}
	if tab.sftpBrowser.Path != "/" || len(tab.sftpBrowser.Entries) != 2 || tab.sftpBrowser.Entries[0].Name != "srv" {
		t.Fatalf("root directory = path %q, entries %#v", tab.sftpBrowser.Path, tab.sftpBrowser.Entries)
	}

	ui.closeSSHTab(tab.ID)
	if transport.sessions[0].closed != 1 || transport.closed != 1 {
		t.Fatalf("SFTP tab cleanup = session %d, transport %d", transport.sessions[0].closed, transport.closed)
	}
}

func TestRefreshSSHTabSFTPReloadsCurrentPathWithoutReplacingSession(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &testSFTPClient{entries: []sshclient.SFTPEntry{
		{Name: "first.txt", Path: "/srv/first.txt", Size: 1},
	}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	tab.sftpInfo = "stale metadata"

	if !ui.refreshSSHTabSFTP(tab.ID) {
		t.Fatal("connected SFTP tab should start refreshing")
	}
	if !tab.sftpLoading {
		t.Fatal("refresh must expose a loading state")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("SFTP refresh did not queue its UI result")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()

	if current.sftp != client || client.closed != 0 {
		t.Fatalf("refresh replaced or closed the SFTP session: current %v, closed %d", current.sftp == client, client.closed)
	}
	if len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("refresh paths = %#v, want /srv", client.readPaths)
	}
	if tab.sftpLoading || tab.sftpError != "" || tab.sftpInfo != "" || len(tab.sftpBrowser.Entries) != 1 || tab.sftpBrowser.Entries[0].Name != "first.txt" {
		t.Fatalf("refreshed browser = loading %v, error %q, info %q, entries %#v", tab.sftpLoading, tab.sftpError, tab.sftpInfo, tab.sftpBrowser.Entries)
	}
}

func TestNavigateSSHTabSFTPEntersDirectoryAndReturnsToParent(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &testSFTPClient{entries: []sshclient.SFTPEntry{
		{Name: "srv", Path: "/srv", Directory: true},
	}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/")
	tab.sftpBrowser.applyEntries(sftpEntriesFromTransport(client.entries))

	if !ui.enterSSHTabSFTPDirectory(tab.ID, "srv") {
		t.Fatal("directory entry should start loading its path")
	}
	waitForSFTPUIEvent(t, ui)
	if tab.sftpBrowser.Path != "/srv" || len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("entered path = browser %q, reads %#v", tab.sftpBrowser.Path, client.readPaths)
	}

	if !ui.parentSSHTabSFTP(tab.ID) {
		t.Fatal("non-root directory should navigate to its parent")
	}
	waitForSFTPUIEvent(t, ui)
	if tab.sftpBrowser.Path != "/" || len(client.readPaths) != 2 || client.readPaths[1] != "/" {
		t.Fatalf("parent path = browser %q, reads %#v", tab.sftpBrowser.Path, client.readPaths)
	}
	if ui.parentSSHTabSFTP(tab.ID) {
		t.Fatal("root directory must not navigate above root")
	}
}

func TestCreateSSHTabSFTPFolderRefreshesCurrentDirectory(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &operationSFTPClient{entries: []sshclient.SFTPEntry{{Name: "new", Path: "/srv/new", Directory: true}}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")

	if !ui.createSSHTabSFTPFolder(tab.ID, "new") {
		t.Fatal("valid folder name should start a remote operation")
	}
	waitForSFTPUIEvent(t, ui)
	if len(client.mkdirs) != 1 || client.mkdirs[0] != "/srv/new" {
		t.Fatalf("mkdir paths = %#v", client.mkdirs)
	}
	if len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("refresh paths = %#v", client.readPaths)
	}
	if tab.sftpLoading || tab.sftpError != "" || len(tab.sftpBrowser.Entries) != 1 {
		t.Fatalf("folder operation state = loading %v, error %q, entries %#v", tab.sftpLoading, tab.sftpError, tab.sftpBrowser.Entries)
	}
}

func TestRenameSSHTabSFTPItemRefreshesCurrentDirectory(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &operationSFTPClient{entries: []sshclient.SFTPEntry{{Name: "renamed.txt", Path: "/srv/renamed.txt"}}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	tab.sftpBrowser.applyEntries([]sftpEntry{{Name: "old.txt", Path: "/srv/old.txt"}})

	if !ui.renameSSHTabSFTPItem(tab.ID, "old.txt", "renamed.txt") {
		t.Fatal("valid rename should start a remote operation")
	}
	waitForSFTPUIEvent(t, ui)
	if len(client.renames) != 1 || client.renames[0] != [2]string{"/srv/old.txt", "/srv/renamed.txt"} {
		t.Fatalf("rename paths = %#v", client.renames)
	}
	if len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("rename refresh paths = %#v", client.readPaths)
	}
}

func TestDeleteSSHTabSFTPSelectionRequiresConfirmationAndRefreshes(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &operationSFTPClient{entries: []sshclient.SFTPEntry{
		{Name: "folder", Path: "/srv/folder", Directory: true},
		{Name: "file.txt", Path: "/srv/file.txt"},
	}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")
	tab.sftpBrowser.applyEntries(sftpEntriesFromTransport(client.entries))
	tab.sftpBrowser.toggleSelection("/srv/folder")
	tab.sftpBrowser.toggleSelection("/srv/file.txt")

	if !ui.deleteSelectedSSHTabSFTPItems(tab.ID) {
		t.Fatal("selected entries should request confirmation")
	}
	if !ui.confirm.active || len(client.removes) != 0 || len(client.removeDirs) != 0 {
		t.Fatalf("delete before confirmation = active %v, files %#v, dirs %#v", ui.confirm.active, client.removes, client.removeDirs)
	}
	ui.confirm.accept()
	waitForSFTPUIEvent(t, ui)
	if len(client.removes) != 1 || client.removes[0] != "/srv/file.txt" || len(client.removeDirs) != 1 || client.removeDirs[0] != "/srv/folder" {
		t.Fatalf("delete paths = files %#v, dirs %#v", client.removes, client.removeDirs)
	}
	if len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("delete refresh paths = %#v", client.readPaths)
	}
}

func TestShowSSHTabSFTPItemInfoUsesCurrentSession(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	modified := time.Date(2026, time.August, 30, 13, 14, 15, 0, time.UTC)
	client := &operationSFTPClient{lstatEntry: sshclient.SFTPEntry{
		Name: "report.txt", Path: "/srv/report.txt", Size: 2048, Modified: modified,
	}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")

	if !ui.showSSHTabSFTPItemInfo(tab.ID, "/srv/report.txt") {
		t.Fatal("file information should start a remote lookup")
	}
	waitForSFTPUIEvent(t, ui)
	if len(client.lstatPaths) != 1 || client.lstatPaths[0] != "/srv/report.txt" {
		t.Fatalf("info paths = %#v", client.lstatPaths)
	}
	if !strings.Contains(tab.sftpInfo, "report.txt") || !strings.Contains(tab.sftpInfo, "2048") {
		t.Fatalf("file info = %q", tab.sftpInfo)
	}
}

func TestCreateSSHTabSFTPSymlinkRefreshesCurrentDirectory(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	client := &operationSFTPClient{entries: []sshclient.SFTPEntry{{Name: "link", Path: "/srv/link", Symlink: true}}}
	current := &sshTabSession{pty: &testSSHCloser{}, sftp: client}
	tab.session = current
	tab.State = sshTabConnected
	tab.View = sshTabViewSFTP
	tab.sftpBrowser = newSFTPBrowserState("/srv")

	if !ui.createSSHTabSFTPSymlink(tab.ID, "/srv/target", "link") {
		t.Fatal("valid symbolic link should start a remote operation")
	}
	waitForSFTPUIEvent(t, ui)
	if len(client.symlinks) != 1 || client.symlinks[0] != [2]string{"/srv/target", "/srv/link"} {
		t.Fatalf("symlink paths = %#v", client.symlinks)
	}
	if len(client.readPaths) != 1 || client.readPaths[0] != "/srv" {
		t.Fatalf("symlink refresh paths = %#v", client.readPaths)
	}
}

func TestSyncSSHTabSFTPEntryWidgetsFollowsRemotePathsAndSelections(t *testing.T) {
	tab := &sshTab{sftpBrowser: newSFTPBrowserState("/")}
	tab.sftpBrowser.applyEntries([]sftpEntry{
		{Name: "first.txt", Path: "/first.txt"},
		{Name: "second.txt", Path: "/second.txt"},
	})
	tab.sftpBrowser.selections["/second.txt"] = true

	tab.syncSFTPEntryWidgets()
	if len(tab.sftpSelectionWidgets) != 2 || len(tab.sftpOpenButtons) != 2 || len(tab.sftpEntryPaths) != 2 {
		t.Fatalf("entry widgets = selections %d, opens %d, paths %d", len(tab.sftpSelectionWidgets), len(tab.sftpOpenButtons), len(tab.sftpEntryPaths))
	}
	if tab.sftpSelectionWidgets[0].Value || !tab.sftpSelectionWidgets[1].Value {
		t.Fatalf("selection widget values = %v/%v", tab.sftpSelectionWidgets[0].Value, tab.sftpSelectionWidgets[1].Value)
	}

	tab.sftpBrowser.applyEntries([]sftpEntry{
		{Name: "new.txt", Path: "/new.txt"},
		{Name: "other.txt", Path: "/other.txt"},
	})
	tab.syncSFTPEntryWidgets()
	if tab.sftpEntryPaths[0] != "/new.txt" || tab.sftpEntryPaths[1] != "/other.txt" {
		t.Fatalf("rebuilt entry paths = %#v", tab.sftpEntryPaths)
	}
	if tab.sftpSelectionWidgets[0].Value || tab.sftpSelectionWidgets[1].Value {
		t.Fatal("refreshed entries inherited stale selection values")
	}
}

func waitForSFTPUIEvent(t *testing.T, ui *Window) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(ui.events) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("SFTP operation did not queue its UI result")
		}
		time.Sleep(time.Millisecond)
	}
	ui.pump()
}
