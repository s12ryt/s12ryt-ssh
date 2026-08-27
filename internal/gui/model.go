// Package gui contains the Gio presentation layer and its testable state.
package gui

import (
	"context"
	"io"

	"s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/vault"
)

// Screen identifies the top-level application flow.
type Screen uint8

const (
	ScreenSetup Screen = iota
	ScreenLogin
	ScreenRecovery
	ScreenWorkspace
	ScreenRemoteLogin
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
	Service           *app.Service
	RemoteService     *remote.Service
	Screen            Screen
	Tab               Tab
	AccountName       string
	RecoveryKey       string
	Status            string
	Error             string
	Registration      vault.Registration
	Session           *app.Session
	RemoteSession     RemoteSession
	RemoteAccountName string
	returnScreen      Screen
}

// RemoteSession is the credential-free workspace contract exposed to the GUI.
type RemoteSession interface {
	Account() remote.Account
	Resources(context.Context) ([]remote.Resource, error)
	ListObjects(context.Context, string, string) ([]remote.S3Object, error)
	UploadObject(context.Context, string, string, io.ReadSeeker, int64) (remote.UploadResult, error)
	DownloadObject(context.Context, string, string) (remote.Download, error)
	DeleteObject(context.Context, string, string) error
	Tables(context.Context, string) ([]string, error)
	Query(context.Context, string, string, []any) (remote.SQLQueryResult, error)
	Exec(context.Context, string, string, []any) (remote.SQLExecResult, error)
	Logout(context.Context) error
}

// NewModel selects the first screen from the local vault metadata.
func NewModel(service *app.Service) *Model {
	return NewModelWithRemote(service, nil)
}

// NewModelWithRemote selects the local first screen and wires the optional remote service.
func NewModelWithRemote(service *app.Service, remoteService *remote.Service) *Model {
	m := &Model{Service: service, RemoteService: remoteService, Screen: ScreenSetup, Tab: TabSSH}
	if service == nil {
		m.Status = "Create a vault to get started."
		return m
	}
	metadata, err := service.Metadata()
	if err != nil {
		m.Status = "Create a vault to get started."
		return m
	}
	m.Screen = ScreenLogin
	m.AccountName = metadata.Name
	m.Status = "Sign in to unlock your encrypted vault."
	return m
}

// BeginRemoteLogin opens the independent remote authentication flow.
func (m *Model) BeginRemoteLogin() {
	if m == nil || (m.Screen != ScreenSetup && m.Screen != ScreenLogin) {
		return
	}
	m.returnScreen = m.Screen
	m.Screen = ScreenRemoteLogin
	m.Status = "Sign in to the remote authentication service."
	m.Error = ""
}

// CancelRemoteLogin returns to the local screen that opened the remote flow.
func (m *Model) CancelRemoteLogin() {
	if m == nil || m.Screen != ScreenRemoteLogin {
		return
	}
	m.Screen = m.returnScreen
	m.Status = localScreenStatus(m.Screen)
	m.Error = ""
}

// SetRemoteSession enters the credential-free S3/SQL remote workspace.
func (m *Model) SetRemoteSession(session RemoteSession) {
	if m == nil || session == nil {
		return
	}
	m.RemoteSession = session
	m.RemoteAccountName = session.Account().Username
	m.Screen = ScreenRemoteWorkspace
	m.Tab = TabStorage
	m.Status = "Remote workspace ready."
	m.Error = ""
}

// SetRegistration moves the flow to the one-time recovery-key screen.
func (m *Model) SetRegistration(registration vault.Registration) {
	if m == nil {
		return
	}
	m.Registration = registration
	m.RecoveryKey = registration.RecoveryKey
	m.AccountName = registration.Name
	m.Screen = ScreenRecovery
	m.Error = ""
	m.Status = "Save this recovery key before continuing."
}

// ContinueFromRecovery moves from the recovery-key screen to login.
func (m *Model) ContinueFromRecovery() {
	if m == nil {
		return
	}
	m.Screen = ScreenLogin
	m.AccountName = m.Registration.Name
	m.RecoveryKey = ""
	m.Status = "Sign in to unlock your encrypted vault."
	m.Error = ""
}

// BeginRecovery opens the recovery form from the login screen.
func (m *Model) BeginRecovery() {
	if m == nil {
		return
	}
	m.Screen = ScreenRecovery
	m.RecoveryKey = ""
	m.Status = "Enter the one-time recovery key and new credentials."
	m.Error = ""
}

// SetSession enters the workspace with a live decrypted session.
func (m *Model) SetSession(session *app.Session) {
	if m == nil {
		return
	}
	m.Session = session
	m.Screen = ScreenWorkspace
	m.Tab = TabSSH
	m.AccountName = session.Registration().Name
	m.Status = "Ready."
	m.Error = ""
}

// SelectTab changes the active workspace capability.
func (m *Model) SelectTab(tab Tab) {
	if m == nil {
		return
	}
	if m.Screen == ScreenWorkspace {
		if tab > TabDatabase {
			return
		}
	} else if m.Screen == ScreenRemoteWorkspace {
		if tab != TabStorage && tab != TabDatabase {
			return
		}
	} else {
		return
	}
	m.Tab = tab
	m.Error = ""
}

// LogoutRemote revokes the remote session and returns to the original local screen.
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
	m.Screen = m.returnScreen
	m.Tab = TabSSH
	m.Status = localScreenStatus(m.Screen)
	m.Error = ""
}

// Logout closes the live session and returns to the login screen.
func (m *Model) Logout() error {
	if m == nil {
		return nil
	}
	var err error
	if m.Session != nil {
		err = m.Session.Close()
	}
	m.Session = nil
	m.Screen = ScreenLogin
	m.RecoveryKey = ""
	m.Status = "Sign in to unlock your encrypted vault."
	m.Error = ""
	return err
}

func localScreenStatus(screen Screen) string {
	if screen == ScreenLogin {
		return "Sign in to unlock your encrypted vault."
	}
	return "Create a vault to get started."
}
