package ssh

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

type testForwardTransport struct{}

func (testForwardTransport) Dial(network, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

func (testForwardTransport) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func TestStartForwardCopiesTrafficAndReportsCounters(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(io.LimitReader(conn, 4))
		_, _ = conn.Write([]byte("pong"))
		_ = data
	}()

	forward, err := startForward(context.Background(), testForwardTransport{}, forwardSpec{
		Type:       forwardLocal,
		ListenHost: "127.0.0.1",
		ListenPort: 0,
		TargetHost: "127.0.0.1",
		TargetPort: target.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()

	conn, err := net.Dial("tcp", forward.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if string(response) != "pong" {
		t.Fatalf("response = %q", response)
	}

	deadline := time.Now().Add(time.Second)
	for {
		up, down := forward.Traffic()
		if up == 4 && down == 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("traffic = %d up, %d down", up, down)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartDynamicForwardSpeaksSOCKS5(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		data := make([]byte, 4)
		_, _ = io.ReadFull(conn, data)
		_, _ = conn.Write([]byte("pong"))
	}()

	forward, err := startForward(context.Background(), testForwardTransport{}, forwardSpec{
		Type:       forwardDynamic,
		ListenHost: "127.0.0.1",
		ListenPort: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()

	conn, err := net.Dial("tcp", forward.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, methodReply); err != nil {
		t.Fatal(err)
	}
	if string(methodReply) != string([]byte{5, 0}) {
		t.Fatalf("method reply = %v", methodReply)
	}
	targetAddress := target.Addr().(*net.TCPAddr)
	request := []byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 0}
	binary.BigEndian.PutUint16(request[8:], uint16(targetAddress.Port))
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	connectReply := make([]byte, 10)
	if _, err := io.ReadFull(conn, connectReply); err != nil {
		t.Fatal(err)
	}
	if connectReply[1] != 0 {
		t.Fatalf("SOCKS connect reply = %v", connectReply)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "pong" {
		t.Fatalf("response = %q", response)
	}
}

func TestStartForwardRejectsInvalidSpecAndClosesCleanly(t *testing.T) {
	if _, err := startForward(context.Background(), testForwardTransport{}, forwardSpec{Type: forwardLocal, ListenPort: 1}); err == nil {
		t.Fatal("invalid local target was accepted")
	}
	if _, err := startForward(nil, testForwardTransport{}, forwardSpec{Type: forwardLocal}); err == nil {
		t.Fatal("nil context was accepted")
	}
}
