package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

// sshFingerprintPattern extracts the host key fingerprint that the SSH client
// reports inside the trailing parentheses of ErrHostKeyNotTrusted.
var sshFingerprintPattern = regexp.MustCompile(`\(([^()]+)\)\s*$`)

// sshProfileFromCredentials converts issued remote credentials into the SSH
// client profile shape. Credentials never touch disk and live in memory only.
func sshProfileFromCredentials(creds remote.SSHHostCredentials) config.SSHProfile {
	return config.SSHProfile{
		Name:               creds.Name,
		Host:               creds.Host,
		Port:               creds.Port,
		User:               creds.Username,
		Password:           creds.Password,
		KeyData:            creds.PrivateKey,
		KeyPassphrase:      creds.KeyPassphrase,
		HostKeyFingerprint: creds.TrustedFingerprint,
	}
}

// parsePendingFingerprint reports whether err is a first-connection host key
// rejection and extracts the actual fingerprint that awaits user trust.
func parsePendingFingerprint(err error) (string, bool) {
	if err == nil || !errors.Is(err, sshclient.ErrHostKeyNotTrusted) {
		return "", false
	}
	match := sshFingerprintPattern.FindStringSubmatch(err.Error())
	if len(match) < 2 {
		return "", false
	}
	fingerprint := strings.TrimSpace(match[1])
	if fingerprint == "" {
		return "", false
	}
	return fingerprint, true
}

// applySSHHosts stores the account host list and keeps the selected host
// attached when the refreshed list still contains it.
func (ui *Window) applySSHHosts(hosts []remote.SSHHost) {
	ui.syncSSHHostResourceAvailability(hosts)
	ui.sshHosts = append([]remote.SSHHost(nil), hosts...)
	ui.sshHostIndex = -1
	for i := range ui.sshHosts {
		if ui.sshHosts[i].ID == ui.sshHostID {
			ui.sshHostIndex = i
			break
		}
	}
	ui.rebuildSSHHostFilter()
}

func (ui *Window) syncSSHHostResourceAvailability(hosts []remote.SSHHost) {
	if ui == nil {
		return
	}
	for _, host := range hosts {
		if host.Enabled {
			if ui.sshPool != nil {
				ui.sshPool.setHostEnabled(host.ID, true)
			}
			if ui.transfers != nil {
				ui.transfers.setHostEnabled(host.ID, true, "")
			}
			continue
		}
		ui.disableSSHHostResources(host.ID)
	}
}

func (ui *Window) disableSSHHostResources(hostID string) {
	if ui == nil || strings.TrimSpace(hostID) == "" {
		return
	}
	disabledTabIDs := make(map[string]bool)
	for _, tab := range ui.sshTabs.tabs {
		if tab == nil || tab.Local || tab.HostID != hostID {
			continue
		}
		disabledTabIDs[tab.ID] = true
		ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
	}
	if ui.transfers != nil {
		ui.transfers.setHostEnabled(hostID, false, ui.text("Host is disabled."))
	}
	if ui.sshPool != nil {
		ui.sshPool.setHostEnabled(hostID, false)
	}
	ui.sshTabs.closeHost(hostID)
	if ui.sshTunnels != nil {
		ui.stopSSHHostTunnels(hostID)
	}
	if disabledTabIDs[ui.sshTabRenameID] {
		ui.closeSSHTabRename()
	}
	if disabledTabIDs[ui.sftpOperationTabID] {
		ui.closeSFTPOperation()
	}
	if ui.sshTabDrag.active && disabledTabIDs[ui.sshTabDrag.tabID] {
		ui.sshTabDrag.reset()
	}
	if len(ui.sftpUploadConflicts) > 0 {
		kept := ui.sftpUploadConflicts[:0]
		for _, conflict := range ui.sftpUploadConflicts {
			if !disabledTabIDs[conflict.TabID] {
				kept = append(kept, conflict)
			}
		}
		ui.sftpUploadConflicts = kept
		if len(kept) == 0 {
			ui.closeSFTPUploadConflicts()
		}
	}
}

func (ui *Window) sshHostEnabled(hostID string) bool {
	if ui == nil {
		return false
	}
	for _, host := range ui.sshHosts {
		if host.ID == hostID {
			return host.Enabled
		}
	}
	// A rule can be loaded before the host list. The connection pool still
	// rejects hosts once an explicit disabled record has been observed.
	return true
}

func (ui *Window) selectSSHHost(index int) {
	if index < 0 || index >= len(ui.sshHosts) {
		return
	}
	ui.sshHostIndex = index
	ui.sshHostID = ui.sshHosts[index].ID
	ui.loadSSHHostForm(ui.sshHosts[index])
}

// loadSSHHostForm fills the editor fields from a stored host. Secret fields
// stay empty: leaving them blank preserves the stored credentials.
func (ui *Window) loadSSHHostForm(host remote.SSHHost) {
	ui.sshName.SetText(host.Name)
	ui.sshHost.SetText(host.Host)
	ui.sshPort.SetText(strconv.Itoa(host.Port))
	ui.sshUser.SetText(host.Username)
	ui.sshPassword.SetText("")
	ui.sshPrivateKey.SetText("")
	ui.sshKeyPass.SetText("")
	ui.sshFingerprint.SetText(host.TrustedFingerprint)
	ui.sshFormOriginal = ui.currentSSHFormValues()
}

func (ui *Window) clearSSHHostForm() {
	ui.sshHostIndex = -1
	ui.sshHostID = ""
	ui.sshName.SetText("")
	ui.sshHost.SetText("")
	ui.sshPort.SetText("22")
	ui.sshUser.SetText("")
	ui.sshPassword.SetText("")
	ui.sshPrivateKey.SetText("")
	ui.sshKeyPass.SetText("")
	ui.sshFingerprint.SetText("")
	ui.sshFormOriginal = ui.currentSSHFormValues()
}

func (ui *Window) currentSSHFormValues() sshFormValues {
	return sshFormValues{
		HostID:      ui.sshHostID,
		Name:        ui.sshName.Text(),
		Host:        ui.sshHost.Text(),
		Port:        ui.sshPort.Text(),
		User:        ui.sshUser.Text(),
		Password:    ui.sshPassword.Text(),
		PrivateKey:  ui.sshPrivateKey.Text(),
		KeyPass:     ui.sshKeyPass.Text(),
		Fingerprint: ui.sshFingerprint.Text(),
	}
}

func (ui *Window) openSSHNewHostForm() {
	ui.clearSSHHostForm()
	ui.sshFormOpen = true
	ui.sshFormOriginal = ui.currentSSHFormValues()
}

func (ui *Window) openSSHEditHostForm(index int) {
	if index < 0 || index >= len(ui.sshHosts) {
		return
	}
	ui.sshHostIndex = index
	ui.sshHostID = ui.sshHosts[index].ID
	ui.loadSSHHostForm(ui.sshHosts[index])
	ui.sshFormOpen = true
}

func (ui *Window) closeSSHHostForm() {
	ui.sshFormOpen = false
	ui.sshFormCloseButton = widget.Clickable{}
	ui.sshFormCancelButton = widget.Clickable{}
	ui.sshFormScrim = widget.Clickable{}
}

