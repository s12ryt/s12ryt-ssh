package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SSHProfile describes a remote SSH connection.
type SSHProfile struct {
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password,omitempty"`
	KeyPath         string `json:"key_path,omitempty"`
	KeyData         string `json:"key_data,omitempty"`
	KeyPassphrase   string `json:"key_passphrase,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`
}

// S3Profile describes an S3/R2-compatible storage bucket.
type S3Profile struct {
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	Bucket       string `json:"bucket"`
	UsePathStyle bool   `json:"use_path_style"`
}

// DBProfile describes a remote SQL database connection.
type DBProfile struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "mysql" or "postgres"
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode,omitempty"`
	TLSMode  string `json:"tls_mode,omitempty"`
}

// Store holds all persisted connection profiles.
type Store struct {
	SSH []SSHProfile `json:"ssh"`
	S3  []S3Profile  `json:"s3"`
	DB  []DBProfile  `json:"db"`
}

// FindSSH returns the SSH profile with the given name.
func (s *Store) FindSSH(name string) (SSHProfile, bool) {
	for _, p := range s.SSH {
		if p.Name == name {
			return p, true
		}
	}
	return SSHProfile{}, false
}

// FindS3 returns the S3 profile with the given name.
func (s *Store) FindS3(name string) (S3Profile, bool) {
	for _, p := range s.S3 {
		if p.Name == name {
			return p, true
		}
	}
	return S3Profile{}, false
}

// FindDB returns the database profile with the given name.
func (s *Store) FindDB(name string) (DBProfile, bool) {
	for _, p := range s.DB {
		if p.Name == name {
			return p, true
		}
	}
	return DBProfile{}, false
}

// Manager persists and loads a Store as JSON.
type Manager struct {
	path string
}

// NewManager creates a Manager. If path is empty, a default user-config path is used.
func NewManager(path string) *Manager {
	if path == "" {
		path = defaultPath()
	}
	return &Manager{path: path}
}

// Path returns the configured file path.
func (m *Manager) Path() string { return m.path }

// Save writes the store to disk as JSON.
func (m *Manager) Save(s *Store) error {
	if s == nil {
		s = &Store{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o600)
}

// Load reads the store from disk. A missing file yields an empty store.
func (m *Manager) Load() (*Store, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".s12ryt-ssh", "config.json")
}
