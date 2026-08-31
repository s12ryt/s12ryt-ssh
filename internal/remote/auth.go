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
	overview, err := s.ResourcesOverview(ctx)
	if err != nil {
		return nil, err
	}
	return overview.Resources, nil
}

// ResourcesOverview returns assigned connections plus the account SSH switch.
func (s *Session) ResourcesOverview(ctx context.Context) (ResourcesOverview, error) {
	var overview ResourcesOverview
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/resources", nil, &overview); err != nil {
		return ResourcesOverview{}, err
	}
	return overview, nil
}

// SSHWorkspacePreferences returns synchronized account-level SSH UI defaults.
func (s *Session) SSHWorkspacePreferences(ctx context.Context) (SSHWorkspacePreferences, error) {
	var preferences SSHWorkspacePreferences
	err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/preferences", nil, &preferences)
	return preferences, err
}

// UpdateSSHWorkspacePreferences persists synchronized account-level SSH UI defaults.
func (s *Session) UpdateSSHWorkspacePreferences(ctx context.Context, input SSHWorkspacePreferencesInput) (SSHWorkspacePreferences, error) {
	var preferences SSHWorkspacePreferences
	err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/preferences", input, &preferences)
	return preferences, err
}

// SSHHosts lists this account's self-managed SSH hosts without credentials.
func (s *Session) SSHHosts(ctx context.Context) ([]SSHHost, error) {
	var response struct {
		Hosts []SSHHost `json:"hosts"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/hosts", nil, &response); err != nil {
		return nil, err
	}
	return response.Hosts, nil
}

// CreateSSHHost stores a new SSH host with its credentials on the server.
func (s *Session) CreateSSHHost(ctx context.Context, input SSHHostInput) (SSHHost, error) {
	var host SSHHost
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/hosts", input, &host); err != nil {
		return SSHHost{}, err
	}
	return host, nil
}

// CloneSSHHost creates a server-side copy, including credentials, with a new name.
func (s *Session) CloneSSHHost(ctx context.Context, hostID, name string) (SSHHost, error) {
	var host SSHHost
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/hosts/"+hostID+"/clone", struct {
		Name string `json:"name"`
	}{Name: name}, &host); err != nil {
		return SSHHost{}, err
	}
	return host, nil
}

// UpdateSSHHost updates host fields; empty credential fields keep the stored secret.
func (s *Session) UpdateSSHHost(ctx context.Context, hostID string, input SSHHostInput) (SSHHost, error) {
	var host SSHHost
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/hosts/"+hostID, input, &host); err != nil {
		return SSHHost{}, err
	}
	return host, nil
}

// DeleteSSHHost removes the host and its stored credentials.
func (s *Session) DeleteSSHHost(ctx context.Context, hostID string) error {
	return s.authorizedJSON(ctx, http.MethodDelete, "/api/v1/ssh/hosts/"+hostID, nil, nil)
}

// SSHHostCredentials fetches the one-time credential issuance for a host.
func (s *Session) SSHHostCredentials(ctx context.Context, hostID string) (SSHHostCredentials, error) {
	var credentials SSHHostCredentials
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/hosts/"+hostID+"/credentials", nil, &credentials); err != nil {
		return SSHHostCredentials{}, err
	}
	return credentials, nil
}

// SetSSHHostFingerprint records the confirmed host key fingerprint.
func (s *Session) SetSSHHostFingerprint(ctx context.Context, hostID, fingerprint string) error {
	return s.SetSSHHostFingerprintWithSource(ctx, hostID, fingerprint, SSHHostFingerprintTOFU)
}

// SetSSHHostFingerprintWithSource records a trusted host key and its origin.
func (s *Session) SetSSHHostFingerprintWithSource(
	ctx context.Context,
	hostID, fingerprint string,
	source SSHHostFingerprintSource,
) error {
	return s.authorizedJSON(ctx, http.MethodPut, "/api/v1/ssh/hosts/"+hostID+"/fingerprint", sshFingerprintRequest{
		Fingerprint: fingerprint,
		Source:      source,
	}, nil)
}

// SSHHostFingerprints returns the complete active and retired trust history.
func (s *Session) SSHHostFingerprints(ctx context.Context, hostID string) ([]SSHHostFingerprint, error) {
	var response struct {
		Fingerprints []SSHHostFingerprint `json:"fingerprints"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/hosts/"+hostID+"/fingerprints", nil, &response); err != nil {
		return nil, err
	}
	return response.Fingerprints, nil
}