func (ui *Window) requestCloseSSHHostForm() {
	if !ui.sshFormOpen || ui.busy {
		return
	}
	if sshFormCloseNeedsConfirmation(sshFormDirty(ui.sshFormOriginal, ui.currentSSHFormValues())) {
		ui.requestConfirm("Discard changes?", "This SSH host form has unsaved changes.", ui.closeSSHHostForm)
		return
	}
	ui.closeSSHHostForm()
}

// sshHostInputFromForm validates the form. A new host requires a password or
// private key; editing an existing host may leave both blank to keep the
// stored secret unchanged.
func (ui *Window) sshHostInputFromForm() (remote.SSHHostInput, error) {
	name := strings.TrimSpace(ui.sshName.Text())
	host := strings.TrimSpace(ui.sshHost.Text())
	user := strings.TrimSpace(ui.sshUser.Text())
	if name == "" {
		return remote.SSHHostInput{}, errors.New("name is required")
	}
	if host == "" {
		return remote.SSHHostInput{}, errors.New("host is required")
	}
	if user == "" {
		return remote.SSHHostInput{}, errors.New("username is required")
	}
	port := 22
	if value := strings.TrimSpace(ui.sshPort.Text()); value != "" {
		parsed, err := parsePort(value)
		if err != nil {
			return remote.SSHHostInput{}, errors.New("port must be a number between 1 and 65535")
		}
		port = parsed
	}
	input := remote.SSHHostInput{
		Name:               name,
		Host:               host,
		Port:               port,
		Username:           user,
		Password:           ui.sshPassword.Text(),
		PrivateKey:         strings.TrimSpace(ui.sshPrivateKey.Text()),
		KeyPassphrase:      ui.sshKeyPass.Text(),
		TrustedFingerprint: strings.TrimSpace(ui.sshFingerprint.Text()),
	}
	if ui.sshHostID == "" && input.Password == "" && input.PrivateKey == "" {
		return remote.SSHHostInput{}, errors.New("password or private key is required")
	}
	return input, nil
}

func (ui *Window) refreshSSHHosts() {
	ui.async("Loading SSH hosts...", func(ctx context.Context) (func(), error) {
		session := ui.model.RemoteSession
		if session == nil {
			return nil, errors.New("remote session is not active")
		}
		hosts, err := session.SSHHosts(ctx)
		if err != nil {
			return nil, err
		}
		return func() { ui.applySSHHosts(hosts) }, nil
	})
}

func (ui *Window) handleRemoteSSH(gtx layout.Context) {
	if ui.sshFormOpen {
		ui.handleSSHHostForm(gtx)
		return
	}
	if tab := ui.sshTabs.active(); tab != nil && tab.State == sshTabConnected {
		if tab.terminalViewButton.Clicked(gtx) {
			ui.sshTabs.setView(tab.ID, sshTabViewTerminal)
			return
		}
		if !tab.Local && tab.sftpViewButton.Clicked(gtx) {
			if tab.sftpBrowser == nil {
				ui.openSSHTabSFTP(tab.ID)
			} else {
				ui.sshTabs.setView(tab.ID, sshTabViewSFTP)
			}
			return
		}
		if tab.View == sshTabViewSFTP {
			if ui.handleSSHTabSFTP(gtx, tab) {
				return
			}
		} else {
			if ui.handleSSHTabClipboard(gtx, tab) {
				return
			}
			if ui.handleSSHTabKeys(gtx, tab) {
				return
			}
			if ui.drainEditors(gtx, &tab.input) {
				ui.sendSSHTabInput(tab.ID)
				return
			}
		}
	}
	ui.rebuildSSHHostFilterIfNeeded()
	for i := range ui.sshRecentButtons {
		if ui.sshRecentButtons[i].Clicked(gtx) {
			ui.openSSHHostTab(ui.sshRecentHostIndices[i])
			return
		}
	}
	for i := range ui.sshHostButtons {
		if ui.sshHostButtons[i].Clicked(gtx) {
			ui.openSSHHostTab(ui.sshHostIndices[i])
			return
		}
	}
	for i := range ui.sshHostEditButtons {
		if ui.sshHostEditButtons[i].Clicked(gtx) {
			ui.openSSHEditHostForm(ui.sshHostIndices[i])
			return
		}
	}
	if ui.sshNew.Clicked(gtx) {
		ui.openSSHNewHostForm()
		return
	}
	if ui.handleSSHTabDrag(gtx) {
		return
	}
	if ui.handleSSHTabActions(gtx) {
		return
	}
	for _, tab := range ui.sshTabs.tabs {
		if tab.tabButton.Clicked(gtx) {
			ui.sshTabs.activate(tab.ID)
			return
		}
		if tab.closeButton.Clicked(gtx) {
			ui.closeSSHTab(tab.ID)
			return
		}
		if tab.retryButton.Clicked(gtx) {
			ui.retrySSHTab(tab.ID)
			return
		}
	}
	if tab := ui.sshTabs.active(); tab != nil {
		if tab.sendButton.Clicked(gtx) {
			ui.sendSSHTabInput(tab.ID)
			return
		}
	}
}

func (ui *Window) handleSSHTabDrag(gtx layout.Context) bool {
	if ui.sshTabDrag.active && ui.sshTabs.get(ui.sshTabDrag.tabID) == nil {
		ui.sshTabDrag.reset()
	}
	kinds := pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel
	for index, tab := range ui.sshTabs.tabs {
		for {
			raw, ok := gtx.Event(pointer.Filter{Target: &tab.dragTag, Kinds: kinds})
			if !ok {
				break
			}
			pointerEvent, ok := raw.(pointer.Event)
			if !ok {
				continue
			}
			switch pointerEvent.Kind {
			case pointer.Press:
				if !pointerEvent.Buttons.Contain(pointer.ButtonPrimary) {
					continue
				}
				ui.sshTabDrag = sshTabDragState{
					active:     true,
					tabID:      tab.ID,
					startIndex: index,
					startX:     pointerEvent.Position.X,
					pointerID:  uint16(pointerEvent.PointerID),
				}
				gtx.Execute(pointer.GrabCmd{Tag: &tab.dragTag, ID: pointerEvent.PointerID})
				return true
			case pointer.Release:
				if !ui.sshTabDrag.active || ui.sshTabDrag.tabID != tab.ID || ui.sshTabDrag.pointerID != uint16(pointerEvent.PointerID) {
					continue
				}
				extent := float32(gtx.Dp(230))
				target := sshTabDragTarget(ui.sshTabDrag.startIndex, pointerEvent.Position.X-ui.sshTabDrag.startX, extent, len(ui.sshTabs.tabs))
				tabID := ui.sshTabDrag.tabID
				ui.sshTabDrag.reset()
				if target >= 0 {
					ui.sshTabs.move(tabID, target)
				}
				return true
			case pointer.Cancel:
				if ui.sshTabDrag.active && ui.sshTabDrag.tabID == tab.ID {
					ui.sshTabDrag.reset()
					return true
				}
			case pointer.Drag:
				if ui.sshTabDrag.active && ui.sshTabDrag.tabID == tab.ID && ui.sshTabDrag.pointerID == uint16(pointerEvent.PointerID) {
					return true
				}
			}
		}
	}
	return false
}

