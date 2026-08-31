package remote

import (
	"context"
	"testing"
)

func TestSessionSSHTunnelRuleLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	created, err := session.CreateSSHTunnel(ctx, SSHTunnelInput{
		Name:       "web-local",
		HostID:     "host-1",
		Type:       SSHTunnelLocal,
		ListenHost: "127.0.0.1",
		ListenPort: 18080,
		TargetHost: "web.internal",
		TargetPort: 8080,
		Enabled:    true,
		AutoStart:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "tunnel-1" || created.Type != SSHTunnelLocal || !created.Enabled || !created.AutoStart {
		t.Fatalf("created tunnel = %+v", created)
	}
	if created.Running || created.TrafficUpBytes != 0 || created.TrafficDownBytes != 0 || created.Version != 1 {
		t.Fatalf("created runtime state = %+v", created)
	}

	rules, err := session.SSHTunnels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].ID != created.ID {
		t.Fatalf("listed tunnels = %+v", rules)
	}

	updated, err := session.UpdateSSHTunnel(ctx, created.ID, SSHTunnelInput{
		Name:       "web-remote",
		HostID:     "host-1",
		Type:       SSHTunnelRemote,
		ListenHost: "0.0.0.0",
		ListenPort: 19090,
		TargetHost: "127.0.0.1",
		TargetPort: 9090,
		Enabled:    true,
		AutoStart:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "web-remote" || updated.Type != SSHTunnelRemote || updated.Version != 2 {
		t.Fatalf("updated tunnel = %+v", updated)
	}

	runtimeUpdated, err := session.UpdateSSHTunnelRuntime(ctx, created.ID, SSHTunnelRuntimeUpdate{
		Running:          true,
		TrafficUpBytes:   4096,
		TrafficDownBytes: 8192,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeUpdated.Running || runtimeUpdated.TrafficUpBytes != 4096 || runtimeUpdated.TrafficDownBytes != 8192 {
		t.Fatalf("updated runtime = %+v", runtimeUpdated)
	}
	if runtimeUpdated.Version != updated.Version {
		t.Fatalf("runtime update changed config version: got %d want %d", runtimeUpdated.Version, updated.Version)
	}

	if err := session.DeleteSSHTunnel(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	rules, err = session.SSHTunnels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("tunnels after delete = %+v", rules)
	}
}
