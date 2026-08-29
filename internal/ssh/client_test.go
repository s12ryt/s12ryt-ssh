package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/config"

	gossh "golang.org/x/crypto/ssh"
)

// pipeDialer returns net.Pipe ends instead of opening TCP connections, so the
// SSH client and an in-process server talk over an in-memory pipe. This avoids
// host networking quirks (e.g. WSL2 loopback) in tests.
type pipeDialer struct {
	serverCfg *gossh.ServerConfig
}

func (p *pipeDialer) Dial(string, string, time.Duration) (net.Conn, error) {
	clientConn, serverConn := bufferedPipe()
	go servePipe(serverConn, p.serverCfg)
	return clientConn, nil
}

// bufferedPipe returns a connected pair of net.Conns backed by net.Pipe but
// with asynchronous (buffered) writes, so the SSH version exchange — where
// both sides write before reading — does not deadlock.
func bufferedPipe() (net.Conn, net.Conn) {
	a, b := net.Pipe()
	return newAsyncConn(a), newAsyncConn(b)
}

type asyncConn struct {
	net.Conn
	writeCh   chan []byte
	done      chan struct{}
	closeOnce sync.Once
	closed    chan struct{}
}

func (c *asyncConn) Write(b []byte) (int, error) {
	buf := make([]byte, len(b))
	copy(buf, b)
	select {
	case c.writeCh <- buf:
		return len(b), nil
	case <-c.closed:
		return 0, net.ErrClosed
	}
}

func (c *asyncConn) Close() error {
	c.closeOnce.Do(func() { close(c.writeCh) })
	<-c.done
	return c.Conn.Close()
}

func newAsyncConn(inner net.Conn) *asyncConn {
	c := &asyncConn{
		Conn:    inner,
		writeCh: make(chan []byte, 512),
		done:    make(chan struct{}),
		closed:  make(chan struct{}),
	}
	go func() {
		defer close(c.done)
		for buf := range c.writeCh {
			if _, err := c.Conn.Write(buf); err != nil {
				close(c.closed)
				return
			}
		}
	}()
	return c
}

// servePipe runs a single SSH server session over a pipe connection.
func servePipe(c net.Conn, cfg *gossh.ServerConfig) {
	defer c.Close()
	sconn, chans, reqs, err := gossh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer sconn.Close()
	go gossh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(gossh.UnknownChannelType, "unknown")
			continue
		}
		channel, chanReqs, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func(ch gossh.Channel, in <-chan *gossh.Request) {
			defer ch.Close()
			interactive := false
			for req := range in {
				switch req.Type {
				case "pty-req", "window-change":
					req.Reply(true, nil)
				case "shell":
					req.Reply(true, nil)
					if !interactive {
						interactive = true
						go func() {
							buf := make([]byte, 128)
							for {
								n, err := ch.Read(buf)
								if n > 0 {
									line := strings.TrimSuffix(string(buf[:n]), "\n")
									_, _ = ch.Write([]byte("echo:" + line))
								}
								if err != nil {
									return
								}
							}
						}()
					}
				case "exec":
					req.Reply(true, nil)
					command := strings.TrimSpace(parseString(req.Payload))
					if command == "hang" {
						// Keep the remote command alive long enough for the client
						// context to cancel it instead of returning on stdin EOF.
						time.Sleep(2 * time.Second)
						return
					}
					ch.Write([]byte("echo:" + command))
					ch.SendRequest("exit-status", false, gossh.Marshal(struct{ Code uint32 }{0}))
					return
				default:
					req.Reply(false, nil)
				}
			}
		}(channel, chanReqs)
	}
}

// newServerConfig builds a ServerConfig that accepts password "secret" and,
// if authorizedPubKey is non-nil, that public key.
func newServerConfig(t *testing.T, authorizedPubKey gossh.PublicKey) *gossh.ServerConfig {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	cfg := &gossh.ServerConfig{
		PasswordCallback: func(c gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if string(password) == "secret" {
				return nil, nil
			}
			return nil, io.ErrUnexpectedEOF
		},
		PublicKeyCallback: func(c gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			if authorizedPubKey != nil && bytes.Equal(key.Marshal(), authorizedPubKey.Marshal()) {
				return nil, nil
			}
			return nil, io.ErrUnexpectedEOF
		},
	}
	cfg.AddHostKey(signer)
	return cfg
}

// parseString decodes an SSH string (uint32 length prefix + bytes) from b.
func parseString(b []byte) string {
	if len(b) < 4 {
		return ""
	}
	n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if 4+n > len(b) {
		n = len(b) - 4
	}
	return string(b[4 : 4+n])
}

func TestBuildClientConfig_Password(t *testing.T) {
	p := config.SSHProfile{User: "u", Password: "secret"}
	cfg := buildClientConfig(p)
	if cfg.User != "u" {
		t.Errorf("user: %q", cfg.User)
	}
	if len(cfg.Auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(cfg.Auth))
	}
	if cfg.Timeout <= 0 {
		t.Error("timeout should be set")
	}
	if cfg.HostKeyCallback == nil {
		t.Error("host key callback should be set")
	}
}

