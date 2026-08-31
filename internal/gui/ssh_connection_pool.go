package gui

import (
	"context"
	"errors"
	"image"
	"io"
	"net"
	"sync"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

type sshTransport interface {
	OpenPTY(context.Context, int, int) (ptyTerminal, error)
	Close() error
}

type sshSFTPTransport interface {
	OpenSFTP() (sshclient.SFTPClient, error)
}

type sshChecksumTransport interface {
	ExecContext(context.Context, string) (string, error)
}

type sshTransportFactory func(remote.SSHHostCredentials) (sshTransport, error)

type sshClientTransport struct {
	client *sshclient.Client
}

func (transport *sshClientTransport) OpenPTY(ctx context.Context, width, height int) (ptyTerminal, error) {
	return transport.client.OpenPTY(ctx, width, height)
}

func (transport *sshClientTransport) OpenSFTP() (sshclient.SFTPClient, error) {
	return transport.client.OpenSFTP()
}

func (transport *sshClientTransport) ExecContext(ctx context.Context, command string) (string, error) {
	return transport.client.ExecContext(ctx, command)
}

func (transport *sshClientTransport) Dial(network, address string) (net.Conn, error) {
	return transport.client.Dial(network, address)
}

func (transport *sshClientTransport) Listen(network, address string) (net.Listener, error) {
	return transport.client.Listen(network, address)
}

func (transport *sshClientTransport) Close() error {
	if transport == nil || transport.client == nil {
		return nil
	}
	return transport.client.Close()
}

type sshConnectionKey struct {
	HostID  string
	Version int
}

type sshConnectionPool struct {
	mu            sync.Mutex
	entries       map[sshConnectionKey]*sshConnectionEntry
	disabledHosts map[string]bool
}

type sshConnectionEntry struct {
	ready     chan struct{}
	transport sshTransport
	err       error
	refs      int
	detached  bool
	closeOnce sync.Once
}

type sshConnectionLease struct {
	pool      *sshConnectionPool
	key       sshConnectionKey
	entry     *sshConnectionEntry
	closeOnce sync.Once
}

type pooledSFTPClient struct {
	client    sshclient.SFTPClient
	lease     *sshConnectionLease
	closeOnce sync.Once
}

func newSSHConnectionPool() *sshConnectionPool {
	return &sshConnectionPool{
		entries:       make(map[sshConnectionKey]*sshConnectionEntry),
		disabledHosts: make(map[string]bool),
	}
}

func newSSHClientTransport(credentials remote.SSHHostCredentials) (sshTransport, error) {
	client := sshclient.NewClient(sshProfileFromCredentials(credentials))
	if err := client.Connect(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &sshClientTransport{client: client}, nil
}

func openPooledSSHTerminal(
	ctx context.Context,
	pool *sshConnectionPool,
	credentials remote.SSHHostCredentials,
	size image.Point,
	factory sshTransportFactory,
) (*sshConnectionLease, ptyTerminal, error) {
	if ctx == nil || pool == nil || credentials.ID == "" || credentials.Version < 1 || factory == nil {
		return nil, nil, errors.New("ssh connection pool: invalid terminal request")
	}
	lease, err := pool.acquire(sshConnectionKey{HostID: credentials.ID, Version: credentials.Version}, func() (sshTransport, error) {
		return factory(credentials)
	})
	if err != nil {
		return nil, nil, err
	}
	width, height := 100, 30
	if size.X > 0 {
		width = size.X
	}
	if size.Y > 0 {
		height = size.Y
	}
	terminal, err := lease.transport().OpenPTY(context.WithoutCancel(ctx), width, height)
	if err != nil {
		_ = lease.Close()
		return nil, nil, err
	}
	return lease, terminal, nil
}

func openPooledSFTP(
	pool *sshConnectionPool,
	credentials remote.SSHHostCredentials,
	factory sshTransportFactory,
) (sshclient.SFTPClient, error) {
	if pool == nil || credentials.ID == "" || credentials.Version < 1 || factory == nil {
		return nil, errors.New("ssh connection pool: invalid SFTP request")
	}
	lease, err := pool.acquire(sshConnectionKey{HostID: credentials.ID, Version: credentials.Version}, func() (sshTransport, error) {
		return factory(credentials)
	})
	if err != nil {
		return nil, err
	}
	transport, ok := lease.transport().(sshSFTPTransport)
	if !ok {
		_ = lease.Close()
		return nil, errors.New("ssh connection pool: transport does not support SFTP")
	}
	client, err := transport.OpenSFTP()
	if err != nil {
		_ = lease.Close()
		return nil, err
	}
	if client == nil {
		_ = lease.Close()
		return nil, errors.New("ssh connection pool: transport returned no SFTP session")
	}
	return &pooledSFTPClient{client: client, lease: lease}, nil
}

func (client *pooledSFTPClient) ReadDir(ctx context.Context, remotePath string) ([]sshclient.SFTPEntry, error) {
	return client.client.ReadDir(ctx, remotePath)
}

func (client *pooledSFTPClient) Lstat(remotePath string) (sshclient.SFTPEntry, error) {
	return client.client.Lstat(remotePath)
}

func (client *pooledSFTPClient) Mkdir(remotePath string) error {
	return client.client.Mkdir(remotePath)
}

func (client *pooledSFTPClient) Rename(oldPath, newPath string) error {
	return client.client.Rename(oldPath, newPath)
}

func (client *pooledSFTPClient) Remove(remotePath string) error {
	return client.client.Remove(remotePath)
}

func (client *pooledSFTPClient) RemoveDirectory(remotePath string) error {
	return client.client.RemoveDirectory(remotePath)
}

func (client *pooledSFTPClient) Symlink(targetPath, linkPath string) error {
	return client.client.Symlink(targetPath, linkPath)
}

func (client *pooledSFTPClient) ReadLink(remotePath string) (string, error) {
	return client.client.ReadLink(remotePath)
}

func (client *pooledSFTPClient) OpenReader(remotePath string, offset int64) (io.ReadCloser, error) {
	return client.client.OpenReader(remotePath, offset)
}

func (client *pooledSFTPClient) OpenWriter(remotePath string, offset int64, truncate bool) (io.WriteCloser, error) {
	return client.client.OpenWriter(remotePath, offset, truncate)
}

func (client *pooledSFTPClient) Close() error {
	if client == nil {
		return nil
	}
	var err error
	client.closeOnce.Do(func() {
		err = errors.Join(client.client.Close(), client.lease.Close())
	})
	return err
}

type pooledSSHForward struct {
	forward   *sshclient.ForwardSession
	lease     *sshConnectionLease
	closeOnce sync.Once
}

func openPooledSSHForward(
	ctx context.Context,
	pool *sshConnectionPool,
	credentials remote.SSHHostCredentials,
	spec sshclient.ForwardSpec,
	factory sshTransportFactory,
) (*pooledSSHForward, error) {
	if ctx == nil || pool == nil || credentials.ID == "" || credentials.Version < 1 || factory == nil {
		return nil, errors.New("ssh connection pool: invalid forwarding request")
	}
	lease, err := pool.acquire(sshConnectionKey{HostID: credentials.ID, Version: credentials.Version}, func() (sshTransport, error) {
		return factory(credentials)
	})
	if err != nil {
		return nil, err
	}
	transport, ok := lease.transport().(sshclient.ForwardTransport)
	if !ok {
		_ = lease.Close()
		return nil, errors.New("ssh connection pool: transport does not support forwarding")
	}
	forward, err := sshclient.StartForward(ctx, transport, spec)
	if err != nil {
		_ = lease.Close()
		return nil, err
	}
	return &pooledSSHForward{forward: forward, lease: lease}, nil
}

func (forward *pooledSSHForward) Addr() net.Addr {
	if forward == nil || forward.forward == nil {
		return nil
	}
	return forward.forward.Addr()
}

func (forward *pooledSSHForward) Traffic() (up, down int64) {
	if forward == nil || forward.forward == nil {
		return 0, 0
	}
	return forward.forward.Traffic()
}

func (forward *pooledSSHForward) Close() error {
	if forward == nil {
		return nil
	}
	var err error
	forward.closeOnce.Do(func() {
		err = errors.Join(forward.forward.Close(), forward.lease.Close())
	})
	return err
}

func (pool *sshConnectionPool) acquire(key sshConnectionKey, factory func() (sshTransport, error)) (*sshConnectionLease, error) {
	if pool == nil || key.HostID == "" || key.Version < 1 || factory == nil {
		return nil, errors.New("ssh connection pool: invalid acquisition")
	}
	pool.mu.Lock()
	if pool.disabledHosts[key.HostID] {
		pool.mu.Unlock()
		return nil, errors.New("ssh connection pool: host is disabled")
	}
	if entry := pool.entries[key]; entry != nil {
		entry.refs++
		pool.mu.Unlock()
		<-entry.ready
		if entry.err != nil {
			return nil, entry.err
		}
		return &sshConnectionLease{pool: pool, key: key, entry: entry}, nil
	}
	entry := &sshConnectionEntry{ready: make(chan struct{}), refs: 1}
	pool.entries[key] = entry
	pool.mu.Unlock()

	entry.transport, entry.err = factory()
	if entry.err == nil && entry.transport == nil {
		entry.err = errors.New("ssh connection pool: factory returned no transport")
	}
	pool.mu.Lock()
	if entry.detached {
		if entry.err == nil {
			entry.err = errors.New("ssh connection pool: acquisition was cancelled")
		}
		close(entry.ready)
		pool.mu.Unlock()
		entry.closeOnce.Do(func() {
			if entry.transport != nil {
				_ = entry.transport.Close()
			}
		})
		return nil, entry.err
	}
	if entry.err != nil {
		delete(pool.entries, key)
	}
	close(entry.ready)
	pool.mu.Unlock()
	if entry.err != nil {
		return nil, entry.err
	}
	return &sshConnectionLease{pool: pool, key: key, entry: entry}, nil
}

func (pool *sshConnectionPool) setHostEnabled(hostID string, enabled bool) {
	if pool == nil || hostID == "" {
		return
	}
	pool.mu.Lock()
	if pool.disabledHosts == nil {
		pool.disabledHosts = make(map[string]bool)
	}
	if enabled {
		delete(pool.disabledHosts, hostID)
		pool.mu.Unlock()
		return
	}
	pool.disabledHosts[hostID] = true
	entries := make([]*sshConnectionEntry, 0)
	for key, entry := range pool.entries {
		if key.HostID != hostID {
			continue
		}
		entry.detached = true
		entries = append(entries, entry)
		delete(pool.entries, key)
	}
	pool.mu.Unlock()
	for _, entry := range entries {
		select {
		case <-entry.ready:
			entry.closeOnce.Do(func() {
				if entry.transport != nil {
					_ = entry.transport.Close()
				}
			})
		default:
		}
	}
}

func (lease *sshConnectionLease) transport() sshTransport {
	if lease == nil || lease.entry == nil {
		return nil
	}
	return lease.entry.transport
}

func (lease *sshConnectionLease) Close() error {
	if lease == nil {
		return nil
	}
	var err error
	lease.closeOnce.Do(func() {
		err = lease.pool.release(lease.key, lease.entry)
	})
	return err
}

func (pool *sshConnectionPool) release(key sshConnectionKey, entry *sshConnectionEntry) error {
	if pool == nil || entry == nil {
		return nil
	}
	pool.mu.Lock()
	current := pool.entries[key]
	if current == entry && entry.refs > 0 {
		entry.refs--
		if entry.refs == 0 {
			delete(pool.entries, key)
		}
	}
	shouldClose := current != entry || entry.refs == 0
	pool.mu.Unlock()
	if !shouldClose {
		return nil
	}
	var err error
	entry.closeOnce.Do(func() {
		if entry.transport != nil {
			err = entry.transport.Close()
		}
	})
	return err
}

func (pool *sshConnectionPool) closeAll() {
	if pool == nil {
		return
	}
	pool.mu.Lock()
	entries := make([]*sshConnectionEntry, 0, len(pool.entries))
	for _, entry := range pool.entries {
		entry.detached = true
		entries = append(entries, entry)
	}
	pool.entries = make(map[sshConnectionKey]*sshConnectionEntry)
	pool.mu.Unlock()
	for _, entry := range entries {
		select {
		case <-entry.ready:
			entry.closeOnce.Do(func() {
				if entry.transport != nil {
					_ = entry.transport.Close()
				}
			})
		default:
		}
	}
}
