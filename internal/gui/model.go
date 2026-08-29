// Package gui contains the Gio presentation layer and its testable state.
package gui

import (
	"context"
	"io"

	"s12ryt-ssh/internal/remote"
)

// Screen identifies the top-level application flow.
type Screen uint8

const (
	ScreenRemoteLogin Screen = iota
	ScreenRemoteWorkspace
)

// Tab identifies a workspace capability.
type Tab uint8

const (
	TabSSH Tab = iota
	TabStorage
	TabDatabase
)

// Model is the GUI state independent of Gio widgets and drawing operations.
type Model struct {
	RemoteService     *remote.Service
	Screen            Screen
	Tab               Tab
	Status            string
	Error             string
	RemoteSession     RemoteSession
	RemoteAccountName string
	SSHEnabled        bool
}

// RemoteSession is the credential-free workspace contract exposed to the GUI.
type RemoteSession interface {
	Account() remote.Account
	ResourcesOverview(context.Context) (remote.ResourcesOverview, error)
	ListObjects(context.Context, string, string) ([]remote.S3Object, error)
	UploadObject(context.Context, string, string, io.ReadSeeker, int64) (remote.UploadResult, error)
	DownloadObject(context.Context, string, string) (remote.Download, error)
	DeleteObject(context.Context, string, string) error
	Tables(context.Context, string) ([]string, error)
	Query(context.Context, string, string, []any) (remote.SQLQueryResult, error)
	Exec(context.Context, string, string, []any) (remote.SQLExecResult, error)
	SSHHosts(context.Context) ([]remote.SSHHost, error)
	CreateSSHHost(context.Context, remote.SSHHostInput) (remote.SSHHost, error)
	UpdateSSHHost(context.Context, string, remote.SSHHostInput) (remote.SSHHost, error)
	DeleteSSHHost(context.Context, string) error
	SSHHostCredentials(context.Context, string) (remote.SSHHostCredentials, error)
	SetSSHHostFingerprint(context.Context, string, string) error
	Logout(context.Context) error
}

// NewModel opens the application on the remote login screen.
func NewModel(remoteService *remote.Service) *Model {
	return &Model{
		RemoteService: remoteService,
		Screen:        ScreenRemoteLogin,
		Tab:           TabStorage,
		Status:        "Sign in to the remote authentication service.",
	}
}

// SetRemoteSession enters the remote workspace with the account SSH flag.
func (m *Model) SetRemoteSession(session RemoteSession, sshEnabled bool) {
	if m == nil || session == nil {
		return
	}
	m.RemoteSession = session
	m.RemoteAccountName = session.Account().Username
	m.SSHEnabled = sshEnabled
	m.Screen = ScreenRemoteWorkspace
	m.Tab = TabStorage
	m.Status = "Remote workspace ready."
	m.Error = ""
}

// SelectTab changes the active workspace capability.
func (m *Model) SelectTab(tab Tab) {
	if m == nil {
		return
	}
	if m.Screen != ScreenRemoteWorkspace {
		return
	}
	if tab == TabSSH && !m.SSHEnabled {
		return
	}
	if tab != TabSSH && tab != TabStorage && tab != TabDatabase {
		return
	}
	m.Tab = tab
	m.Error = ""
}

// LogoutRemote revokes the remote session and returns to the remote login screen.
func (m *Model) LogoutRemote(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var err error
	if m.RemoteSession != nil {
		err = m.RemoteSession.Logout(ctx)
	}
	m.finishRemoteLogout()
	return err
}

func (m *Model) finishRemoteLogout() {
	m.RemoteSession = nil
	m.RemoteAccountName = ""
	m.SSHEnabled = false
	m.Screen = ScreenRemoteLogin
	m.Tab = TabStorage
	m.Status = "Sign in to the remote authentication service."
	m.Error = ""
}
