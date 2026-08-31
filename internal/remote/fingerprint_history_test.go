package remote

import (
	"context"
	"testing"
)

func TestSessionSSHHostFingerprintHistoryLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	host, err := session.CreateSSHHost(ctx, SSHHostInput{
		Name: "web", Host: "web.example.com", Port: 22, Username: "deploy",
		Password: "long-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.SetSSHHostFingerprint(ctx, host.ID, "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := session.SetSSHHostFingerprintWithSource(
		ctx,
		host.ID,
		"MD5:aa:bb:cc",
		SSHHostFingerprintManual,
	); err != nil {
		t.Fatal(err)
	}

	history, err := session.SSHHostFingerprints(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("fingerprint history = %+v", history)
	}
	var current, retired *SSHHostFingerprint
	for index := range history {
		entry := &history[index]
		if entry.Active {
			current = entry
		} else {
			retired = entry
		}
	}
	if current == nil || current.Fingerprint != "MD5:aa:bb:cc" ||
		current.Algorithm != "MD5" || current.Source != SSHHostFingerprintManual {
		t.Fatalf("current fingerprint = %+v", current)
	}
	if retired == nil || retired.Fingerprint != "SHA256:first" ||
		retired.Source != SSHHostFingerprintTOFU || retired.RetiredAt == nil {
		t.Fatalf("retired fingerprint = %+v", retired)
	}

	if err := session.ClearSSHHostFingerprint(ctx, host.ID); err != nil {
		t.Fatal(err)
	}
	history, err = session.SSHHostFingerprints(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range history {
		if entry.Active || entry.RetiredAt == nil {
			t.Fatalf("fingerprint remained active after clear: %+v", entry)
		}
	}
}
