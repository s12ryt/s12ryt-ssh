package remote

import (
	"context"
	"testing"
)

func TestSessionSSHKeyIdentityLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	created, err := session.CreateSSHKeyIdentity(ctx, SSHKeyIdentityInput{
		Name:          "production-deploy",
		PublicKey:     "ssh-ed25519 public",
		Fingerprint:   "SHA256:key",
		PrivateKey:    "private-material",
		KeyPassphrase: "passphrase",
		Enabled:       boolPointer(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "key-1" || created.Name != "production-deploy" ||
		created.PublicKey != "ssh-ed25519 public" ||
		created.Fingerprint != "SHA256:key" || !created.HasPassphrase ||
		!created.Enabled || created.Version != 1 {
		t.Fatalf("created key identity = %+v", created)
	}

	listed, err := session.SSHKeyIdentities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed key identities = %+v", listed)
	}

	secrets, err := session.SSHKeyIdentitySecrets(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.PrivateKey != "private-material" || secrets.KeyPassphrase != "passphrase" {
		t.Fatalf("key identity secrets = %+v", secrets)
	}

	updated, err := session.UpdateSSHKeyIdentity(ctx, created.ID, SSHKeyIdentityInput{
		Name:        "production-disabled",
		PublicKey:   "ssh-ed25519 updated",
		Fingerprint: "SHA256:updated",
		Enabled:     boolPointer(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "production-disabled" || updated.Version != 2 || updated.Enabled {
		t.Fatalf("updated key identity = %+v", updated)
	}

	if err := session.DeleteSSHKeyIdentity(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = session.SSHKeyIdentities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("key identities after delete = %+v", listed)
	}
}
