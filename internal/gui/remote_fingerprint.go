package gui

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
)

const maxFingerprintHistoryHosts = 50

type remoteFingerprintSession interface {
	SSHHostFingerprints(context.Context, string) ([]remote.SSHHostFingerprint, error)
	SetSSHHostFingerprintWithSource(context.Context, string, string, remote.SSHHostFingerprintSource) error
	ClearSSHHostFingerprint(context.Context, string) error
}

func (ui *Window) refreshSSHHostFingerprints() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteFingerprintSession)
	if !ok {
		ui.model.Error = "SSH host fingerprint history service is unavailable"
		return false
	}
	if ui.busy {
		return false
	}
	if ui.sshFingerprints == nil {
		ui.sshFingerprints = newSSHHostFingerprintStore()
	}
	hosts := append([]remote.SSHHost(nil), ui.sshHosts...)
	if len(hosts) > maxFingerprintHistoryHosts {
		hosts = hosts[:maxFingerprintHistoryHosts]
	}
	if len(hosts) == 0 {
		ui.sshFingerprints.replace(nil)
		return true
	}
	ui.async("Loading SSH host fingerprints...", func(ctx context.Context) (func(), error) {
		entries := make([]sshHostFingerprintEntry, 0, len(hosts))
		for _, host := range hosts {
			history, err := session.SSHHostFingerprints(ctx, host.ID)
			if err != nil {
				return nil, fmt.Errorf("load SSH host fingerprint history for %s: %w", host.Name, err)
			}
			entries = append(entries, sshHostFingerprintEntry{Host: host, History: history})
		}
		return func() { ui.sshFingerprints.replace(entries) }, nil
	})
	return true
}

func (ui *Window) optionalFingerprintSession() (remoteFingerprintSession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote SSH session is unavailable")
	}
	session, ok := ui.model.RemoteSession.(remoteFingerprintSession)
	if !ok {
		return nil, errors.New("SSH host fingerprint history service is unavailable")
	}
	return session, nil
}

func (ui *Window) clearSSHHostFingerprintView() {
	if ui == nil {
		return
	}
	if ui.sshFingerprints != nil {
		ui.sshFingerprints.replace(nil)
	}
	ui.sshFingerprintVisibleIDs = nil
	ui.sshFingerprintManualBtns = nil
	ui.sshFingerprintClearBtns = nil
	ui.sshFingerprintCopyBtns = nil
	ui.sshFingerprintCopyValues = nil
	ui.closeManualSSHHostFingerprint()
}

func (ui *Window) syncSSHHostFingerprintButtons(entries []sshHostFingerprintEntry) {
	if ui == nil {
		return
	}
	visibleIDs := make([]string, len(entries))
	copyCount := 0
	for index, entry := range entries {
		visibleIDs[index] = entry.Host.ID
		copyCount += len(entry.History)
	}
	copyValues := make([]string, 0, copyCount)
	for _, entry := range entries {
		for _, fingerprint := range entry.History {
			copyValues = append(copyValues, fingerprint.Fingerprint)
		}
	}
	if slices.Equal(ui.sshFingerprintVisibleIDs, visibleIDs) && slices.Equal(ui.sshFingerprintCopyValues, copyValues) {
		return
	}
	ui.sshFingerprintVisibleIDs = visibleIDs
	ui.sshFingerprintManualBtns = make([]widget.Clickable, len(entries))
	ui.sshFingerprintClearBtns = make([]widget.Clickable, len(entries))
	ui.sshFingerprintCopyBtns = make([]widget.Clickable, copyCount)
	ui.sshFingerprintCopyValues = copyValues
}

func (ui *Window) openManualSSHHostFingerprint(hostID string) bool {
	if ui == nil || ui.sshFingerprints == nil || ui.busy {
		return false
	}
	if _, ok := ui.sshHostFingerprintEntry(hostID); !ok {
		return false
	}
	ui.sshFingerprintManualOpen = true
	ui.sshFingerprintManualHostID = hostID
	ui.sshFingerprintManualEditor.SetText("")
	ui.sshFingerprintManualEditor.SingleLine = true
	ui.model.Error = ""
	return true
}

func (ui *Window) closeManualSSHHostFingerprint() {
	if ui == nil {
		return
	}
	ui.sshFingerprintManualOpen = false
	ui.sshFingerprintManualHostID = ""
	ui.sshFingerprintManualEditor.SetText("")
	ui.sshFingerprintManualClose = widget.Clickable{}
	ui.sshFingerprintManualCancel = widget.Clickable{}
	ui.sshFingerprintManualSave = widget.Clickable{}
	ui.sshFingerprintManualScrim = widget.Clickable{}
}

func (ui *Window) submitManualSSHHostFingerprint() bool {
	if ui == nil || !ui.sshFingerprintManualOpen || ui.busy {
		return false
	}
	fingerprint, err := validateManualSSHHostFingerprint(ui.sshFingerprintManualEditor.Text())
	if err != nil {
		ui.model.Error = ui.text(err.Error())
		return false
	}
	session, err := ui.optionalFingerprintSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	hostID := ui.sshFingerprintManualHostID
	if _, ok := ui.sshHostFingerprintEntry(hostID); !ok {
		return false
	}
	ui.closeManualSSHHostFingerprint()
	ui.async(ui.text("Saving host fingerprint..."), func(ctx context.Context) (func(), error) {
		if err := session.SetSSHHostFingerprintWithSource(ctx, hostID, fingerprint, remote.SSHHostFingerprintManual); err != nil {
			return nil, err
		}
		return func() { ui.refreshSSHHostFingerprints() }, nil
	})
	return true
}

func (ui *Window) clearTrustedSSHHostFingerprint(hostID string) bool {
	if ui == nil || ui.sshFingerprints == nil || ui.busy {
		return false
	}
	if _, ok := ui.sshHostFingerprintEntry(hostID); !ok {
		return false
	}
	session, err := ui.optionalFingerprintSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	ui.requestConfirm(ui.text("Clear trusted fingerprint?"), ui.text("This host will require TOFU confirmation on the next connection."), func() {
		ui.async(ui.text("Clearing host fingerprint..."), func(ctx context.Context) (func(), error) {
			if err := session.ClearSSHHostFingerprint(ctx, hostID); err != nil {
				return nil, err
			}
			return func() { ui.refreshSSHHostFingerprints() }, nil
		})
	})
	return true
}

func (ui *Window) sshHostFingerprintEntry(hostID string) (sshHostFingerprintEntry, bool) {
	if ui == nil || ui.sshFingerprints == nil {
		return sshHostFingerprintEntry{}, false
	}
	for _, entry := range ui.sshFingerprints.snapshot() {
		if entry.Host.ID == hostID {
			return entry, true
		}
	}
	return sshHostFingerprintEntry{}, false
}
