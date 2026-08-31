package remote

import (
	"context"
	"testing"
)

func boolPointer(value bool) *bool {
	return &value
}

func TestSessionSSHCommandSnippetLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	created, err := session.CreateSSHCommandSnippet(ctx, SSHCommandSnippetInput{
		Name:      "deploy",
		Command:   "kubectl rollout restart deployment ${SERVICE}",
		Variables: []string{"SERVICE"},
		Secrets:   map[string]string{"TOKEN": "secret-token"},
		Enabled:   boolPointer(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "snippet-1" || created.Name != "deploy" || !created.Enabled || created.Version != 1 {
		t.Fatalf("created snippet = %+v", created)
	}
	if len(created.Variables) != 1 || created.Variables[0] != "SERVICE" {
		t.Fatalf("created variables = %+v", created.Variables)
	}
	if len(created.SecretNames) != 1 || created.SecretNames[0] != "TOKEN" {
		t.Fatalf("created secret names = %+v", created.SecretNames)
	}

	listed, err := session.SSHCommandSnippets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed snippets = %+v", listed)
	}

	secrets, err := session.SSHCommandSnippetSecrets(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secrets["TOKEN"] != "secret-token" {
		t.Fatalf("snippet secrets = %+v", secrets)
	}

	updated, err := session.UpdateSSHCommandSnippet(ctx, created.ID, SSHCommandSnippetInput{
		Name:    "deploy disabled",
		Command: "echo ${SERVICE}",
		Secrets: map[string]string{"TOKEN": "rotated-token"},
		Enabled: boolPointer(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "deploy disabled" || updated.Command != "echo ${SERVICE}" || updated.Enabled || updated.Version != 2 {
		t.Fatalf("updated snippet = %+v", updated)
	}

	secrets, err = session.SSHCommandSnippetSecrets(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secrets["TOKEN"] != "rotated-token" {
		t.Fatalf("updated snippet secrets = %+v", secrets)
	}

	if err := session.DeleteSSHCommandSnippet(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = session.SSHCommandSnippets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("snippets after delete = %+v", listed)
	}
}
