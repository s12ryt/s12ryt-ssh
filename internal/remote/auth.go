package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"s12ryt-ssh/internal/securestore"
)

var (
	// ErrNoRefreshToken indicates that this device has no saved remote session.
	ErrNoRefreshToken = errors.New("remote: no saved refresh token")
	// ErrSessionClosed indicates that the remote session was logged out.
	ErrSessionClosed = errors.New("remote: session is closed")
)

const accessRefreshSkew = 30 * time.Second

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeviceID string `json:"deviceId"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
	DeviceID     string `json:"deviceId"`
}

// Login authenticates with a password and stores only the returned refresh token.
func Login(ctx context.Context, client *Client, secrets securestore.Store, username, password, deviceID string) (*Session, error) {
	username = strings.TrimSpace(username)
	deviceID = strings.TrimSpace(deviceID)
	var pair TokenPair
	if err := client.doJSON(ctx, http.MethodPost, "/api/v1/auth/login", "", loginRequest{
		Username: username,
		Password: password,
		DeviceID: deviceID,
	}, &pair); err != nil {
		return nil, err
	}
	return newSession(client, secrets, username, deviceID, pair)
}

// Restore rotates a saved refresh token into a new authenticated session.
func Restore(ctx context.Context, client *Client, secrets securestore.Store, username, deviceID string) (*Session, error) {
	username = strings.TrimSpace(username)
	deviceID = strings.TrimSpace(deviceID)
	key := refreshTokenKey(client.BaseURL(), username, deviceID)
	refreshToken, err := secrets.Load(key)
	if errors.Is(err, securestore.ErrNotFound) {
		return nil, ErrNoRefreshToken
	}
	if err != nil {
		return nil, err
	}
	pair, err := client.refresh(ctx, string(refreshToken), deviceID)
	if err != nil {
		return nil, err
	}
	return newSession(client, secrets, username, deviceID, pair)
}

// Session owns an in-memory access token and one DPAPI-backed rotating refresh token.
type Session struct {
	client   *Client
	secrets  securestore.Store
	key      string
	username string
	deviceID string

	mu     sync.RWMutex
	pair   TokenPair
	closed bool

	refreshMu sync.Mutex
}

func newSession(client *Client, secrets securestore.Store, username, deviceID string, pair TokenPair) (*Session, error) {
	key := refreshTokenKey(client.BaseURL(), username, deviceID)
	if err := secrets.Save(key, []byte(pair.RefreshToken)); err != nil {
		return nil, err
	}
	return &Session{
		client: client, secrets: secrets, key: key, username: username, deviceID: deviceID, pair: pair,
	}, nil
}

// Account returns the authenticated account without exposing tokens.
func (s *Session) Account() Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pair.Account
}

// Resources lists the enabled connections assigned to this account.
func (s *Session) Resources(ctx context.Context) ([]Resource, error) {
	var response struct {
		Resources []Resource `json:"resources"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/resources", nil, &response); err != nil {
		return nil, err
	}
	return response.Resources, nil
}

// Logout revokes the server session and always removes the local refresh token.
func (s *Session) Logout(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	accessToken := s.pair.AccessToken
	s.pair = TokenPair{}
	s.mu.Unlock()

	var remoteErr error
	if accessToken != "" {
		remoteErr = s.client.doJSON(ctx, http.MethodPost, "/api/v1/auth/logout", accessToken, nil, nil)
	}
	localErr := s.secrets.Delete(s.key)
	return errors.Join(remoteErr, localErr)
}

func (s *Session) authorizedJSON(ctx context.Context, method, path string, input, output any) error {
	return s.authorized(ctx, func(accessToken string) error {
		return s.client.doJSON(ctx, method, path, accessToken, input, output)
	})
}

func (s *Session) authorized(ctx context.Context, operation func(accessToken string) error) error {
	accessToken, expiresAt, err := s.accessState()
	if err != nil {
		return err
	}
	if time.Now().Add(accessRefreshSkew).UnixMilli() >= expiresAt {
		if err := s.refresh(ctx); err != nil {
			return err
		}
		accessToken, _, err = s.accessState()
		if err != nil {
			return err
		}
	}
	err = operation(accessToken)
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusUnauthorized {
		return err
	}
	if err := s.refresh(ctx); err != nil {
		return err
	}
	accessToken, _, err = s.accessState()
	if err != nil {
		return err
	}
	return operation(accessToken)
}

func (s *Session) refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSessionClosed
	}
	current := s.pair.RefreshToken
	s.mu.RUnlock()
	if current == "" {
		return ErrNoRefreshToken
	}
	pair, err := s.client.refresh(ctx, current, s.deviceID)
	if err != nil {
		return err
	}
	if err := s.secrets.Save(s.key, []byte(pair.RefreshToken)); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	s.pair = pair
	s.mu.Unlock()
	return nil
}

func (s *Session) accessState() (string, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return "", 0, ErrSessionClosed
	}
	return s.pair.AccessToken, s.pair.AccessExpiresAt, nil
}

func (c *Client) refresh(ctx context.Context, refreshToken, deviceID string) (TokenPair, error) {
	var pair TokenPair
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/refresh", "", refreshRequest{
		RefreshToken: refreshToken,
		DeviceID:     deviceID,
	}, &pair)
	return pair, err
}

func refreshTokenKey(baseURL, username, deviceID string) string {
	digest := sha256.Sum256([]byte(baseURL + "\x00" + strings.ToLower(strings.TrimSpace(username)) + "\x00" + strings.TrimSpace(deviceID)))
	return "remote/refresh/" + hex.EncodeToString(digest[:])
}
