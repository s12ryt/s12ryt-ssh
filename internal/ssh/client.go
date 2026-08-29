package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"s12ryt-ssh/internal/config"

	gossh "golang.org/x/crypto/ssh"
)

// Dialer abstracts net connection establishment so it can be customised
// (e.g. tighter timeouts) and replaced in tests.
type Dialer interface {
	Dial(network, address string, timeout time.Duration) (net.Conn, error)
}

// defaultDialer uses the standard net package.
type defaultDialer struct{}

func (defaultDialer) Dial(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

// fastDialer uses a very short timeout (used in tests to fail fast).
type fastDialer struct {
	timeout time.Duration
}

func (f *fastDialer) Dial(network, address string, _ time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, f.timeout)
}

// Client wraps an SSH session against a single host.
type Client struct {
	profile         config.SSHProfile
	dialer          Dialer
	conn            *gossh.Client
	timeout         time.Duration
	hostKeyCallback gossh.HostKeyCallback
}

// Terminal is an interactive SSH PTY session. Its Read and Write methods
// expose the remote terminal stream, while Resize forwards window changes.
type Terminal struct {
	session   *gossh.Session
	stdin     io.WriteCloser
	stdout    io.Reader
	done      chan struct{}
	closeCh   chan struct{}
	closeOnce sync.Once
	waitMu    sync.Mutex
	waitErr   error
}

// ErrHostKeyNotTrusted means that no explicit host key fingerprint was
// configured for the remote host.
var ErrHostKeyNotTrusted = errors.New("ssh: host key is not trusted")

// ErrHostKeyMismatch means that the server key differs from the configured
// fingerprint.
var ErrHostKeyMismatch = errors.New("ssh: host key fingerprint mismatch")

// NewClient creates a Client for the given SSH profile.
func NewClient(p config.SSHProfile) *Client {
	return &Client{profile: p, dialer: defaultDialer{}, timeout: 10 * time.Second}
}

// SetDialer overrides the connection dialer (used for testing / tuning).
func (c *Client) SetDialer(d Dialer) { c.dialer = d }

// SetHostKeyCallback sets the verifier used during the next connection. The
// GUI uses this for an explicit first-connection confirmation.
func (c *Client) SetHostKeyCallback(callback gossh.HostKeyCallback) {
	c.hostKeyCallback = callback
}

// SetTimeout overrides the dial + operation timeout.
func (c *Client) SetTimeout(d time.Duration) { c.timeout = d }

// buildClientConfig constructs the ssh.ClientConfig from a profile.
func buildClientConfig(p config.SSHProfile) *gossh.ClientConfig {
	cfg, _ := buildClientConfigErr(p)
	return cfg
}

func buildClientConfigErr(p config.SSHProfile) (*gossh.ClientConfig, error) {
	cfg := &gossh.ClientConfig{
		User:            p.User,
		HostKeyCallback: trustedHostKeyCallback(p.HostKeyFingerprint),
		Timeout:         10 * time.Second,
	}
	if p.KeyPath != "" {
		auth, err := loadKeyAuth(p.KeyPath, p.KeyPassphrase)
		if err != nil {
			return cfg, fmt.Errorf("ssh: load private key: %w", err)
		}
		cfg.Auth = append(cfg.Auth, auth)
	} else if p.KeyData != "" {
		auth, err := keyDataAuth([]byte(p.KeyData), p.KeyPassphrase)
		if err != nil {
			return cfg, fmt.Errorf("ssh: parse private key: %w", err)
		}
		cfg.Auth = append(cfg.Auth, auth)
	}
	if p.Password != "" {
		cfg.Auth = append(cfg.Auth, gossh.Password(p.Password))
	}
	if len(cfg.Auth) == 0 {
		return cfg, errors.New("ssh: no authentication method configured")
	}
	return cfg, nil
}

