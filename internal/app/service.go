// Package app coordinates local metadata, bootstrap credentials, and the
// encrypted remote profile vault. It intentionally has no GUI dependencies.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/database"
	"s12ryt-ssh/internal/securestore"
	"s12ryt-ssh/internal/storage"
	"s12ryt-ssh/internal/vault"
)

const metadataVersion = 1

var (
	// ErrNotConfigured means that the local vault metadata does not exist.
	ErrNotConfigured = errors.New("app: vault is not configured")
	// ErrInvalidBootstrap means that the selected remote vault backend is not
	// sufficiently described to establish a connection.
	ErrInvalidBootstrap = errors.New("app: invalid bootstrap configuration")
	// ErrSessionClosed means that a logged-in session has already been closed.
	ErrSessionClosed = errors.New("app: session is closed")
)

// Bootstrap contains the connection profile used only to reach the encrypted
// vault. It is stored in securestore, never in Metadata.
type Bootstrap struct {
	Backend string           `json:"backend"`
	S3      config.S3Profile `json:"s3,omitempty"`
	DB      config.DBProfile `json:"db,omitempty"`
}

// Metadata is the non-secret local record needed to find the remote vault.
type Metadata struct {
	Version      int    `json:"version"`
	VaultID      string `json:"vault_id"`
	Name         string `json:"name"`
	Backend      string `json:"backend"`
	BootstrapKey string `json:"bootstrap_key"`
}

// BackendHandle owns a vault backend and any connection that must be closed.
type BackendHandle struct {
	Backend vault.Backend
	Probe   func(context.Context) error
	Close   func() error
}

// BackendFactory opens a backend for a bootstrap profile. It is injectable so
// service behavior can be tested without real cloud or database services.
type BackendFactory func(context.Context, Bootstrap) (BackendHandle, error)

// Service manages one local vault configuration and its remote encrypted
// record.
type Service struct {
	metadataPath string
	secrets      securestore.Store
	factory      BackendFactory
}

// NewService creates a vault service with explicit dependencies.
func NewService(metadataPath string, secrets securestore.Store, factory BackendFactory) *Service {
	return &Service{metadataPath: metadataPath, secrets: secrets, factory: factory}
}

// Configured reports whether a valid local vault metadata record exists.
func (s *Service) Configured() bool {
	_, err := s.Metadata()
	return err == nil
}

// Metadata returns the non-secret local vault record used by the GUI.
func (s *Service) Metadata() (Metadata, error) {
	return s.loadMetadata()
}

// Register creates a vault, verifies the bootstrap connection, and persists
// only encrypted profile data remotely.
func (s *Service) Register(ctx context.Context, bootstrap Bootstrap, name, password string, profiles *config.Store) (vault.Registration, error) {
	if err := validateBootstrap(bootstrap); err != nil {
		return vault.Registration{}, err
	}
	if strings.TrimSpace(name) == "" || password == "" {
		return vault.Registration{}, fmt.Errorf("app: name and password are required")
	}
	handle, err := s.open(ctx, bootstrap)
	if err != nil {
		return vault.Registration{}, err
	}
	defer handle.close()
	if err := probe(ctx, handle); err != nil {
		return vault.Registration{}, err
	}
	registration, payload, err := vault.Create(name, password, profiles)
	if err != nil {
		return vault.Registration{}, err
	}
	bootstrapKey := bootstrapKeyFor(registration.ID)
	bootstrapData, err := json.Marshal(bootstrap)
	if err != nil {
		return vault.Registration{}, err
	}
	if err := s.secrets.Save(bootstrapKey, bootstrapData); err != nil {
		return vault.Registration{}, fmt.Errorf("app: save bootstrap secret: %w", err)
	}
	if err := handle.Backend.Save(ctx, registration.ID, payload); err != nil {
		_ = s.secrets.Delete(bootstrapKey)
		return vault.Registration{}, fmt.Errorf("app: save remote vault: %w", err)
	}
	metadata := Metadata{
		Version:      metadataVersion,
		VaultID:      registration.ID,
		Name:         registration.Name,
		Backend:      normalizeBackend(bootstrap.Backend),
		BootstrapKey: bootstrapKey,
	}
	if err := s.saveMetadata(metadata); err != nil {
		_ = handle.Backend.Delete(ctx, registration.ID)
		_ = s.secrets.Delete(bootstrapKey)
		return vault.Registration{}, fmt.Errorf("app: save metadata: %w", err)
	}
	return registration, nil
}

// TestBootstrap verifies that a remote vault backend can be opened and probed.
func (s *Service) TestBootstrap(ctx context.Context, bootstrap Bootstrap) error {
	if err := validateBootstrap(bootstrap); err != nil {
		return err
	}
	handle, err := s.open(ctx, bootstrap)
	if err != nil {
		return err
	}
	defer handle.close()
	return probe(ctx, handle)
}

