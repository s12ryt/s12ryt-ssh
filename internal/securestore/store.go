// Package securestore stores small bootstrap secrets outside the application
// configuration file.
package securestore

import (
	"errors"
	"sync"
)

// ErrNotFound is returned when a secret has not been stored.
var ErrNotFound = errors.New("securestore: secret not found")

// ErrInvalidKey is returned when a secret name is empty.
var ErrInvalidKey = errors.New("securestore: key is required")

// Store is the storage contract used for bootstrap credentials.
type Store interface {
	Save(key string, secret []byte) error
	Load(key string) ([]byte, error)
	Delete(key string) error
}

// MemoryStore is an isolated in-memory Store for tests and in-process use.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryStore creates an empty memory-backed Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string][]byte)}
}

// Save stores a defensive copy of secret under key.
func (s *MemoryStore) Save(key string, secret []byte) error {
	if key == "" {
		return ErrInvalidKey
	}
	copySecret := append([]byte(nil), secret...)
	s.mu.Lock()
	s.data[key] = copySecret
	s.mu.Unlock()
	return nil
}

// Load returns a defensive copy of the secret for key.
func (s *MemoryStore) Load(key string) ([]byte, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}
	s.mu.RLock()
	secret, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), secret...), nil
}

// Delete removes key. Deleting a missing key is idempotent.
func (s *MemoryStore) Delete(key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return nil
}