// Connect establishes the SSH connection.
func (c *Client) Connect() error {
	addr := net.JoinHostPort(c.profile.Host, itoa(c.profile.Port))
	conn, err := c.dialer.Dial("tcp", addr, c.timeout)
	if err != nil {
		return err
	}
	cfg, err := buildClientConfigErr(c.profile)
	if err != nil {
		conn.Close()
		return err
	}
	if c.hostKeyCallback != nil {
		cfg.HostKeyCallback = c.hostKeyCallback
	}
	cfg.Timeout = c.timeout
	sshConn, chans, reqs, err := gossh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return err
	}
	c.conn = gossh.NewClient(sshConn, chans, reqs)
	return nil
}

// Exec runs a command on the remote host and returns combined output.
func (c *Client) Exec(cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.ExecContext(ctx, cmd)
}

// ExecContext runs a command and closes the session when ctx is cancelled.
func (c *Client) ExecContext(ctx context.Context, cmd string) (string, error) {
	if c.conn == nil {
		return "", errNotConnected
	}
	if ctx == nil {
		return "", errors.New("ssh: nil context")
	}
	session, err := c.conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := session.CombinedOutput(cmd)
		done <- result{out: out, err: err}
	}()
	select {
	case result := <-done:
		return string(result.out), result.err
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	}
}

// OpenPTY opens an interactive shell with a pseudo-terminal of the requested
// size. Cancelling ctx closes the terminal session.
func (c *Client) OpenPTY(ctx context.Context, width, height int) (*Terminal, error) {
	if c.conn == nil {
		return nil, errNotConnected
	}
	if ctx == nil {
		return nil, errors.New("ssh: nil context")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("ssh: invalid terminal size")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return nil, err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = session.Close()
		return nil, err
	}
	if err := session.RequestPty("xterm", height, width, gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = stdin.Close()
		_ = session.Close()
		return nil, err
	}
	if err := session.Shell(); err != nil {
		_ = stdin.Close()
		_ = session.Close()
		return nil, err
	}

	terminal := &Terminal{
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		done:    make(chan struct{}),
		closeCh: make(chan struct{}),
	}
	go func() {
		err := session.Wait()
		terminal.waitMu.Lock()
		terminal.waitErr = err
		terminal.waitMu.Unlock()
		close(terminal.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = terminal.Close()
		case <-terminal.done:
		case <-terminal.closeCh:
		}
	}()
	return terminal, nil
}

// Read reads bytes emitted by the remote terminal.
func (t *Terminal) Read(p []byte) (int, error) {
	if t == nil || t.stdout == nil {
		return 0, io.ErrClosedPipe
	}
	return t.stdout.Read(p)
}

// Write sends bytes to the remote terminal.
func (t *Terminal) Write(p []byte) (int, error) {
	if t == nil || t.stdin == nil {
		return 0, io.ErrClosedPipe
	}
	return t.stdin.Write(p)
}

// Resize changes the remote PTY window size.
func (t *Terminal) Resize(width, height int) error {
	if t == nil || t.session == nil {
		return io.ErrClosedPipe
	}
	if width <= 0 || height <= 0 {
		return errors.New("ssh: invalid terminal size")
	}
	return t.session.WindowChange(height, width)
}

// Wait waits until the remote terminal exits.
func (t *Terminal) Wait() error {
	if t == nil {
		return io.ErrClosedPipe
	}
	<-t.done
	t.waitMu.Lock()
	defer t.waitMu.Unlock()
	return t.waitErr
}

// Close closes the terminal input and SSH session.
func (t *Terminal) Close() error {
	if t == nil {
		return nil
	}
	var err error
	t.closeOnce.Do(func() {
		close(t.closeCh)
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.session != nil {
			err = t.session.Close()
		}
	})
	return err
}

func trustedHostKeyCallback(expected string) gossh.HostKeyCallback {
	expected = strings.TrimSpace(expected)
	return func(hostname string, _ net.Addr, key gossh.PublicKey) error {
		actual := gossh.FingerprintSHA256(key)
		if expected == "" {
			return fmt.Errorf("%w: %s (%s)", ErrHostKeyNotTrusted, hostname, actual)
		}
		if expected != actual {
			return fmt.Errorf("%w: expected %s, got %s", ErrHostKeyMismatch, expected, actual)
		}
		return nil
	}
}

// Close releases the SSH connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
