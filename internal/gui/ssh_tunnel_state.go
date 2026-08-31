package gui

import (
	"net"
	"strings"
	"sync"
	"time"

	"s12ryt-ssh/internal/remote"
)

// sshTunnelRuntime is the local lifetime of one forwarding listener. The
// saved rule remains remote state; this interface only owns its live process.
type sshTunnelRuntime interface {
	Addr() net.Addr
	Traffic() (up, down int64)
	Close() error
}

type sshTunnelEntry struct {
	Rule               remote.SSHTunnelRule
	Runtime            sshTunnelRuntime
	Error              string
	Starting           bool
	RuntimeSyncing     bool
	LastRuntimeSync    time.Time
	LastSyncedUp       int64
	LastSyncedDown     int64
	PendingRuntimeSync remote.SSHTunnelRuntimeUpdate
}

const sshTunnelRuntimeSyncInterval = 5 * time.Second

type sshTunnelFormValues struct {
	ID         string
	Name       string
	HostID     string
	Type       remote.SSHTunnelType
	ListenHost string
	ListenPort int
	TargetHost string
	TargetPort int
	Enabled    bool
	AutoStart  bool
}

func sshTunnelFormFromRule(rule remote.SSHTunnelRule) sshTunnelFormValues {
	return sshTunnelFormValues{
		ID:         rule.ID,
		Name:       rule.Name,
		HostID:     rule.HostID,
		Type:       rule.Type,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		AutoStart:  rule.AutoStart,
	}
}

func (values sshTunnelFormValues) input() remote.SSHTunnelInput {
	return remote.SSHTunnelInput{
		Name:       strings.TrimSpace(values.Name),
		HostID:     strings.TrimSpace(values.HostID),
		Type:       values.Type,
		ListenHost: strings.TrimSpace(values.ListenHost),
		ListenPort: values.ListenPort,
		TargetHost: strings.TrimSpace(values.TargetHost),
		TargetPort: values.TargetPort,
		Enabled:    values.Enabled,
		AutoStart:  values.AutoStart,
	}
}

func validateSSHTunnelInput(input remote.SSHTunnelInput) string {
	if strings.TrimSpace(input.Name) == "" {
		return "Tunnel name is required."
	}
	if strings.TrimSpace(input.HostID) == "" {
		return "Tunnel host is required."
	}
	if input.Type != remote.SSHTunnelLocal && input.Type != remote.SSHTunnelRemote && input.Type != remote.SSHTunnelDynamic {
		return "Tunnel type is invalid."
	}
	if strings.TrimSpace(input.ListenHost) == "" {
		return "Listen host is required."
	}
	if input.ListenPort < 1 || input.ListenPort > 65535 {
		return "Listen port must be between 1 and 65535."
	}
	if input.Type == remote.SSHTunnelDynamic {
		if input.TargetPort < 0 || input.TargetPort > 65535 {
			return "Target port must be between 0 and 65535."
		}
		return ""
	}
	if strings.TrimSpace(input.TargetHost) == "" {
		return "Target host is required."
	}
	if input.TargetPort < 1 || input.TargetPort > 65535 {
		return "Target port must be between 1 and 65535."
	}
	return ""
}

type sshTunnelStore struct {
	mu      sync.RWMutex
	entries []sshTunnelEntry
}

func newSSHTunnelStore() *sshTunnelStore {
	return &sshTunnelStore{}
}

func (store *sshTunnelStore) replace(rules []remote.SSHTunnelRule) {
	if store == nil {
		return
	}
	store.mu.Lock()
	previous := make(map[string]sshTunnelEntry, len(store.entries))
	for _, entry := range store.entries {
		previous[entry.Rule.ID] = entry
	}
	updated := make([]sshTunnelEntry, 0, len(rules))
	kept := make(map[string]bool, len(rules))
	for _, rule := range rules {
		entry := sshTunnelEntry{Rule: rule}
		if old, ok := previous[rule.ID]; ok {
			entry.Runtime = old.Runtime
			entry.Error = old.Error
			entry.Starting = old.Starting
			entry.RuntimeSyncing = old.RuntimeSyncing
			entry.LastRuntimeSync = old.LastRuntimeSync
			entry.LastSyncedUp = old.LastSyncedUp
			entry.LastSyncedDown = old.LastSyncedDown
			entry.PendingRuntimeSync = old.PendingRuntimeSync
			if entry.Runtime != nil {
				entry.Rule.Running = true
			}
		}
		updated = append(updated, entry)
		kept[rule.ID] = true
	}
	removed := make([]sshTunnelRuntime, 0)
	for _, entry := range store.entries {
		if entry.Runtime != nil && !kept[entry.Rule.ID] {
			removed = append(removed, entry.Runtime)
		}
	}
	store.entries = updated
	store.mu.Unlock()
	closeSSHTunnelRuntimes(removed)
}