// Login opens and decrypts the remote profile vault.
func (s *Service) Login(ctx context.Context, name, password string) (*Session, error) {
	metadata, err := s.loadMetadata()
	if err != nil {
		return nil, err
	}
	bootstrapData, err := s.secrets.Load(metadata.BootstrapKey)
	if err != nil {
		return nil, fmt.Errorf("app: load bootstrap secret: %w", err)
	}
	var bootstrap Bootstrap
	if err := json.Unmarshal(bootstrapData, &bootstrap); err != nil {
		return nil, fmt.Errorf("app: decode bootstrap secret: %w", err)
	}
	if normalizeBackend(bootstrap.Backend) != metadata.Backend {
		return nil, fmt.Errorf("%w: backend metadata mismatch", ErrInvalidBootstrap)
	}
	handle, err := s.open(ctx, bootstrap)
	if err != nil {
		return nil, err
	}
	if err := probe(ctx, handle); err != nil {
		handle.close()
		return nil, err
	}
	payload, err := handle.Backend.Load(ctx, metadata.VaultID)
	if err != nil {
		handle.close()
		return nil, fmt.Errorf("app: load remote vault: %w", err)
	}
	profiles, err := vault.Decrypt(payload, name, password)
	if err != nil {
		handle.close()
		return nil, err
	}
	return &Session{
		service:   s,
		metadata:  metadata,
		bootstrap: bootstrap,
		handle:    handle,
		state:     sessionState{profiles: profiles},
		name:      strings.TrimSpace(name),
		password:  password,
	}, nil
}

// Recover rotates the vault login and one-time recovery key, then replaces the
// remote encrypted record before updating local metadata.
func (s *Service) Recover(ctx context.Context, recoveryKey, newName, newPassword string) (vault.Registration, error) {
	metadata, err := s.loadMetadata()
	if err != nil {
		return vault.Registration{}, err
	}
	bootstrapData, err := s.secrets.Load(metadata.BootstrapKey)
	if err != nil {
		return vault.Registration{}, fmt.Errorf("app: load bootstrap secret: %w", err)
	}
	var bootstrap Bootstrap
	if err := json.Unmarshal(bootstrapData, &bootstrap); err != nil {
		return vault.Registration{}, fmt.Errorf("app: decode bootstrap secret: %w", err)
	}
	handle, err := s.open(ctx, bootstrap)
	if err != nil {
		return vault.Registration{}, err
	}
	defer handle.close()
	if err := probe(ctx, handle); err != nil {
		return vault.Registration{}, err
	}
	payload, err := handle.Backend.Load(ctx, metadata.VaultID)
	if err != nil {
		return vault.Registration{}, fmt.Errorf("app: load remote vault: %w", err)
	}
	registration, rotated, err := vault.Recover(payload, recoveryKey, newName, newPassword)
	if err != nil {
		return vault.Registration{}, err
	}
	if err := handle.Backend.Save(ctx, registration.ID, rotated); err != nil {
		return vault.Registration{}, fmt.Errorf("app: save recovered vault: %w", err)
	}
	metadata.Name = registration.Name
	if err := s.saveMetadata(metadata); err != nil {
		return vault.Registration{}, fmt.Errorf("app: save recovered metadata: %w", err)
	}
	return registration, nil
}

type sessionState struct {
	mu       sync.RWMutex
	closed   bool
	profiles *config.Store
}

// Session is a logged-in, decrypted profile vault and its live backend.
type Session struct {
	service   *Service
	metadata  Metadata
	bootstrap Bootstrap
	handle    BackendHandle
	name      string
	password  string
	state     sessionState
	closeOnce sync.Once
}

// Profiles returns a defensive copy of the decrypted profiles.
func (s *Session) Profiles() (*config.Store, error) {
	if s == nil {
		return nil, ErrSessionClosed
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	if s.state.closed {
		return nil, ErrSessionClosed
	}
	return cloneStore(s.state.profiles)
}

// SaveProfiles encrypts and publishes the current profile set.
func (s *Session) SaveProfiles(ctx context.Context, profiles *config.Store) error {
	if s == nil {
		return ErrSessionClosed
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if s.state.closed {
		return ErrSessionClosed
	}
	current, err := s.handle.Backend.Load(ctx, s.metadata.VaultID)
	if err != nil {
		return fmt.Errorf("app: load vault before update: %w", err)
	}
	updated, err := vault.Update(current, s.name, s.password, profiles)
	if err != nil {
		return err
	}
	if err := s.handle.Backend.Save(ctx, s.metadata.VaultID, updated); err != nil {
		return fmt.Errorf("app: save updated vault: %w", err)
	}
	s.state.profiles, err = cloneStore(profiles)
	return err
}

// Registration returns the non-secret current vault identity.
func (s *Session) Registration() vault.Registration {
	if s == nil {
		return vault.Registration{}
	}
	return vault.Registration{ID: s.metadata.VaultID, Name: s.metadata.Name}
}

// Close releases the backend connection. It is safe to call repeatedly.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		s.state.mu.Lock()
		s.state.closed = true
		s.state.mu.Unlock()
		err = s.handle.close()
	})
	return err
}