func (ui *Window) ensureSSHTabActionButtons() {
	want := len(sshTabActionSources())
	if len(ui.sshTabActionButtons) != want {
		ui.sshTabActionButtons = make([]widget.Clickable, want)
	}
}

func (ui *Window) handleSSHTabActions(gtx layout.Context) bool {
	tab := ui.sshTabs.active()
	if tab == nil {
		return false
	}
	ui.ensureSSHTabActionButtons()
	for index, source := range sshTabActionSources() {
		if !ui.sshTabActionButtons[index].Clicked(gtx) {
			continue
		}
		switch source {
		case "Duplicate":
			ui.duplicateSSHTab(tab.ID)
		case "Reconnect":
			ui.reconnectSSHTab(tab.ID)
		case "Rename":
			ui.openSSHTabRename(tab.ID)
		case "Pin":
			ui.sshTabs.setPinned(tab.ID, !tab.Pinned)
		case "Close others":
			ui.finishOtherSSHTabHistory(tab.ID)
			ui.sshTabs.closeOthers(tab.ID)
			ui.model.Status = ui.text("SSH terminal closed.")
		case "Close all":
			ui.finishAllSSHTabHistory()
			ui.sshTabs.closeAll()
			ui.model.Status = ui.text("SSH terminal closed.")
		}
		return true
	}
	return false
}

func (ui *Window) duplicateSSHTab(tabID string) {
	tab := ui.sshTabs.duplicate(tabID)
	if tab == nil {
		return
	}
	ui.model.Error = ""
	ui.model.Status = ui.text("Connecting")
	if tab.Local {
		ui.startLocalTerminalForTab(tab)
		return
	}
	ui.connectSSHTab(tab.ID, tab.HostID, tab.size)
}

func (ui *Window) reconnectSSHTab(tabID string) {
	tab := ui.sshTabs.get(tabID)
	if tab != nil {
		ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
	}
	if tab == nil || !ui.sshTabs.reconnect(tabID) {
		return
	}
	ui.model.Error = ""
	ui.model.Status = ui.text("Connecting")
	if tab.Local {
		ui.startLocalTerminalForTab(tab)
		return
	}
	ui.connectSSHTab(tab.ID, tab.HostID, tab.size)
}

func (ui *Window) openSSHTabRename(tabID string) {
	tab := ui.sshTabs.get(tabID)
	if tab == nil {
		return
	}
	ui.sshTabRenameID = tabID
	ui.sshTabRenameEditor.SetText(sshTabDisplayName(tab))
	ui.sshTabRenameOpen = true
}

func (ui *Window) closeSSHTabRename() {
	ui.sshTabRenameOpen = false
	ui.sshTabRenameID = ""
	ui.sshTabRenameEditor.SetText("")
	ui.sshTabRenameClose = widget.Clickable{}
	ui.sshTabRenameCancel = widget.Clickable{}
	ui.sshTabRenameSave = widget.Clickable{}
	ui.sshTabRenameScrim = widget.Clickable{}
}

func (ui *Window) handleSSHTabRename(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sshTabRenameEditor) {
		ui.saveSSHTabRename()
		return
	}
	if ui.sshTabRenameClose.Clicked(gtx) || ui.sshTabRenameCancel.Clicked(gtx) || ui.sshTabRenameScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHTabRename()
		return
	}
	if ui.sshTabRenameSave.Clicked(gtx) {
		ui.saveSSHTabRename()
	}
}

func (ui *Window) saveSSHTabRename() {
	if strings.TrimSpace(ui.sshTabRenameEditor.Text()) == "" {
		ui.model.Error = ui.text("Tab name is required")
		return
	}
	if ui.sshTabs.rename(ui.sshTabRenameID, ui.sshTabRenameEditor.Text()) {
		ui.model.Error = ""
		ui.closeSSHTabRename()
	}
}

func (ui *Window) handleSSHTabKeys(gtx layout.Context, tab *sshTab) bool {
	if tab == nil {
		return false
	}
	for _, filter := range terminalKeyFilters(&tab.input) {
		for {
			event, ok := gtx.Event(filter)
			if !ok {
				break
			}
			pressed, ok := event.(key.Event)
			if !ok || pressed.State != key.Press {
				continue
			}
			ui.sendSSHTabKey(tab.ID, pressed)
			return true
		}
	}
	return false
}

const terminalClipboardMaxBytes = 1 << 20

func (ui *Window) handleSSHTabClipboard(gtx layout.Context, tab *sshTab) bool {
	if tab == nil {
		return false
	}
	for _, mimeType := range []string{"text/plain", "text/plain;charset=utf-8"} {
		for {
			raw, ok := gtx.Event(transfer.TargetFilter{Target: &tab.clipboardTag, Type: mimeType})
			if !ok {
				break
			}
			dataEvent, ok := raw.(transfer.DataEvent)
			if !ok || dataEvent.Open == nil {
				continue
			}
			reader := dataEvent.Open()
			data, err := io.ReadAll(io.LimitReader(reader, terminalClipboardMaxBytes+1))
			closeErr := reader.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				ui.model.Error = err.Error()
				return true
			}
			if len(data) > terminalClipboardMaxBytes {
				ui.model.Error = "clipboard text exceeds 1 MiB"
				return true
			}
			ui.pasteSSHTabText(tab.ID, string(data))
			return true
		}
	}
	if tab.copyButton.Clicked(gtx) {
		ui.copySSHTabSelection(gtx, tab)
		return true
	}
	if tab.pasteButton.Clicked(gtx) {
		ui.requestSSHTabPaste(gtx, tab)
		return true
	}
	for _, action := range []struct {
		name key.Name
		copy bool
	}{
		{name: key.Name("C"), copy: true},
		{name: key.Name("V")},
	} {
		for {
			raw, ok := gtx.Event(key.Filter{
				Focus:    &tab.input,
				Name:     action.name,
				Required: key.ModCtrl | key.ModShift,
			})
			if !ok {
				break
			}
			pressed, ok := raw.(key.Event)
			if !ok || pressed.State != key.Press {
				continue
			}
			if action.copy {
				ui.copySSHTabSelection(gtx, tab)
			} else {
				ui.requestSSHTabPaste(gtx, tab)
			}
			return true
		}
	}
	return false
}

func (ui *Window) copySSHTabSelection(gtx layout.Context, tab *sshTab) bool {
	if tab == nil {
		return false
	}
	text := tab.selectedTerminalText()
	if text == "" {
		ui.model.Status = ui.text("No terminal text is selected.")
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "text/plain",
		Data: io.NopCloser(strings.NewReader(text)),
	})
	return true
}

func (ui *Window) requestSSHTabPaste(gtx layout.Context, tab *sshTab) bool {
	if tab == nil || tab.session == nil || tab.session.pty == nil {
		return false
	}
	gtx.Execute(clipboard.ReadCmd{Tag: &tab.clipboardTag})
	return true
}

