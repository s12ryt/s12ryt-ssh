package gui

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"s12ryt-ssh/internal/remote"
)

type sshCommandSnippetFormValues struct {
	ID               string
	Name             string
	Command          string
	VariablesText    string
	SecretValuesText string
	ClearSecrets     bool
	Enabled          bool
}

var sshCommandSnippetName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (values sshCommandSnippetFormValues) input() (remote.SSHCommandSnippetInput, error) {
	name := strings.TrimSpace(values.Name)
	if name == "" {
		return remote.SSHCommandSnippetInput{}, fmt.Errorf("Snippet name is required.")
	}
	command := strings.TrimSpace(values.Command)
	if command == "" {
		return remote.SSHCommandSnippetInput{}, fmt.Errorf("Command is required.")
	}
	variables, err := parseSSHCommandSnippetNames(values.VariablesText)
	if err != nil {
		return remote.SSHCommandSnippetInput{}, err
	}
	secrets, err := parseSSHCommandSnippetSecrets(values.SecretValuesText)
	if err != nil {
		return remote.SSHCommandSnippetInput{}, err
	}
	if values.ClearSecrets {
		secrets = map[string]string{}
	}
	enabled := values.Enabled
	return remote.SSHCommandSnippetInput{
		Name:      name,
		Command:   command,
		Variables: variables,
		Secrets:   secrets,
		Enabled:   &enabled,
	}, nil
}

func parseSSHCommandSnippetNames(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, "\r", "")
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' })
	names := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if !sshCommandSnippetName.MatchString(name) {
			return nil, fmt.Errorf("Variable name %q is invalid.", name)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("Duplicate variable name.")
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

func parseSSHCommandSnippetSecrets(raw string) (map[string]string, error) {
	raw = strings.ReplaceAll(raw, "\r", "")
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	secrets := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || !sshCommandSnippetName.MatchString(name) || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("Secret entry must use NAME=value.")
		}
		if _, exists := secrets[name]; exists {
			return nil, fmt.Errorf("Duplicate secret name.")
		}
		secrets[name] = value
	}
	return secrets, nil
}

type sshCommandSnippetEntry struct {
	Snippet remote.SSHCommandSnippet
	Error   string
}

func sshCommandSnippetManagementSources() []string {
	return []string{"New snippet", "Edit snippet", "Delete snippet?"}
}

type sshCommandSnippetStore struct {
	mu      sync.RWMutex
	entries []sshCommandSnippetEntry
}

func newSSHCommandSnippetStore() *sshCommandSnippetStore {
	return &sshCommandSnippetStore{}
}

func (store *sshCommandSnippetStore) replace(snippets []remote.SSHCommandSnippet) {
	if store == nil {
		return
	}
	entries := make([]sshCommandSnippetEntry, 0, len(snippets))
	for _, snippet := range snippets {
		entries = append(entries, sshCommandSnippetEntry{Snippet: cloneSSHCommandSnippet(snippet)})
	}
	store.mu.Lock()
	store.entries = entries
	store.mu.Unlock()
}

func (store *sshCommandSnippetStore) upsert(snippet remote.SSHCommandSnippet) {
	if store == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		if store.entries[index].Snippet.ID != snippet.ID {
			continue
		}
		store.entries[index] = sshCommandSnippetEntry{Snippet: cloneSSHCommandSnippet(snippet)}
		return
	}
	store.entries = append(store.entries, sshCommandSnippetEntry{Snippet: cloneSSHCommandSnippet(snippet)})
}

func (store *sshCommandSnippetStore) remove(id string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index, entry := range store.entries {
		if entry.Snippet.ID != id {
			continue
		}
		store.entries = append(store.entries[:index], store.entries[index+1:]...)
		return true
	}
	return false
}

func (store *sshCommandSnippetStore) snapshot() []sshCommandSnippetEntry {
	if store == nil {
		return nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	entries := make([]sshCommandSnippetEntry, 0, len(store.entries))
	for _, entry := range store.entries {
		entries = append(entries, sshCommandSnippetEntry{
			Snippet: cloneSSHCommandSnippet(entry.Snippet),
			Error:   entry.Error,
		})
	}
	return entries
}

func cloneSSHCommandSnippet(snippet remote.SSHCommandSnippet) remote.SSHCommandSnippet {
	snippet.Variables = append([]string(nil), snippet.Variables...)
	snippet.SecretNames = append([]string(nil), snippet.SecretNames...)
	return snippet
}

func filterSSHCommandSnippets(snippets []remote.SSHCommandSnippet, query string) []remote.SSHCommandSnippet {
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]remote.SSHCommandSnippet, 0, len(snippets))
	for _, snippet := range snippets {
		if needle != "" {
			matched := false
			values := []string{snippet.Name, snippet.Command}
			values = append(values, snippet.Variables...)
			values = append(values, snippet.SecretNames...)
			for _, value := range values {
				if strings.Contains(strings.ToLower(value), needle) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, cloneSSHCommandSnippet(snippet))
	}
	return filtered
}

var sshCommandPlaceholder = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandSSHCommandSnippet(snippet remote.SSHCommandSnippet, variables, secrets map[string]string) (string, error) {
	allowed := make(map[string]struct{}, len(snippet.Variables)+len(snippet.SecretNames))
	for _, name := range snippet.Variables {
		allowed[name] = struct{}{}
	}
	for _, name := range snippet.SecretNames {
		allowed[name] = struct{}{}
	}

	var expansionErr error
	command := sshCommandPlaceholder.ReplaceAllStringFunc(snippet.Command, func(match string) string {
		if expansionErr != nil {
			return match
		}
		parts := sshCommandPlaceholder.FindStringSubmatch(match)
		name := parts[1]
		if _, ok := allowed[name]; !ok {
			expansionErr = fmt.Errorf("snippet variable %q is not declared", name)
			return match
		}
		if value, ok := variables[name]; ok {
			return value
		}
		if value, ok := secrets[name]; ok {
			return value
		}
		expansionErr = fmt.Errorf("snippet variable %q is missing", name)
		return match
	})
	if expansionErr != nil {
		return "", expansionErr
	}
	return command, nil
}