func TestBuildClientConfig_KeyAuth(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(keyBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	p := config.SSHProfile{User: "u", KeyPath: keyPath}
	cfg := buildClientConfig(p)
	if len(cfg.Auth) == 0 {
		t.Fatal("expected auth methods from key")
	}
}

func TestBuildClientConfig_RequiresTrustedHostKey(t *testing.T) {
	cfg := buildClientConfig(config.SSHProfile{User: "u"})
	if cfg == nil || cfg.HostKeyCallback == nil {
		t.Fatal("expected a host key callback")
	}
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := gossh.NewPublicKey(signer.Public())
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.HostKeyCallback("host", nil, key); !errors.Is(err, ErrHostKeyNotTrusted) {
		t.Fatalf("expected ErrHostKeyNotTrusted, got %v", err)
	}
}

func TestBuildClientConfig_VerifiesFingerprint(t *testing.T) {
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := gossh.NewPublicKey(signer.Public())
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := gossh.FingerprintSHA256(key)
	cfg := buildClientConfig(config.SSHProfile{User: "u", HostKeyFingerprint: fingerprint})
	if err := cfg.HostKeyCallback("host", nil, key); err != nil {
		t.Fatalf("trusted key rejected: %v", err)
	}
	wrong := buildClientConfig(config.SSHProfile{User: "u", HostKeyFingerprint: "SHA256:not-the-key"})
	if err := wrong.HostKeyCallback("host", nil, key); !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("expected ErrHostKeyMismatch, got %v", err)
	}
}

func TestClient_ConnectAndExec_Password(t *testing.T) {
	cfg := newServerConfig(t, nil)
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u", Password: "secret"})
	c.SetDialer(&pipeDialer{serverCfg: cfg})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	out, err := c.Exec("hello")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.Contains(out, "echo:hello") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestClient_ConnectAndExec_Key(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(keyBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	pub, err := gossh.NewPublicKey(ed25519.PublicKey(priv.Public().(ed25519.PublicKey)))
	if err != nil {
		t.Fatal(err)
	}

	cfg := newServerConfig(t, pub)
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u", KeyPath: keyPath})
	c.SetDialer(&pipeDialer{serverCfg: cfg})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	out, err := c.Exec("hi")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.Contains(out, "echo:hi") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestClient_Connect_BadPassword(t *testing.T) {
	cfg := newServerConfig(t, nil)
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u", Password: "wrong"})
	c.SetDialer(&pipeDialer{serverCfg: cfg})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err == nil {
		c.Close()
		t.Error("expected connect error for wrong password")
	}
}

func TestClient_ExecNotConnected(t *testing.T) {
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u"})
	if _, err := c.Exec("ls"); err == nil {
		t.Error("expected error when exec without connect")
	}
}

func TestClient_ExecContextTimeout(t *testing.T) {
	cfg := newServerConfig(t, nil)
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u", Password: "secret"})
	c.SetDialer(&pipeDialer{serverCfg: cfg})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.ExecContext(ctx, "hang"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestClient_Connect_Timeout(t *testing.T) {
	p := config.SSHProfile{Name: "t", Host: "10.255.255.1", Port: 22, User: "u", Password: "x"}
	c := NewClient(p)
	c.SetDialer(&fastDialer{timeout: 200 * time.Millisecond})
	if err := c.Connect(); err == nil {
		c.Close()
		t.Error("expected timeout error")
	}
}

func TestBuildClientConfig_KeyDataAuth(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	p := config.SSHProfile{User: "u", KeyData: string(pem.EncodeToMemory(keyBytes))}
	cfg := buildClientConfig(p)
	if len(cfg.Auth) == 0 {
		t.Fatal("expected auth methods from inline key data")
	}
}

func TestBuildClientConfig_KeyDataWithPassphrase(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := gossh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("phrase"))
	if err != nil {
		t.Fatal(err)
	}
	pemData := string(pem.EncodeToMemory(keyBytes))
	cfg, err := buildClientConfigErr(config.SSHProfile{
		User:           "u",
		KeyData:        pemData,
		KeyPassphrase:  "phrase",
	})
	if err != nil {
		t.Fatalf("passphrase key: %v", err)
	}
	if len(cfg.Auth) == 0 {
		t.Fatal("expected auth methods from encrypted key data")
	}
	if _, err := buildClientConfigErr(config.SSHProfile{
		User:          "u",
		KeyData:       pemData,
		KeyPassphrase: "wrong",
	}); err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestClient_ConnectAndExec_KeyData(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := gossh.NewPublicKey(ed25519.PublicKey(priv.Public().(ed25519.PublicKey)))
	if err != nil {
		t.Fatal(err)
	}

	cfg := newServerConfig(t, pub)
	c := NewClient(config.SSHProfile{
		Name:    "t",
		Host:    "x",
		Port:    22,
		User:    "u",
		KeyData: string(pem.EncodeToMemory(keyBytes)),
	})
	c.SetDialer(&pipeDialer{serverCfg: cfg})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	out, err := c.Exec("hi")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.Contains(out, "echo:hi") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestClient_OpenPTYInteractive(t *testing.T) {
	c := NewClient(config.SSHProfile{Name: "t", Host: "x", Port: 22, User: "u", Password: "secret"})
	c.SetDialer(&pipeDialer{serverCfg: newServerConfig(t, nil)})
	c.SetHostKeyCallback(gossh.InsecureIgnoreHostKey())
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	terminal, err := c.OpenPTY(context.Background(), 80, 24)
	if err != nil {
		t.Fatalf("OpenPTY: %v", err)
	}
	defer terminal.Close()

	if err := terminal.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := terminal.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	read := make(chan struct {
		value string
		err   error
	}, 1)
	go func() {
		buf := make([]byte, 128)
		n, err := terminal.Read(buf)
		read <- struct {
			value string
			err   error
		}{string(buf[:n]), err}
	}()
	select {
	case result := <-read:
		if result.err != nil {
			t.Fatalf("Read: %v", result.err)
		}
		if !strings.Contains(result.value, "echo:hello") {
			t.Fatalf("unexpected terminal output: %q", result.value)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for terminal output")
	}
}