func terminalKeyFilters(focus *widget.Editor) []key.Filter {
	filters := make([]key.Filter, 0, 22)
	for _, name := range []key.Name{
		key.NameLeftArrow,
		key.NameRightArrow,
		key.NameUpArrow,
		key.NameDownArrow,
		key.NameHome,
		key.NameEnd,
		key.NameReturn,
		key.NameEnter,
		key.NameTab,
		key.NameEscape,
		key.NameDeleteBackward,
		key.NameDeleteForward,
		key.NamePageUp,
		key.NamePageDown,
		key.NameF1,
		key.NameF2,
		key.NameF3,
		key.NameF4,
		key.NameF5,
		key.NameF6,
		key.NameF7,
		key.NameF8,
		key.NameF9,
		key.NameF10,
		key.NameF11,
		key.NameF12,
	} {
		filters = append(filters, key.Filter{
			Focus:    focus,
			Name:     name,
			Optional: key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper,
		})
	}
	for _, name := range []key.Name{"C", "D"} {
		filters = append(filters, key.Filter{Focus: focus, Name: name, Required: key.ModCtrl})
	}
	return filters
}

func (ui *Window) handleSSHHostForm(gtx layout.Context) {
	if ui.drainEditors(gtx,
		&ui.sshName, &ui.sshHost, &ui.sshPort, &ui.sshUser,
		&ui.sshPassword, &ui.sshPrivateKey, &ui.sshKeyPass, &ui.sshFingerprint,
	) {
		ui.saveSSHHost()
		return
	}
	if ui.sshFormCloseButton.Clicked(gtx) || ui.sshFormCancelButton.Clicked(gtx) || ui.sshFormScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.requestCloseSSHHostForm()
		return
	}
	if ui.sshSave.Clicked(gtx) {
		ui.saveSSHHost()
		return
	}
	if ui.sshDelete.Clicked(gtx) {
		ui.deleteSSHHost()
		return
	}
}

func (ui *Window) escapePressed(gtx layout.Context) bool {
	for {
		event, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return false
		}
		pressed, ok := event.(key.Event)
		if ok && pressed.State == key.Press {
			return true
		}
	}
}

func (ui *Window) saveSSHHost() {
	if ui.busy {
		return
	}
	input, err := ui.sshHostInputFromForm()
	if err != nil {
		ui.model.Error = err.Error()
		return
	}
	hostID := ui.sshHostID
	ui.async("Saving SSH host...", func(ctx context.Context) (func(), error) {
		session := ui.model.RemoteSession
		if session == nil {
			return nil, errors.New("remote session is not active")
		}
		if hostID == "" {
			host, err := session.CreateSSHHost(ctx, input)
			if err != nil {
				return nil, err
			}
			return func() {
				ui.sshHostID = host.ID
				ui.closeSSHHostForm()
				ui.refreshSSHHosts()
			}, nil
		}
		if _, err := session.UpdateSSHHost(ctx, hostID, input); err != nil {
			return nil, err
		}
		return func() {
			ui.closeSSHHostForm()
			ui.refreshSSHHosts()
		}, nil
	})
}

func (ui *Window) deleteSSHHost() {
	if ui.busy || ui.sshHostID == "" {
		return
	}
	hostID := ui.sshHostID
	name := strings.TrimSpace(ui.sshName.Text())
	message := fmt.Sprintf("Delete %q? This cannot be undone.", name)
	ui.requestConfirm("Delete SSH host?", message, func() {
		ui.async("Deleting SSH host...", func(ctx context.Context) (func(), error) {
			session := ui.model.RemoteSession
			if session == nil {
				return nil, errors.New("remote session is not active")
			}
			if err := session.DeleteSSHHost(ctx, hostID); err != nil {
				return nil, err
			}
			return func() {
				ui.closeSSHHostForm()
				ui.clearSSHHostForm()
				ui.refreshSSHHosts()
			}, nil
		})
	})
}

func (ui *Window) connectSSHHost() {
	if ui.busy {
		return
	}
	hostID := ui.sshHostID
	if hostID == "" {
		ui.model.Error = "Select or save an SSH host first"
		return
	}
	for index := range ui.sshHosts {
		if ui.sshHosts[index].ID == hostID {
			ui.openSSHHostTab(index)
			return
		}
	}
	ui.model.Error = "Select or save an SSH host first"
}

func (ui *Window) openSSHHostTab(index int) {
	if index < 0 || index >= len(ui.sshHosts) {
		return
	}
	host := ui.sshHosts[index]
	if !host.Enabled {
		ui.model.Error = ui.text("Host is disabled.")
		return
	}
	ui.sshHostIndex = index
	ui.sshHostID = host.ID
	tab := ui.sshTabs.open(host)
	tab.size = ui.terminalSize
	ui.model.Status = ui.text("Connecting")
	ui.connectSSHTab(tab.ID, host.ID, tab.size)
}

func (ui *Window) connectSSHTab(tabID, hostID string, size image.Point) {
	if tab := ui.sshTabs.get(tabID); tab != nil {
		ui.startSSHSessionHistory(tab)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		session := ui.model.RemoteSession
		if session == nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, errors.New("remote session is not active")) })
			return
		}
		creds, err := session.SSHHostCredentials(ctx, hostID)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		client, term, err := ui.openPooledSSHTerminal(ctx, creds, size)
		if err == nil {
			ui.queueSSHTabApply(func() { ui.attachSSHTab(tabID, client, term) }, func() {
				_ = term.Close()
				_ = client.Close()
			})
			return
		}
		fingerprint, ok := parsePendingFingerprint(err)
		if !ok {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		ui.queueSSHTabApply(func() { ui.beginFingerprintConfirmTab(tabID, hostID, fingerprint, size) })
	}()
}

func (ui *Window) connectSSHTabWithFingerprint(tabID, hostID, fingerprint string, size image.Point) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		session := ui.model.RemoteSession
		if session == nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, errors.New("remote session is not active")) })
			return
		}
		if err := session.SetSSHHostFingerprint(ctx, hostID, fingerprint); err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		creds, err := session.SSHHostCredentials(ctx, hostID)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		client, term, err := ui.openPooledSSHTerminal(ctx, creds, size)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		ui.queueSSHTabApply(func() { ui.attachSSHTab(tabID, client, term) }, func() {
			_ = term.Close()
			_ = client.Close()
		})
	}()
}

func (ui *Window) queueSSHTabApply(apply func(), cleanup ...func()) bool {
	release := func() {
		for _, cleanup := range cleanup {
			cleanup()
		}
	}
	if ui == nil {
		release()
		return false
	}
	id, ok := ui.registerSSHTabApply(release)
	if !ok {
		release()
		return false
	}
	event := asyncEvent{apply: func() {
		if ui.claimSSHTabApply(id) && apply != nil {
			apply()
		}
	}}
	select {
	case <-ui.closed:
		ui.discardSSHTabApply(id)
		return false
	case ui.events <- event:
		if ui.window != nil {
			ui.window.Invalidate()
		}
		return true
	}
}

func (ui *Window) failSSHTab(tabID string, err error) {
	if ui.sshTabs.fail(tabID, err) {
		if tab := ui.sshTabs.get(tabID); tab != nil {
			message := ""
			if err != nil {
				message = err.Error()
			}
			ui.finishSSHSessionHistory(tab, remote.SSHSessionFailed, message)
		}
		ui.model.Status = ui.text("Connection failed")
	}
}