func (store *sshTunnelStore) attachRuntime(id string, runtime sshTunnelRuntime) bool {
	if store == nil || runtime == nil {
		return false
	}
	store.mu.Lock()
	for index := range store.entries {
		if store.entries[index].Rule.ID != id {
			continue
		}
		previous := store.entries[index].Runtime
		store.entries[index].Runtime = runtime
		store.entries[index].Rule.Running = true
		store.entries[index].Error = ""
		store.entries[index].Starting = false
		store.mu.Unlock()
		if previous != nil && previous != runtime {
			_ = previous.Close()
		}
		return true
	}
	store.mu.Unlock()
	_ = runtime.Close()
	return false
}

func (store *sshTunnelStore) attachStartingRuntime(id string, runtime sshTunnelRuntime) bool {
	if store == nil || runtime == nil {
		return false
	}
	store.mu.Lock()
	for index := range store.entries {
		entry := &store.entries[index]
		if entry.Rule.ID != id || !entry.Starting {
			continue
		}
		previous := entry.Runtime
		entry.Runtime = runtime
		entry.Rule.Running = true
		entry.Error = ""
		entry.Starting = false
		store.mu.Unlock()
		if previous != nil && previous != runtime {
			_ = previous.Close()
		}
		return true
	}
	store.mu.Unlock()
	_ = runtime.Close()
	return false
}

func (store *sshTunnelStore) setError(id, message string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		if store.entries[index].Rule.ID != id {
			continue
		}
		store.entries[index].Rule.Running = false
		store.entries[index].Error = message
		store.entries[index].Starting = false
		return true
	}
	return false
}

func (store *sshTunnelStore) setStartingError(id, message string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		if store.entries[index].Rule.ID != id || !store.entries[index].Starting {
			continue
		}
		store.entries[index].Rule.Running = false
		store.entries[index].Error = message
		store.entries[index].Starting = false
		return true
	}
	return false
}

func (store *sshTunnelStore) setStarting(id string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		entry := &store.entries[index]
		if entry.Rule.ID != id || entry.Runtime != nil || entry.Starting {
			continue
		}
		entry.Starting = true
		entry.Rule.Running = false
		entry.Error = ""
		return true
	}
	return false
}

func (store *sshTunnelStore) stop(id string) bool {
	_, ok := store.stopWithRuntimeUpdate(id)
	return ok
}

func (store *sshTunnelStore) stopWithRuntimeUpdate(id string) (remote.SSHTunnelRuntimeUpdate, bool) {
	if store == nil {
		return remote.SSHTunnelRuntimeUpdate{}, false
	}
	store.mu.Lock()
	for index := range store.entries {
		if store.entries[index].Rule.ID != id {
			continue
		}
		runtime := store.entries[index].Runtime
		up, down := store.entries[index].Rule.TrafficUpBytes, store.entries[index].Rule.TrafficDownBytes
		if runtime != nil {
			up, down = runtime.Traffic()
		}
		store.entries[index].Runtime = nil
		store.entries[index].Rule.Running = false
		store.entries[index].Rule.TrafficUpBytes = up
		store.entries[index].Rule.TrafficDownBytes = down
		store.entries[index].Error = ""
		store.entries[index].Starting = false
		store.entries[index].RuntimeSyncing = false
		store.entries[index].PendingRuntimeSync = remote.SSHTunnelRuntimeUpdate{}
		store.mu.Unlock()
		if runtime != nil {
			_ = runtime.Close()
		}
		return remote.SSHTunnelRuntimeUpdate{Running: false, TrafficUpBytes: up, TrafficDownBytes: down}, true
	}
	store.mu.Unlock()
	return remote.SSHTunnelRuntimeUpdate{}, false
}

func (store *sshTunnelStore) prepareRuntimeSync(id string, now time.Time, force, running bool) (remote.SSHTunnelRuntimeUpdate, bool) {
	if store == nil {
		return remote.SSHTunnelRuntimeUpdate{}, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		entry := &store.entries[index]
		if entry.Rule.ID != id || entry.RuntimeSyncing {
			continue
		}
		up, down := entry.Rule.TrafficUpBytes, entry.Rule.TrafficDownBytes
		if entry.Runtime != nil {
			up, down = entry.Runtime.Traffic()
		}
		if !force {
			if now.Sub(entry.LastRuntimeSync) < sshTunnelRuntimeSyncInterval {
				return remote.SSHTunnelRuntimeUpdate{}, false
			}
			if up == entry.LastSyncedUp && down == entry.LastSyncedDown && running == entry.Rule.Running {
				return remote.SSHTunnelRuntimeUpdate{}, false
			}
		}
		update := remote.SSHTunnelRuntimeUpdate{Running: running, TrafficUpBytes: up, TrafficDownBytes: down}
		entry.RuntimeSyncing = true
		entry.PendingRuntimeSync = update
		return update, true
	}
	return remote.SSHTunnelRuntimeUpdate{}, false
}

