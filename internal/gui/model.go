// Package gui contains the Gio presentation layer and its testable state.
package gui

import (
	"s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/vault"
)

// Screen identifies the top-level application flow.
type Screen uint8

const (
	ScreenSetup Screen = iota
	ScreenLogin
	ScreenRecovery
	ScreenWorkspace
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
	Service      *app.Service
	Screen       Screen
	Tab          Tab
	AccountName  string
	RecoveryKey  string
	Status       string
	Error        string
	Registration vault.Registration
	Session      *app.Session
}

// NewModel selects the first screen from the local vault metadata.
func NewModel(service *app.Service) *Model {
	m := &Model{Service: service, Screen: ScreenSetup, Tab: TabSSH}
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
	if m == nil || m.Screen != ScreenWorkspace {
		return
	}
	if tab > TabDatabase {
		return
	}
	m.Tab = tab
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