func (ui *Window) closeSSHTab(tabID string) {
	if ui.sshTabDrag.active && ui.sshTabDrag.tabID == tabID {
		ui.sshTabDrag.reset()
	}
	tab := ui.sshTabs.get(tabID)
	if tab != nil {
		ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
	}
	if ui.sshTabs.close(tabID) != nil {
		ui.model.Status = ui.text("SSH terminal closed.")
	}
}

func (ui *Window) retrySSHTab(tabID string) {
	tab := ui.sshTabs.get(tabID)
	if tab != nil {
		ui.finishSSHSessionHistory(tab, remote.SSHSessionClosed, "")
	}
	if tab == nil || !ui.sshTabs.retry(tabID) {
		return
	}
	ui.model.Error = ""
	ui.model.Status = ui.text("Connecting")
	if tab.Local {
		ui.startLocalTerminalForTab(tab)
		return
	}
	ui.connectSSHTab(tab.ID, tab.HostID, tab.size)
}

func (ui *Window) beginFingerprintConfirmTab(tabID, hostID, fingerprint string, size image.Point) {
	if ui.sshTabs.get(tabID) == nil {
		return
	}
	message := fmt.Sprintf("Host key fingerprint:\n%s\nTrust this key and connect?", fingerprint)
	ui.requestConfirm("Trust this host key?", message, func() {
		if ui.sshTabs.get(tabID) == nil {
			return
		}
		ui.connectSSHTabWithFingerprint(tabID, hostID, fingerprint, size)
	})
}

func (ui *Window) attachSSHTab(tabID string, client interface{ Close() error }, term ptyTerminal) {
	tab := ui.sshTabs.get(tabID)
	if tab == nil {
		if term != nil {
			_ = term.Close()
		}
		if client != nil {
			_ = client.Close()
		}
		return
	}
	if tab.session != nil {
		tab.session.close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	tab.session = &sshTabSession{pty: term, client: client, ctx: ctx, cancel: cancel}
	tab.State = sshTabConnected
	tab.Error = ""
	ui.sshTabs.activate(tabID)
	ui.finishSSHSessionHistory(tab, remote.SSHSessionConnected, "")
	ui.model.Status = ui.text("Connected")
	ui.readSSHTab(tabID, tab.session)
}

func (ui *Window) openPooledSSHTerminal(ctx context.Context, credentials remote.SSHHostCredentials, size image.Point) (*sshConnectionLease, ptyTerminal, error) {
	if ui == nil {
		return nil, nil, errors.New("ssh connection pool: window is not available")
	}
	if ui.sshPool == nil {
		ui.sshPool = newSSHConnectionPool()
	}
	factory := ui.sshTransportFactory
	if factory == nil {
		factory = newSSHClientTransport
	}
	return openPooledSSHTerminal(ctx, ui.sshPool, credentials, size, factory)
}

func (ui *Window) readSSHTab(tabID string, session *sshTabSession) {
	go func() {
		if session == nil || session.pty == nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			n, err := session.pty.Read(buf)
			if n > 0 {
				if tab := ui.sshTabs.get(tabID); tab != nil {
					tab.appendOutput(string(buf[:n]))
					if ui.window != nil {
						ui.window.Invalidate()
					}
				}
			}
			if err != nil {
				ui.queueSSHTabApply(func() {
					if ui.sshTabs.endSession(tabID, session, err) {
						status := remote.SSHSessionFailed
						message := err.Error()
						if errors.Is(err, io.EOF) {
							status = remote.SSHSessionClosed
							message = ""
						}
						if tab := ui.sshTabs.get(tabID); tab != nil {
							ui.finishSSHSessionHistory(tab, status, message)
						}
						ui.model.Status = ui.text("Connection failed")
					}
				})
				return
			}
		}
	}()
}

func (ui *Window) sendSSHTabInput(tabID string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.session == nil || tab.session.pty == nil {
		return false
	}
	text := tab.input.Text()
	if text == "" {
		return true
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if _, err := tab.session.pty.Write([]byte(text)); err != nil {
		ui.failSSHTab(tabID, err)
		return false
	}
	tab.input.SetText("")
	return true
}

func (ui *Window) pasteSSHTabText(tabID, text string) bool {
	data, multiline := prepareTerminalPaste(text)
	if len(data) == 0 {
		return true
	}
	if !multiline {
		return ui.writeSSHTabBytes(tabID, data)
	}
	if ui.busy {
		return false
	}
	ui.requestConfirm(
		ui.text("Paste multiple lines?"),
		ui.text("Pasting multiple lines may execute several commands."),
		func() { ui.writeSSHTabBytes(tabID, data) },
	)
	return true
}

func (ui *Window) writeSSHTabBytes(tabID string, data []byte) bool {
	if len(data) == 0 {
		return true
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.session == nil || tab.session.pty == nil {
		return false
	}
	n, err := tab.session.pty.Write(data)
	if err != nil || n != len(data) {
		if err == nil {
			err = io.ErrShortWrite
		}
		ui.failSSHTab(tabID, err)
		return false
	}
	return true
}

func (ui *Window) sendSSHTabKey(tabID string, event key.Event) bool {
	data := encodeTerminalKey(event)
	if len(data) == 0 {
		return false
	}
	return ui.writeSSHTabBytes(tabID, data)
}

// resizeSSHTab keeps the remote PTY and the tab's terminal screen in sync.
// Local shell pipes do not expose a resize operation, so their emulator can
// still follow the available content size without failing the layout update.
func (ui *Window) resizeSSHTab(tabID string, size image.Point) bool {
	if size.X < 1 || size.Y < 1 {
		return false
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.State != sshTabConnected || tab.session == nil || tab.session.pty == nil {
		return false
	}
	frame := tab.emulator.Frame()
	if frame.Width == size.X && frame.Height == size.Y {
		tab.size = size
		return true
	}
	if resizer, ok := tab.session.pty.(interface{ Resize(int, int) error }); ok {
		if err := resizer.Resize(size.X, size.Y); err != nil {
			ui.sshTabs.fail(tabID, err)
			return false
		}
	}
	if tab.emulator != nil {
		if err := tab.emulator.Resize(size.X, size.Y); err != nil {
			ui.sshTabs.fail(tabID, err)
			return false
		}
	}
	tab.size = size
	return true
}

// dialSSHTerminal dials the host directly from this client using credentials
// issued by the remote service. The PTY context is Background because the
// terminal outlives the async worker that spawned it.
func dialSSHTerminal(creds remote.SSHHostCredentials, size image.Point) (*sshclient.Client, ptyTerminal, error) {
	client := sshclient.NewClient(sshProfileFromCredentials(creds))
	if err := client.Connect(); err != nil {
		client.Close()
		return nil, nil, err
	}
	width, height := 100, 30
	if size.X > 0 {
		width = size.X
	}
	if size.Y > 0 {
		height = size.Y
	}
	term, err := client.OpenPTY(context.Background(), width, height)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return client, term, nil
}

// beginFingerprintConfirm asks the user to trust the host key reported by the
// first connection attempt, persists it, then retries the connection.
func (ui *Window) beginFingerprintConfirm(hostID, fingerprint string, creds remote.SSHHostCredentials) {
	message := fmt.Sprintf("Host key fingerprint:\n%s\nTrust this key and connect?", fingerprint)
	ui.requestConfirm("Trust this host key?", message, func() {
		size := ui.terminalSize
		ui.async("Trusting host key...", func(ctx context.Context) (func(), error) {
			session := ui.model.RemoteSession
			if session == nil {
				return nil, errors.New("remote session is not active")
			}
			if err := session.SetSSHHostFingerprint(ctx, hostID, fingerprint); err != nil {
				return nil, err
			}
			creds.TrustedFingerprint = fingerprint
			client, term, err := dialSSHTerminal(creds, size)
			if err != nil {
				return nil, err
			}
			return func() { ui.attachSSHTerminal(client, term) }, nil
		})
	})
}

func (ui *Window) attachSSHTerminal(client *sshclient.Client, term ptyTerminal) {
	ui.closeSSH()
	ui.ssh = client
	ui.terminal = term
	ui.terminalCtx, ui.terminalCancel = context.WithCancel(context.Background())
	go ui.readTerminal(term)
	ui.model.Status = "SSH terminal connected."
}

func (ui *Window) remoteSSHView(gtx layout.Context) layout.Dimensions {
	if len(ui.sshTabs.tabs) == 0 {
		return ui.remoteSSHEmptyView(gtx)
	}
	return ui.remoteSSHTabsView(gtx)
}

func (ui *Window) remoteSSHEmptyView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(ui.theme, ui.text("SSH terminal workspace"))
					title.Color = colorText
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := material.Body2(ui.theme, ui.text("Select an SSH host to open a terminal tab."))
					message.Color = colorMuted
					return message.Layout(gtx)
				}),
			)
		})
	})
}

