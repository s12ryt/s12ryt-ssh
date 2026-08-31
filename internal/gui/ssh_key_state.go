package gui

import (
	"fmt"
	"strings"
	"sync"

	"s12ryt-ssh/internal/remote"
)

type sshKeyIdentityFormValues struct {
	ID                  string
	Name                string
	PublicKey           string
	Fingerprint         string
	PrivateKey          string
	KeyPassphrase       string
	ClearSecretMaterial bool
	Enabled             bool
	HasPassphrase       bool
}

func (values sshKeyIdentityFormValues) input() (remote.SSHKeyIdentityInput, error) {
	name := strings.TrimSpace(values.Name)
	if name == "" {
		return remote.SSHKeyIdentityInput{}, fmt.Errorf("Key name is required.")
	}
	privateKey := values.PrivateKey
	if values.ID == "" && strings.TrimSpace(privateKey) == "" {
		return remote.SSHKeyIdentityInput{}, fmt.Errorf("Private key is required.")
	}
	enabled := values.Enabled
	return remote.SSHKeyIdentityInput{
		Name:                name,
		PublicKey:           strings.TrimSpace(values.PublicKey),
		Fingerprint:         strings.TrimSpace(values.Fingerprint),
		PrivateKey:          privateKey,
		KeyPassphrase:       values.KeyPassphrase,
		Enabled:             &enabled,
		ClearSecretMaterial: values.ClearSecretMaterial,
	}, nil
}

type sshKeyIdentityEntry struct {
	Key   remote.SSHKeyIdentity
	Error string
}

func sshKeyIdentityManagementSources() []string {
	return []string{"New key", "Edit key", "Delete key?"}
}

type sshKeyIdentityStore struct {
	mu      sync.RWMutex
	entries []sshKeyIdentityEntry
}

func newSSHKeyIdentityStore() *sshKeyIdentityStore {
	return &sshKeyIdentityStore{}
}

func (store *sshKeyIdentityStore) replace(keys []remote.SSHKeyIdentity) {
	if store == nil {
		return
	}
	entries := make([]sshKeyIdentityEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, sshKeyIdentityEntry{Key: key})
	}
	store.mu.Lock()
	store.entries = entries
	store.mu.Unlock()
}

func (store *sshKeyIdentityStore) upsert(key remote.SSHKeyIdentity) {
	if store == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		if store.entries[index].Key.ID != key.ID {
			continue
		}
		store.entries[index] = sshKeyIdentityEntry{Key: key}
		return
	}
	store.entries = append(store.entries, sshKeyIdentityEntry{Key: key})
}

func (store *sshKeyIdentityStore) remove(id string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index, entry := range store.entries {
		if entry.Key.ID != id {
			continue
		}
		store.entries = append(store.entries[:index], store.entries[index+1:]...)
		return true
	}
	return false
}

func (store *sshKeyIdentityStore) snapshot() []sshKeyIdentityEntry {
	if store == nil {
		return nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return append([]sshKeyIdentityEntry(nil), store.entries...)
}

func filterSSHKeyIdentities(keys []remote.SSHKeyIdentity, query string) []remote.SSHKeyIdentity {
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]remote.SSHKeyIdentity, 0, len(keys))
	for _, key := range keys {
		if needle != "" &&
			!strings.Contains(strings.ToLower(key.Name), needle) &&
			!strings.Contains(strings.ToLower(key.PublicKey), needle) &&
			!strings.Contains(strings.ToLower(key.Fingerprint), needle) {
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}

func filteredSSHKeyIdentityEntries(entries []sshKeyIdentityEntry, query string) []sshKeyIdentityEntry {
	keys := make([]remote.SSHKeyIdentity, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	filteredKeys := filterSSHKeyIdentities(keys, query)
	filtered := make([]sshKeyIdentityEntry, 0, len(filteredKeys))
	for _, key := range filteredKeys {
		for _, entry := range entries {
			if entry.Key.ID == key.ID {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered
}
