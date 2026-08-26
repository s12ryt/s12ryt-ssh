package storage

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrNotFound is returned when a requested object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// Object describes a stored object's metadata.
type Object struct {
	Key      string
	Size     int64
	Modified time.Time
}

// Storage is the common interface for remote object storage backends.
type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) ([]Object, error)
	Delete(ctx context.Context, key string) error
}

// MemoryStorage is an in-process Storage useful for testing and as a fallback.
type MemoryStorage struct {
	mu   sync.RWMutex
	data map[string]memEntry
}

type memEntry struct {
	data     []byte
	modified time.Time
}

// NewMemoryStorage creates an empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{data: map[string]memEntry{}}
}

// Put stores data under key.
func (s *MemoryStorage) Put(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dup := make([]byte, len(data))
	copy(dup, data)
	s.data[key] = memEntry{data: dup, modified: time.Now()}
	return nil
}

// Get returns the data for key, or ErrNotFound.
func (s *MemoryStorage) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, nil
}

// List returns objects whose key starts with prefix.
func (s *MemoryStorage) List(_ context.Context, prefix string) ([]Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var objs []Object
	for k, e := range s.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			objs = append(objs, Object{Key: k, Size: int64(len(e.data)), Modified: e.modified})
		}
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Key < objs[j].Key })
	return objs, nil
}

// Delete removes key. Deleting a missing key is not an error.
func (s *MemoryStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