func (ui *Window) remoteSSHTabsView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteSSHTabBar(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteSSHTabActions(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteSSHTabViewSwitcher(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteSSHTabToolbar(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					tab := ui.sshTabs.active()
					if tab == nil {
						return layout.Dimensions{}
					}
					return ui.remoteSSHTabContent(gtx, tab)
				}),
			)
		})
	})
}

func (ui *Window) remoteSSHTabViewSwitcher(gtx layout.Context) layout.Dimensions {
	tab := ui.sshTabs.active()
	if tab == nil || tab.State != sshTabConnected {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &tab.terminalViewButton, "Terminal", tab.View == sshTabViewTerminal)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.Local {
				return layout.Dimensions{}
			}
			return ui.button(gtx, &tab.sftpViewButton, "SFTP", tab.View == sshTabViewSFTP)
		}),
	)
}

func (ui *Window) remoteSSHTabActions(gtx layout.Context) layout.Dimensions {
	tab := ui.sshTabs.active()
	if tab == nil {
		return layout.Dimensions{}
	}
	ui.ensureSSHTabActionButtons()
	ui.sshTabActionList.Axis = layout.Horizontal
	return ui.sshTabActionList.Layout(gtx, len(sshTabActionSources()), func(gtx layout.Context, index int) layout.Dimensions {
		sources := sshTabActionSources()
		if index < 0 || index >= len(sources) {
			return layout.Dimensions{}
		}
		source := sources[index]
		if source == "Pin" && tab.Pinned {
			source = "Unpin"
		}
		gtx.Constraints.Min.X = gtx.Dp(108)
		gtx.Constraints.Max.X = gtx.Dp(144)
		return layout.Inset{Right: unit.Dp(fieldGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshTabActionButtons[index], source, false)
		})
	})
}

func (ui *Window) remoteSSHTabBar(gtx layout.Context) layout.Dimensions {
	ui.terminalTabList.Axis = layout.Horizontal
	return ui.terminalTabList.Layout(gtx, len(ui.sshTabs.tabs), func(gtx layout.Context, index int) layout.Dimensions {
		tab := ui.sshTabs.tabs[index]
		gtx.Constraints.Min.X = gtx.Dp(168)
		gtx.Constraints.Max.X = gtx.Dp(230)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.sshTabDragHandle(gtx, tab)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &tab.tabButton, ui.sshTabTitle(index), tab.ID == ui.sshTabs.activeID)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.actionButton(gtx, &tab.closeButton, "Close", false, true)
			}),
		)
	})
}

func (ui *Window) sshTabDragHandle(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil {
		return layout.Dimensions{}
	}
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(24)
			label := material.Body2(ui.theme, "::")
			label.Color = colorMuted
			return layout.Inset{Top: unit.Dp(fieldGap), Bottom: unit.Dp(fieldGap), Left: unit.Dp(fieldGap), Right: unit.Dp(fieldGap)}.Layout(gtx, label.Layout)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			area := clip.Rect{Max: gtx.Constraints.Min}.Push(gtx.Ops)
			event.Op(gtx.Ops, &tab.dragTag)
			if ui.sshTabDrag.active && ui.sshTabDrag.tabID == tab.ID {
				pointer.CursorGrabbing.Add(gtx.Ops)
			} else {
				pointer.CursorGrab.Add(gtx.Ops)
			}
			area.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
	)
}

func (ui *Window) sshTabTitle(index int) string {
	tab := ui.sshTabs.tabs[index]
	count := 0
	ordinal := 0
	for _, candidate := range ui.sshTabs.tabs {
		if candidate.HostID != tab.HostID {
			continue
		}
		count++
		if candidate.ID == tab.ID {
			ordinal = count
		}
	}
	if count > 1 {
		return fmt.Sprintf("%s #%d", sshTabDisplayName(tab), ordinal)
	}
	return sshTabDisplayName(tab)
}

func (ui *Window) remoteSSHTabToolbar(gtx layout.Context) layout.Dimensions {
	tab := ui.sshTabs.active()
	if tab == nil {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Subtitle1(ui.theme, sshTabDisplayName(tab))
					label.Color = colorText
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, tab.Endpoint)
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.sshTabStatus(gtx, tab) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.State != sshTabConnected || tab.View != sshTabViewTerminal {
				return layout.Dimensions{}
			}
			return ui.button(gtx, &tab.copyButton, "Copy", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.State != sshTabConnected || tab.View != sshTabViewTerminal {
				return layout.Dimensions{}
			}
			return ui.button(gtx, &tab.pasteButton, "Paste", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.State != sshTabError {
				return layout.Dimensions{}
			}
			return ui.button(gtx, &tab.retryButton, "Retry", true)
		}),
	)
}

func (ui *Window) sshTabStatus(gtx layout.Context, tab *sshTab) layout.Dimensions {
	text := sshTabStatusSource(tab.State)
	statusColor := colorMuted
	switch tab.State {
	case sshTabConnected:
		statusColor = colorTeal
	case sshTabError:
		statusColor = colorDanger
	}
	label := material.Body2(ui.theme, ui.text(text))
	label.Color = statusColor
	return label.Layout(gtx)
}