// ClearSSHHostFingerprint removes current trust while retaining retired history.
func (s *Session) ClearSSHHostFingerprint(ctx context.Context, hostID string) error {
	return s.authorizedJSON(ctx, http.MethodDelete, "/api/v1/ssh/hosts/"+hostID+"/fingerprint", nil, nil)
}

// SSHTunnels lists this account's saved SSH forwarding rules.
func (s *Session) SSHTunnels(ctx context.Context) ([]SSHTunnelRule, error) {
	var response struct {
		Tunnels []SSHTunnelRule `json:"tunnels"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/tunnels", nil, &response); err != nil {
		return nil, err
	}
	return response.Tunnels, nil
}

// CreateSSHTunnel saves an SSH forwarding rule.
func (s *Session) CreateSSHTunnel(ctx context.Context, input SSHTunnelInput) (SSHTunnelRule, error) {
	var tunnel SSHTunnelRule
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/tunnels", input, &tunnel); err != nil {
		return SSHTunnelRule{}, err
	}
	return tunnel, nil
}

// UpdateSSHTunnel updates a saved SSH forwarding rule.
func (s *Session) UpdateSSHTunnel(ctx context.Context, tunnelID string, input SSHTunnelInput) (SSHTunnelRule, error) {
	var tunnel SSHTunnelRule
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/tunnels/"+tunnelID, input, &tunnel); err != nil {
		return SSHTunnelRule{}, err
	}
	return tunnel, nil
}

// UpdateSSHTunnelRuntime reports local forwarding state without changing the saved rule version.
func (s *Session) UpdateSSHTunnelRuntime(ctx context.Context, tunnelID string, input SSHTunnelRuntimeUpdate) (SSHTunnelRule, error) {
	var tunnel SSHTunnelRule
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/tunnels/"+tunnelID+"/runtime", input, &tunnel); err != nil {
		return SSHTunnelRule{}, err
	}
	return tunnel, nil
}

// DeleteSSHTunnel removes a saved SSH forwarding rule.
func (s *Session) DeleteSSHTunnel(ctx context.Context, tunnelID string) error {
	return s.authorizedJSON(ctx, http.MethodDelete, "/api/v1/ssh/tunnels/"+tunnelID, nil, nil)
}

// SSHCommandSnippets lists saved commands without exposing secret values.
func (s *Session) SSHCommandSnippets(ctx context.Context) ([]SSHCommandSnippet, error) {
	var response struct {
		Snippets []SSHCommandSnippet `json:"snippets"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/snippets", nil, &response); err != nil {
		return nil, err
	}
	return response.Snippets, nil
}

// CreateSSHCommandSnippet saves a command and its encrypted secret values remotely.
func (s *Session) CreateSSHCommandSnippet(ctx context.Context, input SSHCommandSnippetInput) (SSHCommandSnippet, error) {
	var snippet SSHCommandSnippet
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/snippets", input, &snippet); err != nil {
		return SSHCommandSnippet{}, err
	}
	return snippet, nil
}

// UpdateSSHCommandSnippet updates a saved command and its optional secret values.
func (s *Session) UpdateSSHCommandSnippet(ctx context.Context, snippetID string, input SSHCommandSnippetInput) (SSHCommandSnippet, error) {
	var snippet SSHCommandSnippet
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/snippets/"+snippetID, input, &snippet); err != nil {
		return SSHCommandSnippet{}, err
	}
	return snippet, nil
}

// DeleteSSHCommandSnippet removes a saved command.
func (s *Session) DeleteSSHCommandSnippet(ctx context.Context, snippetID string) error {
	return s.authorizedJSON(ctx, http.MethodDelete, "/api/v1/ssh/snippets/"+snippetID, nil, nil)
}

