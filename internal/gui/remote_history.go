package gui

import (
	"context"
	"errors"
	"time"

	"s12ryt-ssh/internal/remote"
)

type remoteSessionHistorySession interface {
	SSHSessionHistory(context.Context) ([]remote.SSHSessionHistory, error)
}

type remoteSessionHistoryCRUD interface {
	remoteSessionHistorySession
	CreateSSHSessionHistory(context.Context, remote.SSHSessionHistoryInput) (remote.SSHSessionHistory, error)
	UpdateSSHSessionHistory(context.Context, string, remote.SSHSessionHistoryUpdate) (remote.SSHSessionHistory, error)
}

func (ui *Window) startSSHSessionHistory(tab *sshTab) bool {
	if ui == nil || ui.model == nil || tab == nil || tab.Local || tab.HostID == "" {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteSessionHistoryCRUD)
	if !ok {
		return false
	}
	if tab.history == nil {
		tab.history = newSSHSessionHistoryTracker()
	}
	attempt := tab.history.begin(tab.HostID, tab.HostName, time.Now().UnixMilli())
	tab.historyAttempt = attempt
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		record, err := session.CreateSSHSessionHistory(ctx, remote.SSHSessionHistoryInput{
			HostID: tab.HostID,
			Status: remote.SSHSessionConnecting,
		})
		if err != nil || record.ID == "" {
			return
		}
		update, current := tab.history.markCreated(attempt, record.ID)
		if !current {
			update = sshSessionHistoryUpdate{ID: record.ID, Status: remote.SSHSessionClosed}
		}
		if update.Status != remote.SSHSessionConnecting {
			sendSSHSessionHistoryUpdate(session, update)
		}
	}()
	return true
}

func (ui *Window) finishSSHSessionHistory(tab *sshTab, status remote.SSHSessionHistoryStatus, message string) {
	if ui == nil || ui.model == nil || tab == nil || tab.Local || tab.history == nil || tab.historyAttempt == nil {
		return
	}
	session, ok := ui.model.RemoteSession.(remoteSessionHistoryCRUD)
	if !ok {
		return
	}
	latency := int(time.Now().UnixMilli() - tab.historyAttempt.startedAtMS)
	if latency < 0 {
		latency = 0
	}
	endedAt := int64(0)
	if status == remote.SSHSessionFailed || status == remote.SSHSessionClosed {
		endedAt = time.Now().UnixMilli()
	}
	update, ok := tab.history.finish(tab.historyAttempt, status, latency, message, endedAt)
	if ok {
		go sendSSHSessionHistoryUpdate(session, update)
	}
}

func (ui *Window) finishOtherSSHTabHistory(activeID string) {
	if ui == nil {
		return
	}
	for _, tab := range ui.sshTabs.tabs {
		if tab != nil && tab.ID != activeID {
			ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
		}
	}
}

func (ui *Window) finishAllSSHTabHistory() {
	if ui == nil {
		return
	}
	for _, tab := range ui.sshTabs.tabs {
		if tab != nil {
			ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
		}
	}
}

func sendSSHSessionHistoryUpdate(session remoteSessionHistoryCRUD, update sshSessionHistoryUpdate) {
	if session == nil || update.ID == "" {
		return
	}
	latency := update.LatencyMS
	errorMessage := update.ErrorMessage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _ = session.UpdateSSHSessionHistory(ctx, update.ID, remote.SSHSessionHistoryUpdate{
		Status:       update.Status,
		LatencyMS:    &latency,
		ErrorMessage: &errorMessage,
	})
}

func (ui *Window) refreshSSHSessionHistory() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteSessionHistorySession)
	if !ok {
		ui.model.Error = "SSH session history service is unavailable"
		return false
	}
	if ui.busy {
		return false
	}
	if ui.sshHistory == nil {
		ui.sshHistory = newSSHSessionHistoryStore()
	}
	ui.async("Loading SSH session history...", func(ctx context.Context) (func(), error) {
		records, err := session.SSHSessionHistory(ctx)
		if err != nil {
			return nil, err
		}
		return func() { ui.sshHistory.replace(records) }, nil
	})
	return true
}

func (ui *Window) optionalSessionHistorySession() (remoteSessionHistorySession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote SSH session is unavailable")
	}
	session, ok := ui.model.RemoteSession.(remoteSessionHistorySession)
	if !ok {
		return nil, errors.New("SSH session history service is unavailable")
	}
	return session, nil
}

func (ui *Window) clearSSHSessionHistoryView() {
	if ui == nil {
		return
	}
	if ui.sshHistory != nil {
		ui.sshHistory.replace(nil)
	}
}