func (ui *Window) remoteSSHTabContent(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab.State == sshTabError {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := material.Body1(ui.theme, tab.Error)
					message.Color = colorDanger
					return message.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body2(ui.theme, ui.text("Use Retry to try this host again, or Close to remove this tab.")).Layout(gtx)
				}),
			)
		})
	}
	if tab.State != sshTabConnected || tab.session == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(ui.theme, ui.text("Connecting to SSH host..."))
			label.Color = colorMuted
			return label.Layout(gtx)
		})
	}
	if tab.View == sshTabViewSFTP {
		return ui.remoteSSHTabSFTPContent(gtx, tab)
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if tab.emulator != nil {
				return ui.terminalFrameView(gtx, tab)
			}
			return ui.outputList(gtx, &tab.outputList, tab.outputSnapshot(), "Terminal output will appear here", true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.editorField(gtx, &tab.input, "Terminal input")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &tab.sendButton, "Send", false)
				}),
			)
		}),
	)
}

func (ui *Window) remoteSSHTabSFTPContent(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil || tab.sftpBrowser == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(ui.theme, ui.text("Opening SFTP..."))
			label.Color = colorMuted
			return label.Layout(gtx)
		})
	}
	tab.syncSFTPEntryWidgets()
	browser := tab.sftpBrowser
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(ui.theme, ui.text("Remote path")+": "+browser.Path)
					label.Color = colorText
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &tab.sftpParentButton, "Parent folder", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &tab.sftpRefreshButton, "Refresh files", false)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.remoteSSHTabSFTPActions(gtx, tab)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.sftpLoading {
				label := material.Caption(ui.theme, ui.text("Loading remote files..."))
				label.Color = colorMuted
				return label.Layout(gtx)
			}
			if tab.sftpError != "" {
				label := material.Caption(ui.theme, tab.sftpError)
				label.Color = colorDanger
				return label.Layout(gtx)
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.sftpInfoPanel(gtx, tab)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.sftpEntriesDropArea(gtx, tab)
		}),
	)
}

func (ui *Window) sftpEntriesDropArea(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil || tab.sftpBrowser == nil {
		return layout.Dimensions{}
	}
	tab.sftpList.Axis = layout.Vertical
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if len(tab.sftpBrowser.Entries) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(ui.theme, ui.text("No files in this folder."))
					label.Color = colorMuted
					return label.Layout(gtx)
				})
			}
			return tab.sftpList.Layout(gtx, len(tab.sftpBrowser.Entries), func(gtx layout.Context, index int) layout.Dimensions {
				if index < 0 || index >= len(tab.sftpBrowser.Entries) {
					return layout.Dimensions{}
				}
				return ui.sftpEntryRow(gtx, tab, index)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			area := clip.Rect{Max: size}.Push(gtx.Ops)
			event.Op(gtx.Ops, &tab.sftpDropTag)
			area.Pop()
			return layout.Dimensions{Size: size}
		}),
	)
}

func (ui *Window) sftpInfoPanel(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil {
		return layout.Dimensions{}
	}
	lines := sftpInfoLines(tab.sftpInfo)
	if len(lines) == 0 {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: unit.Dp(fieldGap), Bottom: unit.Dp(fieldGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(ui.theme, ui.text("File information"))
				label.Color = colorText
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, infoLineChildren(ui.theme, lines[1:])...)
			}),
		)
	})
}

func infoLineChildren(theme *material.Theme, lines []string) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(lines))
	for _, line := range lines {
		line := line
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(theme, line)
			label.Color = colorMuted
			return label.Layout(gtx)
		}))
	}
	return children
}

func (ui *Window) remoteSSHTabSFTPActions(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil {
		return layout.Dimensions{}
	}
	ui.ensureSFTPActionButtons(tab)
	actions := sftpActionSources()
	tab.sftpActionList.Axis = layout.Horizontal
	return tab.sftpActionList.Layout(gtx, len(actions), func(gtx layout.Context, index int) layout.Dimensions {
		if index < 0 || index >= len(actions) || index >= len(tab.sftpActionButtons) {
			return layout.Dimensions{}
		}
		gtx.Constraints.Min.X = gtx.Dp(118)
		gtx.Constraints.Max.X = gtx.Dp(176)
		return layout.Inset{Right: unit.Dp(fieldGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &tab.sftpActionButtons[index], actions[index], false)
		})
	})
}

func (ui *Window) sftpEntryRow(gtx layout.Context, tab *sshTab, index int) layout.Dimensions {
	if tab == nil || tab.sftpBrowser == nil || index < 0 || index >= len(tab.sftpBrowser.Entries) || index >= len(tab.sftpSelectionWidgets) {
		return layout.Dimensions{}
	}
	entry := tab.sftpBrowser.Entries[index]
	return layout.Inset{Top: unit.Dp(fieldGap), Bottom: unit.Dp(fieldGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(ui.theme, &tab.sftpSelectionWidgets[index], "").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				name := entry.Name
				if entry.Directory {
					name += "/"
				}
				if entry.Directory {
					return ui.button(gtx, &tab.sftpOpenButtons[index], name, false)
				}
				label := material.Body2(ui.theme, name)
				label.Color = colorText
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				metadata := sftpEntryMetadata(entry)
				label := material.Caption(ui.theme, metadata)
				label.Color = colorMuted
				return label.Layout(gtx)
			}),
		)
	})
}

func sftpEntryMetadata(entry sftpEntry) string {
	if entry.Directory {
		return "directory"
	}
	if entry.Size < 0 {
		return ""
	}
	return fmt.Sprintf("%d B", entry.Size)
}

func (ui *Window) terminalFrameView(gtx layout.Context, tab *sshTab) layout.Dimensions {
	if tab == nil || tab.emulator == nil {
		return layout.Dimensions{}
	}
	cellSize := image.Point{
		X: gtx.Dp(unit.Dp(8)),
		Y: gtx.Dp(unit.Dp(20)),
	}
	if viewport := gtx.Constraints.Max; viewport.X > 0 && viewport.Y > 0 {
		ui.resizeSSHTab(tab.ID, terminalGridSize(viewport, cellSize))
	}
	frame := tab.emulator.Frame()
	appearance := normalizeTerminalAppearance(ui.terminalAppearance)
	if !tab.Local {
		for _, host := range ui.sshHosts {
			if host.ID == tab.HostID {
				appearance = terminalAppearanceForHost(appearance, host)
				break
			}
		}
	}
	ui.handleTerminalSelection(gtx, tab, frame, cellSize)
	tab.outputList.Axis = layout.Vertical
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return tab.outputList.Layout(gtx, len(frame.Cells), func(gtx layout.Context, index int) layout.Dimensions {
				if index < 0 || index >= len(frame.Cells) {
					return layout.Dimensions{}
				}
				return ui.terminalFrameRow(gtx, frame.Cells[index], index, tab.selection, appearance)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			area := clip.Rect{Max: size}.Push(gtx.Ops)
			event.Op(gtx.Ops, &tab.terminalTag)
			event.Op(gtx.Ops, &tab.clipboardTag)
			pointer.CursorText.Add(gtx.Ops)
			area.Pop()
			return layout.Dimensions{Size: size}
		}),
	)
}

