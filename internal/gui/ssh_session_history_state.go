package gui

import (
	"fmt"
	"strings"
	"sync"

	"s12ryt-ssh/internal/remote"
)

type sshSessionHistoryStore struct {
	mu      sync.RWMutex
	records []remote.SSHSessionHistory
}

type sshSessionHistoryDetail struct {
	Source string
	Value  string
	Danger bool
}

type sshSessionHistoryAttempt struct {
	token       uint64
	historyID   string
	status      remote.SSHSessionHistoryStatus
	latencyMS   int
	error       string
	endedAt     int64
	hasEndedAt  bool
	hostID      string
	hostName    string
	startedAtMS int64
}

type sshSessionHistoryUpdate struct {
	ID           string
	Status       remote.SSHSessionHistoryStatus
	LatencyMS    int
	ErrorMessage string
	EndedAt      *int64
}

type sshSessionHistoryTracker struct {
	mu      sync.Mutex
	next    uint64
	current *sshSessionHistoryAttempt
}

func newSSHSessionHistoryTracker() *sshSessionHistoryTracker {
	return &sshSessionHistoryTracker{}
}

func (t *sshSessionHistoryTracker) begin(hostID, hostName string, startedAtMS int64) *sshSessionHistoryAttempt {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	attempt := &sshSessionHistoryAttempt{
		token:       t.next,
		status:      remote.SSHSessionConnecting,
		hostID:      hostID,
		hostName:    hostName,
		startedAtMS: startedAtMS,
	}
	t.current = attempt
	return attempt
}

func (t *sshSessionHistoryTracker) markCreated(attempt *sshSessionHistoryAttempt, historyID string) (sshSessionHistoryUpdate, bool) {
	if t == nil || attempt == nil || strings.TrimSpace(historyID) == "" {
		return sshSessionHistoryUpdate{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != attempt {
		return sshSessionHistoryUpdate{}, false
	}
	attempt.historyID = historyID
	if attempt.status == remote.SSHSessionConnecting {
		return sshSessionHistoryUpdate{}, true
	}
	return attempt.update(), true
}

func (t *sshSessionHistoryTracker) finish(attempt *sshSessionHistoryAttempt, status remote.SSHSessionHistoryStatus, latencyMS int, message string, endedAtMS int64) (sshSessionHistoryUpdate, bool) {
	if t == nil || attempt == nil {
		return sshSessionHistoryUpdate{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != attempt {
		return sshSessionHistoryUpdate{}, false
	}
	attempt.status = status
	attempt.latencyMS = latencyMS
	attempt.error = message
	attempt.hasEndedAt = endedAtMS > 0
	attempt.endedAt = endedAtMS
	if attempt.historyID == "" {
		return sshSessionHistoryUpdate{}, false
	}
	return attempt.update(), true
}

func (attempt *sshSessionHistoryAttempt) update() sshSessionHistoryUpdate {
	update := sshSessionHistoryUpdate{
		ID:           attempt.historyID,
		Status:       attempt.status,
		LatencyMS:    attempt.latencyMS,
		ErrorMessage: attempt.error,
	}
	if attempt.hasEndedAt {
		endedAt := attempt.endedAt
		update.EndedAt = &endedAt
	}
	return update
}

func newSSHSessionHistoryStore() *sshSessionHistoryStore {
	return &sshSessionHistoryStore{}
}

func (s *sshSessionHistoryStore) replace(records []remote.SSHSessionHistory) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.records = cloneSSHSessionHistory(records)
	s.mu.Unlock()
}

func (s *sshSessionHistoryStore) snapshot() []remote.SSHSessionHistory {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	records := cloneSSHSessionHistory(s.records)
	s.mu.RUnlock()
	return records
}

func cloneSSHSessionHistory(records []remote.SSHSessionHistory) []remote.SSHSessionHistory {
	if len(records) == 0 {
		return nil
	}
	cloned := make([]remote.SSHSessionHistory, len(records))
	copy(cloned, records)
	for index, record := range records {
		if record.EndedAt != nil {
			endedAt := *record.EndedAt
			cloned[index].EndedAt = &endedAt
		}
	}
	return cloned
}

func filterSSHSessionHistory(records []remote.SSHSessionHistory, query string) []remote.SSHSessionHistory {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return cloneSSHSessionHistory(records)
	}
	filtered := make([]remote.SSHSessionHistory, 0, len(records))
	for _, record := range records {
		values := []string{record.HostName, record.HostID, string(record.Status), record.ErrorMessage}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), query) {
				filtered = append(filtered, cloneSSHSessionHistory([]remote.SSHSessionHistory{record})[0])
				break
			}
		}
	}
	return filtered
}

func sshSessionHistoryStatusSource(status remote.SSHSessionHistoryStatus) string {
	switch status {
	case remote.SSHSessionConnecting:
		return "Connecting"
	case remote.SSHSessionConnected:
		return "Connected"
	case remote.SSHSessionFailed:
		return "Failed"
	case remote.SSHSessionClosed:
		return "Closed"
	default:
		return string(status)
	}
}

func sshSessionHistoryDetails(record remote.SSHSessionHistory) []sshSessionHistoryDetail {
	details := []sshSessionHistoryDetail{
		{Source: "Started", Value: formatFingerprintTimestamp(record.StartedAt)},
	}
	if record.LatencyMS > 0 {
		details = append(details, sshSessionHistoryDetail{
			Source: "Latency",
			Value:  fmt.Sprintf("%d ms", record.LatencyMS),
		})
	}
	if record.EndedAt != nil {
		details = append(details, sshSessionHistoryDetail{
			Source: "Ended",
			Value:  formatFingerprintTimestamp(*record.EndedAt),
		})
	}
	if message := strings.TrimSpace(record.ErrorMessage); message != "" {
		details = append(details, sshSessionHistoryDetail{
			Source: "Error details",
			Value:  message,
			Danger: true,
		})
	}
	return details
}
