//go:build windows

package securestore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPIStore persists secrets as Windows user-scoped DPAPI blobs.
type DPAPIStore struct {
	dir string
}

// NewDPAPIStore creates a store in the current user's local configuration
// directory.
func NewDPAPIStore() *DPAPIStore {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return NewDPAPIStoreAt(filepath.Join(dir, "s12ryt-ssh", "securestore"))
}

// NewDPAPIStoreAt creates a DPAPI store rooted at dir. It is useful for tests
// and for callers that need an explicit application data location.
func NewDPAPIStoreAt(dir string) *DPAPIStore {
	return &DPAPIStore{dir: dir}
}

// Save protects secret with the current Windows user and persists the blob.
func (s *DPAPIStore) Save(key string, secret []byte) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	protected, err := protect(secret)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, protected, 0o600)
}

// Load reads and unprotects the blob for key.
func (s *DPAPIStore) Load(key string) ([]byte, error) {
	path, err := s.pathFor(key)
	if err != nil {
		return nil, err
	}
	protected, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return unprotect(protected)
}

// Delete removes a stored blob and is idempotent.
func (s *DPAPIStore) Delete(key string) error {
	path, err := s.pathFor(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *DPAPIStore) pathFor(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, fmt.Sprintf("%x.dpapi", digest[:])), nil
}

func protect(secret []byte) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(secret))}
	if len(secret) > 0 {
		input.Data = &secret[0]
	}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, 0, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func unprotect(protected []byte) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(protected))}
	if len(protected) > 0 {
		input.Data = &protected[0]
	}
	var output windows.DataBlob
	var description *uint16
	if err := windows.CryptUnprotectData(&input, &description, nil, 0, nil, 0, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if description != nil {
		defer windows.LocalFree(windows.Handle(unsafe.Pointer(description)))
	}
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}