// SSHCommandSnippetSecrets fetches secret values only when a command is executed.
func (s *Session) SSHCommandSnippetSecrets(ctx context.Context, snippetID string) (map[string]string, error) {
	var secrets map[string]string
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/snippets/"+snippetID+"/secrets", nil, &secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}

// SSHKeyIdentities lists reusable key identities without private material.
func (s *Session) SSHKeyIdentities(ctx context.Context) ([]SSHKeyIdentity, error) {
	var response struct {
		Keys []SSHKeyIdentity `json:"keys"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/keys", nil, &response); err != nil {
		return nil, err
	}
	return response.Keys, nil
}

// CreateSSHKeyIdentity stores a reusable private-key identity remotely.
func (s *Session) CreateSSHKeyIdentity(ctx context.Context, input SSHKeyIdentityInput) (SSHKeyIdentity, error) {
	var identity SSHKeyIdentity
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/keys", input, &identity); err != nil {
		return SSHKeyIdentity{}, err
	}
	return identity, nil
}

// UpdateSSHKeyIdentity updates a reusable key identity.
func (s *Session) UpdateSSHKeyIdentity(ctx context.Context, keyID string, input SSHKeyIdentityInput) (SSHKeyIdentity, error) {
	var identity SSHKeyIdentity
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/keys/"+keyID, input, &identity); err != nil {
		return SSHKeyIdentity{}, err
	}
	return identity, nil
}

// DeleteSSHKeyIdentity removes a reusable key identity and its private material.
func (s *Session) DeleteSSHKeyIdentity(ctx context.Context, keyID string) error {
	return s.authorizedJSON(ctx, http.MethodDelete, "/api/v1/ssh/keys/"+keyID, nil, nil)
}

// SSHKeyIdentitySecrets fetches private material only when a key is needed.
func (s *Session) SSHKeyIdentitySecrets(ctx context.Context, keyID string) (SSHKeyIdentitySecrets, error) {
	var secrets SSHKeyIdentitySecrets
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/keys/"+keyID+"/secrets", nil, &secrets); err != nil {
		return SSHKeyIdentitySecrets{}, err
	}
	return secrets, nil
}

// SSHSessionHistory lists recent SSH connection metadata without terminal content.
func (s *Session) SSHSessionHistory(ctx context.Context) ([]SSHSessionHistory, error) {
	var response struct {
		History []SSHSessionHistory `json:"history"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, "/api/v1/ssh/session-history", nil, &response); err != nil {
		return nil, err
	}
	return response.History, nil
}

// CreateSSHSessionHistory starts a server-side SSH connection lifecycle record.
func (s *Session) CreateSSHSessionHistory(ctx context.Context, input SSHSessionHistoryInput) (SSHSessionHistory, error) {
	var history SSHSessionHistory
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/session-history", input, &history); err != nil {
		return SSHSessionHistory{}, err
	}
	return history, nil
}

// UpdateSSHSessionHistory advances an existing lifecycle record.
func (s *Session) UpdateSSHSessionHistory(
	ctx context.Context,
	historyID string,
	input SSHSessionHistoryUpdate,
) (SSHSessionHistory, error) {
	var history SSHSessionHistory
	if err := s.authorizedJSON(ctx, http.MethodPatch, "/api/v1/ssh/session-history/"+historyID, input, &history); err != nil {
		return SSHSessionHistory{}, err
	}
	return history, nil
}

// ExportSSHWorkspace asks the service to build an opaque workspace package.
func (s *Session) ExportSSHWorkspace(ctx context.Context, input SSHWorkspaceExportRequest) (string, error) {
	var response struct {
		Package string `json:"package"`
	}
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/workspace/export", input, &response); err != nil {
		return "", err
	}
	if response.Package == "" {
		return "", errors.New("remote workspace export returned an empty package")
	}
	return response.Package, nil
}

// PreviewSSHWorkspaceImport decrypts and validates a package without changing server state.
func (s *Session) PreviewSSHWorkspaceImport(ctx context.Context, encoded, password string) (SSHWorkspaceImportPreview, error) {
	var preview SSHWorkspaceImportPreview
	input := struct {
		Package  string `json:"package"`
		Password string `json:"password,omitempty"`
	}{Package: encoded, Password: password}
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/workspace/import/preview", input, &preview); err != nil {
		return SSHWorkspaceImportPreview{}, err
	}
	return preview, nil
}

// ApplySSHWorkspaceImport atomically applies a package and explicit conflict decisions.
func (s *Session) ApplySSHWorkspaceImport(ctx context.Context, input SSHWorkspaceImportRequest) (SSHWorkspaceImportResult, error) {
	var result SSHWorkspaceImportResult
	if err := s.authorizedJSON(ctx, http.MethodPost, "/api/v1/ssh/workspace/import/apply", input, &result); err != nil {
		return SSHWorkspaceImportResult{}, err
	}
	return result, nil
}

type sshFingerprintRequest struct {
	Fingerprint string                   `json:"fingerprint"`
	Source      SSHHostFingerprintSource `json:"source,omitempty"`
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

func rememberedPasswordKey(baseURL, username, deviceID string) string {
	digest := sha256.Sum256([]byte(baseURL + "\x00" + strings.ToLower(strings.TrimSpace(username)) + "\x00" + strings.TrimSpace(deviceID)))
	return "remote/password/" + hex.EncodeToString(digest[:])
}