func (store *sshTunnelStore) completeRuntimeSync(id string, update remote.SSHTunnelRuntimeUpdate, now time.Time, success bool) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for index := range store.entries {
		entry := &store.entries[index]
		if entry.Rule.ID != id || !entry.RuntimeSyncing || entry.PendingRuntimeSync != update {
			continue
		}
		entry.RuntimeSyncing = false
		entry.PendingRuntimeSync = remote.SSHTunnelRuntimeUpdate{}
		if success {
			entry.LastRuntimeSync = now
			entry.LastSyncedUp = update.TrafficUpBytes
			entry.LastSyncedDown = update.TrafficDownBytes
			entry.Rule.Running = update.Running
			entry.Rule.TrafficUpBytes = update.TrafficUpBytes
			entry.Rule.TrafficDownBytes = update.TrafficDownBytes
		}
		return true
	}
	return false
}

func (store *sshTunnelStore) stopHost(hostID string) int {
	hostID = strings.TrimSpace(hostID)
	if store == nil || hostID == "" {
		return 0
	}
	store.mu.Lock()
	runtimes := make([]sshTunnelRuntime, 0)
	stopped := 0
	for index := range store.entries {
		entry := &store.entries[index]
		if entry.Rule.HostID != hostID || (entry.Runtime == nil && !entry.Starting && !entry.Rule.Running) {
			continue
		}
		if entry.Runtime != nil {
			runtimes = append(runtimes, entry.Runtime)
		}
		entry.Runtime = nil
		entry.Rule.Running = false
		entry.Error = ""
		entry.Starting = false
		stopped++
	}
	store.mu.Unlock()
	closeSSHTunnelRuntimes(runtimes)
	return stopped
}

func (store *sshTunnelStore) remove(id string) bool {
	if store == nil {
		return false
	}
	store.mu.Lock()
	for index, entry := range store.entries {
		if entry.Rule.ID != id {
			continue
		}
		runtime := entry.Runtime
		store.entries = append(store.entries[:index], store.entries[index+1:]...)
		store.mu.Unlock()
		if runtime != nil {
			_ = runtime.Close()
		}
		return true
	}
	store.mu.Unlock()
	return false
}

func (store *sshTunnelStore) get(id string) (sshTunnelEntry, bool) {
	if store == nil {
		return sshTunnelEntry{}, false
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, entry := range store.entries {
		if entry.Rule.ID == id {
			return entry, true
		}
	}
	return sshTunnelEntry{}, false
}

func (store *sshTunnelStore) snapshot() []sshTunnelEntry {
	if store == nil {
		return nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return append([]sshTunnelEntry(nil), store.entries...)
}

func (store *sshTunnelStore) closeAll() {
	if store == nil {
		return
	}
	store.mu.Lock()
	runtimes := make([]sshTunnelRuntime, 0, len(store.entries))
	for _, entry := range store.entries {
		if entry.Runtime != nil {
			runtimes = append(runtimes, entry.Runtime)
		}
	}
	store.entries = nil
	store.mu.Unlock()
	closeSSHTunnelRuntimes(runtimes)
}

func closeSSHTunnelRuntimes(runtimes []sshTunnelRuntime) {
	for _, runtime := range runtimes {
		if runtime != nil {
			_ = runtime.Close()
		}
	}
}

func sshTunnelDirectionSource(tunnelType remote.SSHTunnelType) string {
	switch tunnelType {
	case remote.SSHTunnelRemote:
		return "Remote"
	case remote.SSHTunnelDynamic:
		return "Dynamic SOCKS"
	default:
		return "Local"
	}
}

func sshTunnelStatusSource(running bool, err string) string {
	if err != "" {
		return "Failed"
	}
	if running {
		return "Running"
	}
	return "Stopped"
}

func sshTunnelActionSource(entry sshTunnelEntry) string {
	if entry.Starting {
		return "Starting"
	}
	if entry.Runtime != nil {
		return "Stop"
	}
	return "Start"
}

func sshTunnelEntryStatusSource(entry sshTunnelEntry) string {
	if entry.Starting {
		return "Starting"
	}
	return sshTunnelStatusSource(entry.Runtime != nil, entry.Error)
}
