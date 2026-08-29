package gui

import (
	"context"
	"testing"

	"s12ryt-ssh/internal/remote"
)

func TestNewModelOpensRemoteLogin(t *testing.T) {
	model := NewModel(nil)
	if model.Screen != ScreenRemoteLogin {
		t.Fatalf("initial screen: got %v", model.Screen)
	}
	if model.Status == "" {
		t.Fatal("initial status is empty")
	}
}

type fakeRemoteSession struct {
	logoutCount      int
	fingerprintCalls []string
}

func (s *fakeRemoteSession) Account() remote.Account {
	return remote.Account{ID: "account-1", Username: "remote-alice"}
}

func (s *fakeRemoteSession) ResourcesOverview(context.Context) (remote.ResourcesOverview, error) {
	return remote.ResourcesOverview{}, nil
}

func (s *fakeRemoteSession) SSHHosts(context.Context) ([]remote.SSHHost, error) {
	return []remote.SSHHost{{ID: "host-1", Name: "web", Host: "web.example.com", Port: 22, Username: "deploy"}}, nil
}

func (s *fakeRemoteSession) CreateSSHHost(context.Context, remote.SSHHostInput) (remote.SSHHost, error) {
	return remote.SSHHost{ID: "host-2"}, nil
}

func (s *fakeRemoteSession) UpdateSSHHost(context.Context, string, remote.SSHHostInput) (remote.SSHHost, error) {
	return remote.SSHHost{ID: "host-1"}, nil
}

func (s *fakeRemoteSession) DeleteSSHHost(context.Context, string) error {
	return nil
}

func (s *fakeRemoteSession) SSHHostCredentials(context.Context, string) (remote.SSHHostCredentials, error) {
	return remote.SSHHostCredentials{ID: "host-1", Host: "web.example.com", Port: 22, Username: "deploy"}, nil
}

func (s *fakeRemoteSession) SetSSHHostFingerprint(_ context.Context, _, fingerprint string) error {
	s.fingerprintCalls = append(s.fingerprintCalls, fingerprint)
	return nil
}

func (s *fakeRemoteSession) Logout(context.Context) error {
	s.logoutCount++
	return nil
}

func TestModelRemoteWorkspaceExcludesSSHAndReturnsToRemoteLogin(t *testing.T) {
	model := NewModel(nil)
	session := &fakeRemoteSession{}
	model.SetRemoteSession(session, false)
	if model.Screen != ScreenRemoteWorkspace || model.SSHEnabled || model.RemoteAccountName != "remote-alice" {
		t.Fatalf("remote workspace state = %+v", model)
	}
	if err := model.LogoutRemote(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.logoutCount != 1 || model.Screen != ScreenRemoteLogin || model.RemoteSession != nil {
		t.Fatalf("remote logout state = %+v, logout count = %d", model, session.logoutCount)
	}
}

func TestModelRemoteWorkspaceEnablesSSHByAccountFlag(t *testing.T) {
	model := NewModel(nil)
	model.SetRemoteSession(&fakeRemoteSession{}, true)
	if !model.SSHEnabled {
		t.Fatal("ssh enabled flag not stored")
	}
	model.LogoutRemote(context.Background())
	if model.SSHEnabled {
		t.Fatal("ssh enabled flag not reset on logout")
	}
	model.SetRemoteSession(&fakeRemoteSession{}, true)
	if !model.SSHEnabled {
		t.Fatal("ssh enabled flag not stored after re-login")
	}
}
