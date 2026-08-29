package gui

import (
	"bytes"
	"context"
	"io"
	"testing"

	"s12ryt-ssh/internal/remote"
)

func TestNewModelOpensRemoteLogin(t *testing.T) {
	model := NewModel(nil)
	if model.Screen != ScreenRemoteLogin {
		t.Fatalf("initial screen: got %v", model.Screen)
	}
	if model.Tab != TabStorage {
		t.Fatalf("initial tab: got %v", model.Tab)
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

func (s *fakeRemoteSession) ListObjects(context.Context, string, string) ([]remote.S3Object, error) {
	return nil, nil
}

func (s *fakeRemoteSession) UploadObject(context.Context, string, string, io.ReadSeeker, int64) (remote.UploadResult, error) {
	return remote.UploadResult{}, nil
}

func (s *fakeRemoteSession) DownloadObject(context.Context, string, string) (remote.Download, error) {
	return remote.Download{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (s *fakeRemoteSession) DeleteObject(context.Context, string, string) error {
	return nil
}

func (s *fakeRemoteSession) Tables(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *fakeRemoteSession) Query(context.Context, string, string, []any) (remote.SQLQueryResult, error) {
	return remote.SQLQueryResult{}, nil
}

func (s *fakeRemoteSession) Exec(context.Context, string, string, []any) (remote.SQLExecResult, error) {
	return remote.SQLExecResult{}, nil
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
	if model.Screen != ScreenRemoteWorkspace || model.Tab != TabStorage || model.RemoteAccountName != "remote-alice" {
		t.Fatalf("remote workspace state = %+v", model)
	}
	model.SelectTab(TabSSH)
	if model.Tab != TabStorage {
		t.Fatalf("remote workspace accepted SSH tab: %v", model.Tab)
	}
	model.SelectTab(TabDatabase)
	if model.Tab != TabDatabase {
		t.Fatalf("remote database tab = %v", model.Tab)
	}
	if err := model.LogoutRemote(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.logoutCount != 1 || model.Screen != ScreenRemoteLogin || model.RemoteSession != nil {
		t.Fatalf("remote logout state = %+v, logout count = %d", model, session.logoutCount)
	}
}

func TestModelRemoteWorkspaceEnablesSSHTabByAccountFlag(t *testing.T) {
	model := NewModel(nil)
	model.SetRemoteSession(&fakeRemoteSession{}, true)
	if !model.SSHEnabled {
		t.Fatal("ssh enabled flag not stored")
	}
	model.SelectTab(TabSSH)
	if model.Tab != TabSSH {
		t.Fatalf("ssh tab rejected while enabled: %v", model.Tab)
	}
	model.LogoutRemote(context.Background())
	if model.SSHEnabled {
		t.Fatal("ssh enabled flag not reset on logout")
	}
	model.SetRemoteSession(&fakeRemoteSession{}, true)
	model.SelectTab(TabSSH)
	if model.Tab != TabSSH {
		t.Fatalf("ssh tab rejected after re-login: %v", model.Tab)
	}
}
