package ssh

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// ForwardType identifies an SSH port-forwarding mode.
type ForwardType string

const (
	ForwardLocal   ForwardType = "local"
	ForwardRemote  ForwardType = "remote"
	ForwardDynamic ForwardType = "dynamic_socks"
)

// ForwardSpec describes one runtime forwarding listener.
type ForwardSpec struct {
	Type       ForwardType
	ListenHost string
	ListenPort int
	TargetHost string
	TargetPort int
}

// ForwardTransport supplies the SSH-side Dial and Listen operations.
type ForwardTransport interface {
	Dial(network, address string) (net.Conn, error)
	Listen(network, address string) (net.Listener, error)
}

// ForwardSession owns one forwarding listener and all of its active streams.
type ForwardSession struct {
	listener  net.Listener
	transport ForwardTransport
	spec      ForwardSpec
	cancel    context.CancelFunc

	closeOnce   sync.Once
	wg          sync.WaitGroup
	mu          sync.Mutex
	closed      bool
	connections map[net.Conn]struct{}
	up          atomic.Int64
	down        atomic.Int64
}

// StartForward starts a local, remote, or dynamic SOCKS5 forwarding listener.
func StartForward(ctx context.Context, transport ForwardTransport, spec ForwardSpec) (*ForwardSession, error) {
	if ctx == nil {
		return nil, errors.New("ssh: forwarding requires a context")
	}
	if transport == nil {
		return nil, errors.New("ssh: forwarding requires a transport")
	}
	if spec.ListenHost == "" {
		spec.ListenHost = "127.0.0.1"
	}
	if spec.ListenPort < 0 || spec.ListenPort > 65535 {
		return nil, errors.New("ssh: invalid forwarding listen port")
	}
	if spec.Type != ForwardDynamic {
		if strings.TrimSpace(spec.TargetHost) == "" || spec.TargetPort < 1 || spec.TargetPort > 65535 {
			return nil, errors.New("ssh: forwarding target is required")
		}
	} else if spec.TargetPort < 0 || spec.TargetPort > 65535 {
		return nil, errors.New("ssh: invalid forwarding target port")
	}

	var listener net.Listener
	var err error
	address := net.JoinHostPort(spec.ListenHost, strconv.Itoa(spec.ListenPort))
	if spec.Type == ForwardRemote {
		listener, err = transport.Listen("tcp", address)
	} else {
		listener, err = net.Listen("tcp", address)
	}
	if err != nil {
		return nil, err
	}
	derived, cancel := context.WithCancel(ctx)
	session := &ForwardSession{
		listener:    listener,
		transport:   transport,
		spec:        spec,
		cancel:      cancel,
		connections: make(map[net.Conn]struct{}),
	}
	session.wg.Add(1)
	go session.acceptLoop(derived)
	go func() {
		<-derived.Done()
		_ = session.Close()
	}()
	return session, nil
}

// Addr returns the bound listener address.
func (session *ForwardSession) Addr() net.Addr {
	if session == nil || session.listener == nil {
		return nil
	}
	return session.listener.Addr()
}

// Traffic returns the bytes copied from the local side to the remote side and back.
func (session *ForwardSession) Traffic() (up, down int64) {
	if session == nil {
		return 0, 0
	}
	return session.up.Load(), session.down.Load()
}

// Close stops accepting connections and closes all active streams.
func (session *ForwardSession) Close() error {
	if session == nil {
		return nil
	}
	var closeErr error
	session.closeOnce.Do(func() {
		session.cancel()
		session.mu.Lock()
		session.closed = true
		connections := make([]net.Conn, 0, len(session.connections))
		for connection := range session.connections {
			connections = append(connections, connection)
		}
		session.mu.Unlock()
		closeErr = session.listener.Close()
		for _, connection := range connections {
			_ = connection.Close()
		}
		session.wg.Wait()
	})
	if errors.Is(closeErr, net.ErrClosed) {
		return nil
	}
	return closeErr
}

func (session *ForwardSession) acceptLoop(ctx context.Context) {
	defer session.wg.Done()
	for {
		connection, err := session.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}
		if !session.register(connection) {
			_ = connection.Close()
			return
		}
		session.wg.Add(1)
		go func() {
			defer session.wg.Done()
			defer session.unregister(connection)
			if session.spec.Type == ForwardDynamic {
				session.serveSOCKS(ctx, connection)
				return
			}
			session.serveForward(ctx, connection)
		}()
	}
}

func (session *ForwardSession) register(connection net.Conn) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return false
	}
	session.connections[connection] = struct{}{}
	return true
}

func (session *ForwardSession) unregister(connection net.Conn) {
	session.mu.Lock()
	delete(session.connections, connection)
	session.mu.Unlock()
	_ = connection.Close()
}

