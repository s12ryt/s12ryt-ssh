package gui

import (
	"context"
	"image"
	"io"
	"net"
	"testing"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

type testSSHTransport struct {
	closed       int
	closedSignal chan struct{}
	opened       []image.Point
	ptys         []*testSSHCloser
}

type testSFTPTransport struct {
	*testSSHTransport
	entries  []sshclient.SFTPEntry
	sessions []*testSFTPClient
}

type testForwardTransport struct {
	*testSSHTransport
}

func (transport *testForwardTransport) Dial(network, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

func (transport *testForwardTransport) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func (transport *testSFTPTransport) OpenSFTP() (sshclient.SFTPClient, error) {
	session := &testSFTPClient{entries: append([]sshclient.SFTPEntry(nil), transport.entries...)}
	transport.sessions = append(transport.sessions, session)
	return session, nil
}

type testSFTPClient struct {
	closed    int
	entries   []sshclient.SFTPEntry
	readPaths []string
}

func (client *testSFTPClient) ReadDir(_ context.Context, remotePath string) ([]sshclient.SFTPEntry, error) {
	client.readPaths = append(client.readPaths, remotePath)
	return append([]sshclient.SFTPEntry(nil), client.entries...), nil
}
func (*testSFTPClient) Lstat(string) (sshclient.SFTPEntry, error) {
	return sshclient.SFTPEntry{}, nil
}
func (*testSFTPClient) Mkdir(string) error              { return nil }
func (*testSFTPClient) Rename(string, string) error     { return nil }
func (*testSFTPClient) Remove(string) error             { return nil }
func (*testSFTPClient) RemoveDirectory(string) error    { return nil }
func (*testSFTPClient) Symlink(string, string) error    { return nil }
func (*testSFTPClient) ReadLink(string) (string, error) { return "", nil }
func (*testSFTPClient) OpenReader(string, int64) (io.ReadCloser, error) {
	return nil, nil
}
func (*testSFTPClient) OpenWriter(string, int64, bool) (io.WriteCloser, error) {
	return nil, nil
}
func (client *testSFTPClient) Close() error {
	client.closed++
	return nil
}

func (transport *testSSHTransport) OpenPTY(_ context.Context, width, height int) (ptyTerminal, error) {
	pty := &testSSHCloser{}
	transport.opened = append(transport.opened, image.Pt(width, height))
	transport.ptys = append(transport.ptys, pty)
	return pty, nil
}

func (transport *testSSHTransport) Close() error {
	transport.closed++
	if transport.closed == 1 && transport.closedSignal != nil {
		close(transport.closedSignal)
	}
	return nil
}

func TestSSHConnectionPoolSharesSameHostVersionUntilLastRelease(t *testing.T) {
	pool := newSSHConnectionPool()
	key := sshConnectionKey{HostID: "host-1", Version: 3}
	created := 0
	factory := func() (sshTransport, error) {
		created++
		return &testSSHTransport{}, nil
	}

	first, err := pool.acquire(key, factory)
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	second, err := pool.acquire(key, factory)
	if err != nil {
		t.Fatalf("acquire second lease: %v", err)
	}
	if created != 1 || first.transport() != second.transport() {
		t.Fatalf("same host version created %d transports", created)
	}
	transport := first.transport().(*testSSHTransport)
	if err := first.Close(); err != nil {
		t.Fatalf("release first lease: %v", err)
	}
	if transport.closed != 0 {
		t.Fatal("shared transport closed before its last lease")
	}
	if err := second.Close(); err != nil {
		t.Fatalf("release second lease: %v", err)
	}
	if transport.closed != 1 {
		t.Fatalf("transport close count = %d, want 1", transport.closed)
	}
}

func TestSSHConnectionPoolIsolatesConfigurationVersionsAndClosesAll(t *testing.T) {
	pool := newSSHConnectionPool()
	oldTransport := &testSSHTransport{}
	newTransport := &testSSHTransport{}
	oldLease, err := pool.acquire(sshConnectionKey{HostID: "host-1", Version: 1}, func() (sshTransport, error) {
		return oldTransport, nil
	})
	if err != nil {
		t.Fatalf("acquire old version: %v", err)
	}
	newLease, err := pool.acquire(sshConnectionKey{HostID: "host-1", Version: 2}, func() (sshTransport, error) {
		return newTransport, nil
	})
	if err != nil {
		t.Fatalf("acquire new version: %v", err)
	}
	if oldLease.transport() == newLease.transport() {
		t.Fatal("different host versions shared one transport")
	}

	pool.closeAll()
	if oldTransport.closed != 1 || newTransport.closed != 1 {
		t.Fatalf("close counts = old %d, new %d; want 1 each", oldTransport.closed, newTransport.closed)
	}
	if err := oldLease.Close(); err != nil {
		t.Fatalf("release old lease after closeAll: %v", err)
	}
	if err := newLease.Close(); err != nil {
		t.Fatalf("release new lease after closeAll: %v", err)
	}
	if oldTransport.closed != 1 || newTransport.closed != 1 {
		t.Fatal("lease release closed transports twice after closeAll")
	}
}

func TestOpenPooledSSHTerminalSharesTransportAndKeepsPTYsIndependent(t *testing.T) {
	pool := newSSHConnectionPool()
	credentials := remote.SSHHostCredentials{ID: "host-1", Version: 7}
	transport := &testSSHTransport{}
	created := 0
	factory := func(got remote.SSHHostCredentials) (sshTransport, error) {
		created++
		if got.ID != credentials.ID || got.Version != credentials.Version {
			t.Fatalf("factory credentials = %+v", got)
		}
		return transport, nil
	}

	firstLease, firstPTY, err := openPooledSSHTerminal(context.Background(), pool, credentials, image.Pt(80, 24), factory)
	if err != nil {
		t.Fatalf("open first pooled terminal: %v", err)
	}
	secondLease, secondPTY, err := openPooledSSHTerminal(context.Background(), pool, credentials, image.Pt(120, 40), factory)
	if err != nil {
		t.Fatalf("open second pooled terminal: %v", err)
	}
	if created != 1 {
		t.Fatalf("transport factory calls = %d, want 1", created)
	}
	if firstPTY == secondPTY || len(transport.ptys) != 2 {
		t.Fatal("pooled tabs must receive independent PTY sessions")
	}
	if len(transport.opened) != 2 || transport.opened[0] != image.Pt(80, 24) || transport.opened[1] != image.Pt(120, 40) {
		t.Fatalf("opened PTY sizes = %+v", transport.opened)
	}

	firstSession := &sshTabSession{pty: firstPTY, client: firstLease}
	secondSession := &sshTabSession{pty: secondPTY, client: secondLease}
	firstSession.close()
	if transport.closed != 0 || !transport.ptys[0].closed || transport.ptys[1].closed {
		t.Fatalf("first close = transport %d, PTYs %+v", transport.closed, transport.ptys)
	}
	secondSession.close()
	if transport.closed != 1 || !transport.ptys[1].closed {
		t.Fatalf("last close = transport %d, second PTY closed %v", transport.closed, transport.ptys[1].closed)
	}
}

func TestOpenPooledSFTPSharesTransportAndKeepsSessionsIndependent(t *testing.T) {
	pool := newSSHConnectionPool()
	credentials := remote.SSHHostCredentials{ID: "host-1", Version: 7}
	transport := &testSFTPTransport{testSSHTransport: &testSSHTransport{}}
	created := 0
	factory := func(got remote.SSHHostCredentials) (sshTransport, error) {
		created++
		if got.ID != credentials.ID || got.Version != credentials.Version {
			t.Fatalf("factory credentials = %+v", got)
		}
		return transport, nil
	}

	first, err := openPooledSFTP(pool, credentials, factory)
	if err != nil {
		t.Fatalf("open first pooled SFTP session: %v", err)
	}
	second, err := openPooledSFTP(pool, credentials, factory)
	if err != nil {
		t.Fatalf("open second pooled SFTP session: %v", err)
	}
	if created != 1 || len(transport.sessions) != 2 || first == second {
		t.Fatalf("pooled SFTP = factories %d, sessions %d", created, len(transport.sessions))
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first pooled SFTP session: %v", err)
	}
	if transport.sessions[0].closed != 1 || transport.sessions[1].closed != 0 || transport.closed != 0 {
		t.Fatalf("first close = sessions %d/%d, transport %d", transport.sessions[0].closed, transport.sessions[1].closed, transport.closed)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second pooled SFTP session: %v", err)
	}
	if transport.sessions[1].closed != 1 || transport.closed != 1 {
		t.Fatalf("last close = session %d, transport %d", transport.sessions[1].closed, transport.closed)
	}
}

func TestOpenPooledSSHForwardSharesTransportUntilLastRuleCloses(t *testing.T) {
	pool := newSSHConnectionPool()
	credentials := remote.SSHHostCredentials{ID: "host-1", Version: 9}
	transport := &testForwardTransport{testSSHTransport: &testSSHTransport{}}
	created := 0
	factory := func(got remote.SSHHostCredentials) (sshTransport, error) {
		created++
		if got.ID != credentials.ID || got.Version != credentials.Version {
			t.Fatalf("factory credentials = %+v", got)
		}
		return transport, nil
	}
	spec := sshclient.ForwardSpec{
		Type:       sshclient.ForwardLocal,
		ListenHost: "127.0.0.1",
		ListenPort: 0,
		TargetHost: "127.0.0.1",
		TargetPort: 1,
	}
	first, err := openPooledSSHForward(context.Background(), pool, credentials, spec, factory)
	if err != nil {
		t.Fatalf("open first pooled forward: %v", err)
	}
	second, err := openPooledSSHForward(context.Background(), pool, credentials, spec, factory)
	if err != nil {
		t.Fatalf("open second pooled forward: %v", err)
	}
	if created != 1 || first == second {
		t.Fatalf("forward pool created %d transports", created)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first forward: %v", err)
	}
	if transport.closed != 0 {
		t.Fatalf("transport closed after first forward: %d", transport.closed)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second forward: %v", err)
	}
	if transport.closed != 1 {
		t.Fatalf("transport close count = %d, want 1", transport.closed)
	}
}