func (s *Service) open(ctx context.Context, bootstrap Bootstrap) (BackendHandle, error) {
	if s == nil || s.factory == nil {
		return BackendHandle{}, errors.New("app: backend factory is not configured")
	}
	handle, err := s.factory(ctx, bootstrap)
	if err != nil {
		return BackendHandle{}, err
	}
	if handle.Backend == nil {
		_ = handle.close()
		return BackendHandle{}, errors.New("app: backend factory returned nil backend")
	}
	return handle, nil
}

func (h BackendHandle) close() error {
	if h.Close == nil {
		return nil
	}
	return h.Close()
}

func probe(ctx context.Context, handle BackendHandle) error {
	if handle.Probe == nil {
		return nil
	}
	return handle.Probe(ctx)
}

func (s *Service) saveMetadata(metadata Metadata) error {
	if s == nil || strings.TrimSpace(s.metadataPath) == "" {
		return errors.New("app: metadata path is required")
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.metadataPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".metadata-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.metadataPath)
}

func (s *Service) loadMetadata() (Metadata, error) {
	if s == nil || strings.TrimSpace(s.metadataPath) == "" {
		return Metadata{}, ErrNotConfigured
	}
	data, err := os.ReadFile(s.metadataPath)
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, ErrNotConfigured
	}
	if err != nil {
		return Metadata{}, err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("app: decode metadata: %w", err)
	}
	if metadata.Version != metadataVersion || metadata.VaultID == "" || metadata.Name == "" || metadata.BootstrapKey == "" {
		return Metadata{}, fmt.Errorf("%w: invalid metadata", ErrNotConfigured)
	}
	metadata.Backend = normalizeBackend(metadata.Backend)
	if metadata.Backend == "" {
		return Metadata{}, fmt.Errorf("%w: missing backend", ErrNotConfigured)
	}
	return metadata, nil
}

func validateBootstrap(bootstrap Bootstrap) error {
	switch normalizeBackend(bootstrap.Backend) {
	case "s3":
		if strings.TrimSpace(bootstrap.S3.Endpoint) == "" || strings.TrimSpace(bootstrap.S3.Bucket) == "" || bootstrap.S3.AccessKey == "" || bootstrap.S3.SecretKey == "" {
			return fmt.Errorf("%w: S3 endpoint, bucket, access key, and secret key are required", ErrInvalidBootstrap)
		}
	case "sql":
		if strings.TrimSpace(bootstrap.DB.Type) == "" || strings.TrimSpace(bootstrap.DB.Host) == "" || bootstrap.DB.Port <= 0 || bootstrap.DB.User == "" || bootstrap.DB.Password == "" || bootstrap.DB.Database == "" {
			return fmt.Errorf("%w: SQL type, host, port, user, password, and database are required", ErrInvalidBootstrap)
		}
		client, err := database.NewDBClient(bootstrap.DB)
		if err != nil {
			// NewDBClient validates driver and DSN construction without requiring
			// a live remote connection.
			return fmt.Errorf("%w: %v", ErrInvalidBootstrap, err)
		}
		_ = client.Close()
	default:
		return fmt.Errorf("%w: backend must be s3 or sql", ErrInvalidBootstrap)
	}
	return nil
}

func normalizeBackend(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "r2" || value == "object" {
		return "s3"
	}
	return value
}

func bootstrapKeyFor(id string) string { return "vault/" + id + "/bootstrap" }

func cloneStore(store *config.Store) (*config.Store, error) {
	if store == nil {
		return &config.Store{}, nil
	}
	data, err := json.Marshal(store)
	if err != nil {
		return nil, err
	}
	var clone config.Store
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

// DefaultBackendFactory opens the selected production S3/R2 or SQL backend.
func DefaultBackendFactory(ctx context.Context, bootstrap Bootstrap) (BackendHandle, error) {
	switch normalizeBackend(bootstrap.Backend) {
	case "s3":
		remote, err := storage.NewS3Storage(bootstrap.S3)
		if err != nil {
			return BackendHandle{}, err
		}
		return BackendHandle{
			Backend: vault.NewObjectBackend(remote),
			Probe: func(ctx context.Context) error {
				_, err := remote.List(ctx, "s12ryt/vault/")
				return err
			},
		}, nil
	case "sql":
		remote, err := database.NewDBClient(bootstrap.DB)
		if err != nil {
			return BackendHandle{}, err
		}
		backend, err := vault.NewSQLBackend(remote, bootstrap.DB.Type)
		if err != nil {
			remote.Close()
			return BackendHandle{}, err
		}
		return BackendHandle{
			Backend: backend,
			Probe:   remote.Ping,
			Close:   remote.Close,
		}, nil
	default:
		return BackendHandle{}, fmt.Errorf("%w: backend must be s3 or sql", ErrInvalidBootstrap)
	}
}
