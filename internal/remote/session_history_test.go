package remote

import (
	"context"
	"testing"
)

func TestSessionSSHSessionHistoryLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	host, err := session.CreateSSHHost(ctx, SSHHostInput{
		Name: "web", Host: "web.example.com", Port: 22,
		Username: "deploy", Password: "long-password",
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := session.CreateSSHSessionHistory(ctx, SSHSessionHistoryInput{
		HostID: host.ID,
		Status: SSHSessionConnecting,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "history-1" || created.HostID != host.ID || created.HostName != host.Name {
		t.Fatalf("created history = %+v", created)
	}
	if created.Status != SSHSessionConnecting || created.LatencyMS != 0 || created.EndedAt != nil {
		t.Fatalf("created state = %+v", created)
	}

	connected, err := session.UpdateSSHSessionHistory(ctx, created.ID, SSHSessionHistoryUpdate{
		Status:    SSHSessionConnected,
		LatencyMS: intPointer(42),
	})
	if err != nil {
		t.Fatal(err)
	}
	if connected.Status != SSHSessionConnected || connected.LatencyMS != 42 || connected.EndedAt != nil {
		t.Fatalf("connected state = %+v", connected)
	}

	closed, err := session.UpdateSSHSessionHistory(ctx, created.ID, SSHSessionHistoryUpdate{
		Status: SSHSessionClosed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != SSHSessionClosed || closed.LatencyMS != 42 || closed.EndedAt == nil {
		t.Fatalf("closed state = %+v", closed)
	}

	history, err := session.SSHSessionHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != created.ID || history[0].ErrorMessage != "" {
		t.Fatalf("history = %+v", history)
	}
}

func intPointer(value int) *int {
	return &value
}
