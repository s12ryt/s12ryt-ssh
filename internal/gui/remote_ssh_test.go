package gui

import (
	"errors"
	"fmt"
	"testing"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

func TestSSHProfileFromCredentialsMapsFields(t *testing.T) {
	creds := remote.SSHHostCredentials{
		ID: "host-1", Name: "web", Host: "web.example.com", Port: 2222,
		Username: "deploy", Password: "secret", PrivateKey: "-----BEGIN KEY-----",
		KeyPassphrase: "phrase", TrustedFingerprint: "SHA256:abc",
	}
	profile := sshProfileFromCredentials(creds)
	if profile.Name != "web" || profile.Host != "web.example.com" || profile.Port != 2222 || profile.User != "deploy" {
		t.Fatalf("profile fields = %+v", profile)
	}
	if profile.Password != "secret" || profile.KeyData != "-----BEGIN KEY-----" || profile.KeyPassphrase != "phrase" {
		t.Fatalf("profile credentials = %+v", profile)
	}
	if profile.HostKeyFingerprint != "SHA256:abc" {
		t.Fatalf("fingerprint = %q", profile.HostKeyFingerprint)
	}
}

func TestParsePendingFingerprintExtractsActualFingerprint(t *testing.T) {
	err := fmt.Errorf("%w: %s (%s)", sshclient.ErrHostKeyNotTrusted, "web.example.com", "SHA256:xyz")
	fp, ok := parsePendingFingerprint(err)
	if !ok || fp != "SHA256:xyz" {
		t.Fatalf("parse = %q ok=%v", fp, ok)
	}
	if _, ok := parsePendingFingerprint(errors.New("other error")); ok {
		t.Fatal("unrelated error parsed as host key")
	}
	mismatch := fmt.Errorf("%w: expected %s, got %s", sshclient.ErrHostKeyMismatch, "SHA256:a", "SHA256:b")
	if _, ok := parsePendingFingerprint(mismatch); ok {
		t.Fatal("mismatch error must not trigger confirmation")
	}
}

func TestSSHHostInputFromFormValidates(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshName.SetText("web")
	ui.sshHost.SetText("web.example.com")
	ui.sshPort.SetText("2222")
	ui.sshUser.SetText("deploy")
	ui.sshPassword.SetText("secret")
	input, err := ui.sshHostInputFromForm()
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "web" || input.Host != "web.example.com" || input.Port != 2222 {
		t.Fatalf("input = %+v", input)
	}
	if input.Username != "deploy" || input.Password != "secret" {
		t.Fatalf("input credentials = %+v", input)
	}
	ui.sshPort.SetText("0")
	if _, err := ui.sshHostInputFromForm(); err == nil {
		t.Fatal("port 0 accepted")
	}
}

func TestSSHHostInputRequiresCredentialsForNewHost(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshName.SetText("web")
	ui.sshHost.SetText("web.example.com")
	ui.sshPort.SetText("22")
	ui.sshUser.SetText("deploy")
	if _, err := ui.sshHostInputFromForm(); err == nil {
		t.Fatal("new host without credentials accepted")
	}
	ui.sshHostID = "host-1"
	if _, err := ui.sshHostInputFromForm(); err != nil {
		t.Fatalf("edit with empty credentials rejected: %v", err)
	}
}

func TestApplyAndSelectSSHHosts(t *testing.T) {
	ui := NewWindow(nil)
	ui.applySSHHosts([]remote.SSHHost{
		{ID: "host-1", Name: "web", Host: "web.example.com", Port: 22, Username: "deploy", TrustedFingerprint: "SHA256:abc"},
		{ID: "host-2", Name: "db", Host: "db.example.com", Port: 2222, Username: "root"},
	})
	if len(ui.sshHosts) != 2 || len(ui.sshHostButtons) != 2 {
		t.Fatalf("hosts = %d buttons = %d", len(ui.sshHosts), len(ui.sshHostButtons))
	}
	ui.selectSSHHost(1)
	if ui.sshHostID != "host-2" || ui.sshHostIndex != 1 {
		t.Fatalf("selection = id %q index %d", ui.sshHostID, ui.sshHostIndex)
	}
	if ui.sshName.Text() != "db" || ui.sshHost.Text() != "db.example.com" || ui.sshPort.Text() != "2222" || ui.sshUser.Text() != "root" {
		t.Fatalf("form = name %q host %q port %q user %q", ui.sshName.Text(), ui.sshHost.Text(), ui.sshPort.Text(), ui.sshUser.Text())
	}
	ui.clearSSHHostForm()
	if ui.sshHostID != "" || ui.sshHostIndex != -1 || ui.sshName.Text() != "" {
		t.Fatalf("cleared form = id %q index %d name %q", ui.sshHostID, ui.sshHostIndex, ui.sshName.Text())
	}
}
