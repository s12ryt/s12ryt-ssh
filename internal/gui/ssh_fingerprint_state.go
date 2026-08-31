package gui

import (
	"errors"
	"strings"
	"sync"

	"s12ryt-ssh/internal/remote"
)

type sshHostFingerprintEntry struct {
	Host    remote.SSHHost
	History []remote.SSHHostFingerprint
}

type sshHostFingerprintStore struct {
	mu      sync.RWMutex
	entries []sshHostFingerprintEntry
}

func newSSHHostFingerprintStore() *sshHostFingerprintStore {
	return &sshHostFingerprintStore{}
}

func (s *sshHostFingerprintStore) replace(entries []sshHostFingerprintEntry) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.entries = cloneSSHHostFingerprintEntries(entries)
	s.mu.Unlock()
}

func (s *sshHostFingerprintStore) snapshot() []sshHostFingerprintEntry {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	entries := cloneSSHHostFingerprintEntries(s.entries)
	s.mu.RUnlock()
	return entries
}

func cloneSSHHostFingerprintEntries(entries []sshHostFingerprintEntry) []sshHostFingerprintEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]sshHostFingerprintEntry, len(entries))
	for i, entry := range entries {
		cloned[i].Host = cloneFingerprintHost(entry.Host)
		cloned[i].History = make([]remote.SSHHostFingerprint, len(entry.History))
		copy(cloned[i].History, entry.History)
		for j, fingerprint := range entry.History {
			if fingerprint.RetiredAt != nil {
				retiredAt := *fingerprint.RetiredAt
				cloned[i].History[j].RetiredAt = &retiredAt
			}
		}
	}
	return cloned
}

func cloneFingerprintHost(host remote.SSHHost) remote.SSHHost {
	host.Tags = append([]string(nil), host.Tags...)
	if host.Settings.Environment != nil {
		host.Settings.Environment = make(map[string]string, len(host.Settings.Environment))
		for name, value := range host.Settings.Environment {
			host.Settings.Environment[name] = value
		}
	}
	return host
}

func filterSSHHostFingerprintEntries(entries []sshHostFingerprintEntry, query string) []sshHostFingerprintEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return cloneSSHHostFingerprintEntries(entries)
	}
	filtered := make([]sshHostFingerprintEntry, 0, len(entries))
	for _, entry := range entries {
		searchable := []string{
			entry.Host.Name,
			entry.Host.Host,
			entry.Host.TrustedFingerprint,
		}
		for _, fingerprint := range entry.History {
			searchable = append(searchable, fingerprint.Algorithm, fingerprint.Fingerprint, string(fingerprint.Source))
		}
		for _, value := range searchable {
			if strings.Contains(strings.ToLower(value), query) {
				filtered = append(filtered, cloneSSHHostFingerprintEntries([]sshHostFingerprintEntry{entry})[0])
				break
			}
		}
	}
	return filtered
}

func validateManualSSHHostFingerprint(value string) (string, error) {
	value = strings.TrimSpace(value)
	algorithm, digest, found := strings.Cut(value, ":")
	algorithm = strings.ToUpper(strings.TrimSpace(algorithm))
	digest = strings.TrimSpace(digest)
	if !found || algorithm == "" || digest == "" {
		return "", errors.New("Fingerprint must include an algorithm and digest.")
	}
	return algorithm + ":" + digest, nil
}