func (ui *Window) handleTerminalSelection(gtx layout.Context, tab *sshTab, frame terminalFrame, cellSize image.Point) {
	if tab == nil || frame.Width < 1 || frame.Height < 1 {
		return
	}
	kinds := pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: &tab.terminalTag, Kinds: kinds})
		if !ok {
			return
		}
		event, ok := raw.(pointer.Event)
		if !ok {
			continue
		}
		cell := terminalCellAt(
			image.Pt(int(event.Position.X), int(event.Position.Y)),
			cellSize,
			image.Pt(frame.Width, frame.Height),
		)
		switch event.Kind {
		case pointer.Press:
			if event.Buttons.Contain(pointer.ButtonSecondary) {
				ui.requestSSHTabPaste(gtx, tab)
				continue
			}
			if !event.Buttons.Contain(pointer.ButtonPrimary) {
				continue
			}
			tab.clearTerminalSelection()
			tab.selectionAnchor = cell
			tab.selectionPointerID = uint16(event.PointerID)
			tab.selecting = true
			gtx.Execute(pointer.GrabCmd{Tag: &tab.terminalTag, ID: event.PointerID})
		case pointer.Drag:
			if tab.selecting && tab.selectionPointerID == uint16(event.PointerID) {
				tab.selection = terminalDragSelection(tab.selectionAnchor, cell)
			}
		case pointer.Release:
			if !tab.selecting || tab.selectionPointerID != uint16(event.PointerID) {
				continue
			}
			tab.selection = terminalDragSelection(tab.selectionAnchor, cell)
			tab.selecting = false
			if tab.selection.active {
				ui.copySSHTabSelection(gtx, tab)
			}
		case pointer.Cancel:
			tab.selecting = false
		}
	}
}

func (ui *Window) terminalFrameRow(gtx layout.Context, cells []terminalCell, row int, selection terminalSelection, appearance terminalAppearance) layout.Dimensions {
	if len(cells) == 0 {
		return layout.Dimensions{}
	}
	type cellRun struct {
		style    terminalCell
		text     string
		selected bool
	}
	runs := make([]cellRun, 0, len(cells))
	for column, cell := range cells {
		selected := terminalCellSelected(selection, image.Pt(column, row))
		if len(runs) > 0 && runs[len(runs)-1].selected == selected && sameTerminalCellStyle(runs[len(runs)-1].style, cell) {
			runs[len(runs)-1].text += cell.Text
			continue
		}
		runs = append(runs, cellRun{style: cell, text: cell.Text, selected: selected})
	}
	children := make([]layout.FlexChild, 0, len(runs))
	for _, run := range runs {
		run := run
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			foreground, background := terminalCellColors(run.style, appearance)
			if run.selected {
				foreground = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
				background = color.NRGBA{R: 0x16, G: 0x62, B: 0x72, A: 0xff}
			}
			label := material.Label(ui.theme, unit.Sp(appearance.FontSize), run.text)
			label.Font.Typeface = terminalTypeface(appearance.Font)
			label.Color = foreground
			if run.style.Bold {
				label.Font.Weight = font.Bold
			}
			if run.style.Italics {
				label.Font.Style = font.Italic
			}
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, background, clip.Rect{Max: gtx.Constraints.Min}.Op())
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(label.Layout),
			)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func sameTerminalCellStyle(left, right terminalCell) bool {
	return left.Foreground == right.Foreground &&
		left.Background == right.Background &&
		left.Bold == right.Bold &&
		left.Italics == right.Italics &&
		left.Underline == right.Underline &&
		left.Reverse == right.Reverse
}

func terminalCellForeground(cell terminalCell) color.NRGBA {
	if cell.Reverse {
		return colorBackground
	}
	switch cell.Foreground {
	case terminalColorRed:
		return colorDanger
	case terminalColorDefault:
		return colorText
	default:
		return colorText
	}
}

func (ui *Window) remoteSSHSidebar(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(230)
		gtx.Constraints.Max.X = gtx.Dp(270)
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							label := material.Subtitle1(ui.theme, ui.text("SSH hosts"))
							label.Color = colorText
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshNew, "New host", false)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(ui.sshHosts) == 0 {
						return material.Body2(ui.theme, ui.text("No SSH hosts yet.")).Layout(gtx)
					}
					return ui.remoteList.Layout(gtx, len(ui.sshHostIndices), func(gtx layout.Context, index int) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return ui.sshHostRow(gtx, index)
					})
				}),
			)
		})
	})
}

func (ui *Window) sshHostRow(gtx layout.Context, visibleIndex int) layout.Dimensions {
	if visibleIndex < 0 || visibleIndex >= len(ui.sshHostIndices) || visibleIndex >= len(ui.sshHostButtons) || visibleIndex >= len(ui.sshHostEditButtons) {
		return layout.Dimensions{}
	}
	index := ui.sshHostIndices[visibleIndex]
	if index < 0 || index >= len(ui.sshHosts) {
		return layout.Dimensions{}
	}
	host := ui.sshHosts[index]
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshHostButtons[visibleIndex], host.Name, ui.sshHostIndex == index)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshHostEditButtons[visibleIndex], "Edit", false)
		}),
	)
}

func (ui *Window) remoteSSHHostStrip(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(rowGap), Bottom: unit.Dp(rowGap), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							label := material.Subtitle1(ui.theme, ui.text("SSH hosts"))
							label.Color = colorText
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshNew, "New host", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(ui.sshHosts) == 0 {
						return material.Body2(ui.theme, ui.text("No SSH hosts yet.")).Layout(gtx)
					}
					ui.sshHostStripList.Axis = layout.Horizontal
					return ui.sshHostStripList.Layout(gtx, len(ui.sshHostIndices), func(gtx layout.Context, index int) layout.Dimensions {
						itemWidth := gtx.Dp(220)
						if itemWidth > gtx.Constraints.Max.X {
							itemWidth = gtx.Constraints.Max.X
						}
						gtx.Constraints.Min.X = itemWidth
						gtx.Constraints.Max.X = itemWidth
						return ui.sshHostRow(gtx, index)
					})
				}),
			)
		})
	})
}

func (ui *Window) remoteSSHFormView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshHostFormFields(gtx)
		})
	})
}

func (ui *Window) sshHostFormFields(gtx layout.Context) layout.Dimensions {
	return ui.sshFormList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.editorRow(gtx, "Name", &ui.sshName, "Host", &ui.sshHost)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.editorRow(gtx, "Port", &ui.sshPort, "Username", &ui.sshUser)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.labeledField(gtx, &ui.sshPassword, "Password", true, true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(privateKeyMinHeight)
				return ui.labeledField(gtx, &ui.sshPrivateKey, "Private key", false, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.labeledField(gtx, &ui.sshKeyPass, "Key passphrase", true, true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.labeledField(gtx, &ui.sshFingerprint, "Host fingerprint", true, false)
			}),
		)
	})
}

func (ui *Window) remoteSSHTerminalView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.outputList(gtx, &ui.terminalList, ui.terminalSnapshot(), "Terminal output will appear here", true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.editorField(gtx, &ui.terminalInput, "Terminal input")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSend, "Send", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.sshClose, "Close terminal", false, true)
						}),
					)
				}),
			)
		})
	})
}
