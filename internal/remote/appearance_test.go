package remote

import (
	"context"
	"strings"
	"testing"
)

func TestSessionSSHWorkspacePreferencesLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	preferences, err := session.SSHWorkspacePreferences(ctx)
	if err != nil {
		t.Fatalf("get workspace preferences: %v", err)
	}
	if preferences.TerminalAppearance.Font != SSHTerminalFontBuiltin || preferences.TerminalAppearance.FontSize != 13 || preferences.Version != 1 {
		t.Fatalf("default preferences = %+v", preferences)
	}

	updated, err := session.UpdateSSHWorkspacePreferences(ctx, SSHWorkspacePreferencesInput{
		TerminalAppearance: SSHTerminalAppearance{
			Font:       SSHTerminalFontSystem,
			FontSize:   17,
			Foreground: "#f0f0f0",
			Background: "#080808",
		},
	})
	if err != nil {
		t.Fatalf("update workspace preferences: %v", err)
	}
	if updated.Version != 2 || updated.TerminalAppearance.Font != SSHTerminalFontSystem {
		t.Fatalf("updated preferences = %+v", updated)
	}
	body := fixture.lastBody()
	if !strings.Contains(body, `"fontSize":17`) || !strings.Contains(body, `"foreground":"#f0f0f0"`) {
		t.Fatalf("preference request body = %s", body)
	}
}

func TestSSHHostInputSerializesTerminalAppearanceClearIntent(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	created, err := session.CreateSSHHost(context.Background(), SSHHostInput{
		Name: "web", Host: "web.example.com", Port: 22, Username: "deploy", Password: "secret",
		Settings: &SSHConnectionSettings{TerminalAppearance: &SSHTerminalAppearanceOverride{FontSize: 18}},
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	_, err = session.UpdateSSHHost(context.Background(), created.ID, SSHHostInput{
		Name: "web", Host: "web.example.com", Port: 22, Username: "deploy",
		Settings: &SSHConnectionSettings{Compression: true}, ClearTerminalAppearance: true,
	})
	if err != nil {
		t.Fatalf("clear host appearance: %v", err)
	}
	body := fixture.lastBody()
	if !strings.Contains(body, `"clearTerminalAppearance":true`) {
		t.Fatalf("host update body = %s", body)
	}
}
