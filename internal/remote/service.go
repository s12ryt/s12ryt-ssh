package remote

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"s12ryt-ssh/internal/securestore"
)

// ErrNoPreferences indicates that no complete remote login preference is available.
var ErrNoPreferences = errors.New("remote: no saved login preferences")

// Service owns non-sensitive login preferences and the secure refresh-token store.
type Service struct {
	preferencesPath string
	secrets         securestore.Store
	http            *http.Client
}

// NewService creates a remote authentication service for the desktop client.
func NewService(preferencesPath string, secrets securestore.Store, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Service{preferencesPath: preferencesPath, secrets: secrets, http: httpClient}
}

// Preferences returns the current non-sensitive remote login fields.
func (s *Service) Preferences() (Preferences, error) {
	if s == nil || s.preferencesPath == "" {
		return Preferences{}, nil
	}
	return LoadPreferences(s.preferencesPath)
}

// Login authenticates with a password and persists only URL, username, device ID, and refresh token.
func (s *Service) Login(ctx context.Context, rawURL, username, password string) (*Session, error) {
	if s == nil || s.secrets == nil {
		return nil, errors.New("remote: secure store is required")
	}
	client, err := NewClient(rawURL, s.http)
	if err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	preferences, err := s.Preferences()
	if err != nil {
		return nil, err
	}
	deviceID := preferences.DeviceID
	if preferences.BaseURL != client.BaseURL() || !strings.EqualFold(preferences.Username, username) || deviceID == "" {
		deviceID, err = generateDeviceID()
		if err != nil {
			return nil, err
		}
	}
	session, err := Login(ctx, client, s.secrets, username, password, deviceID)
	if err != nil {
		return nil, err
	}
	if err := SavePreferences(s.preferencesPath, Preferences{
		BaseURL: client.BaseURL(), Username: username, DeviceID: deviceID,
	}); err != nil {
		_ = session.Logout(ctx)
		return nil, err
	}
	return session, nil
}

// Restore rotates a DPAPI-backed refresh token using the saved URL, username, and device ID.
func (s *Service) Restore(ctx context.Context) (*Session, error) {
	if s == nil || s.secrets == nil {
		return nil, errors.New("remote: secure store is required")
	}
	preferences, err := s.Preferences()
	if err != nil {
		return nil, err
	}
	if preferences.BaseURL == "" || preferences.Username == "" || preferences.DeviceID == "" {
		return nil, ErrNoPreferences
	}
	client, err := NewClient(preferences.BaseURL, s.http)
	if err != nil {
		return nil, err
	}
	return Restore(ctx, client, s.secrets, preferences.Username, preferences.DeviceID)
}

func generateDeviceID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return "desktop-" + hex.EncodeToString(data), nil
}