func (session *ForwardSession) serveForward(ctx context.Context, local net.Conn) {
	address := net.JoinHostPort(session.spec.TargetHost, strconv.Itoa(session.spec.TargetPort))
	remote, err := session.transport.Dial("tcp", address)
	if err != nil {
		return
	}
	if !session.register(remote) {
		_ = remote.Close()
		return
	}
	defer session.unregister(remote)
	proxyConnections(ctx, local, remote, &session.up, &session.down)
}

func proxyConnections(ctx context.Context, left, right net.Conn, up, down *atomic.Int64) {
	type result struct{ bytes int64 }
	results := make(chan result, 2)
	copySide := func(destination net.Conn, source net.Conn, counter *atomic.Int64) {
		bytes, _ := io.Copy(destination, source)
		counter.Add(bytes)
		results <- result{bytes: bytes}
	}
	go copySide(right, left, up)
	go copySide(left, right, down)
	select {
	case <-ctx.Done():
	case <-results:
	}
	_ = left.Close()
	_ = right.Close()
	<-results
}

func (session *ForwardSession) serveSOCKS(ctx context.Context, connection net.Conn) {
	address, err := readSOCKS5Request(connection)
	if err != nil {
		return
	}
	remote, err := session.transport.Dial("tcp", address)
	if err != nil {
		_, _ = connection.Write(socks5Reply(5))
		return
	}
	if !session.register(remote) {
		_ = remote.Close()
		return
	}
	defer session.unregister(remote)
	if _, err := connection.Write(socks5Reply(0)); err != nil {
		return
	}
	proxyConnections(ctx, connection, remote, &session.up, &session.down)
}

func readSOCKS5Request(connection net.Conn) (string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(connection, header); err != nil || header[0] != 5 {
		return "", errors.New("ssh: invalid SOCKS version")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(connection, methods); err != nil {
		return "", err
	}
	noAuth := false
	for _, method := range methods {
		if method == 0 {
			noAuth = true
			break
		}
	}
	if !noAuth {
		_, _ = connection.Write([]byte{5, 0xff})
		return "", errors.New("ssh: SOCKS authentication is unavailable")
	}
	if _, err := connection.Write([]byte{5, 0}); err != nil {
		return "", err
	}
	request := make([]byte, 4)
	if _, err := io.ReadFull(connection, request); err != nil {
		return "", err
	}
	if request[0] != 5 || request[1] != 1 || request[2] != 0 {
		_, _ = connection.Write(socks5Reply(7))
		return "", errors.New("ssh: unsupported SOCKS request")
	}
	host, err := readSOCKS5Host(connection, request[3])
	if err != nil {
		_, _ = connection.Write(socks5Reply(8))
		return "", err
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(connection, port); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), nil
}

func readSOCKS5Host(connection net.Conn, addressType byte) (string, error) {
	switch addressType {
	case 1:
		address := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(connection, address); err != nil {
			return "", err
		}
		return net.IP(address).String(), nil
	case 3:
		length := []byte{0}
		if _, err := io.ReadFull(connection, length); err != nil {
			return "", err
		}
		address := make([]byte, int(length[0]))
		if _, err := io.ReadFull(connection, address); err != nil {
			return "", err
		}
		if len(address) == 0 {
			return "", errors.New("ssh: empty SOCKS hostname")
		}
		return string(address), nil
	case 4:
		address := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(connection, address); err != nil {
			return "", err
		}
		return net.IP(address).String(), nil
	default:
		return "", fmt.Errorf("ssh: unsupported SOCKS address type %d", addressType)
	}
}

func socks5Reply(code byte) []byte {
	return []byte{5, code, 0, 1, 0, 0, 0, 0, 0, 0}
}

// Client implements ForwardTransport for SSH remote forwarding.
func (c *Client) Dial(network, address string) (net.Conn, error) {
	if c == nil || c.conn == nil {
		return nil, errNotConnected
	}
	return c.conn.Dial(network, address)
}

// Listen implements ForwardTransport for SSH remote forwarding.
func (c *Client) Listen(network, address string) (net.Listener, error) {
	if c == nil || c.conn == nil {
		return nil, errNotConnected
	}
	return c.conn.Listen(network, address)
}

type forwardType = ForwardType
type forwardSpec = ForwardSpec

const (
	forwardLocal   = ForwardLocal
	forwardRemote  = ForwardRemote
	forwardDynamic = ForwardDynamic
)

func startForward(ctx context.Context, transport ForwardTransport, spec ForwardSpec) (*ForwardSession, error) {
	return StartForward(ctx, transport, spec)
}
