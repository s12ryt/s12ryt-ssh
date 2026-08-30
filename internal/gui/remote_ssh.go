package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
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
	ui.sshHosts = append([]remote.SSHHost(nil), hosts...)
	ui.sshHostButtons = make([]widget.Clickable, len(ui.sshHosts))
	ui.sshHostEditButtons = make([]widget.Clickable, len(ui.sshHosts))
	ui.sshHostIndex = -1
	for i := range ui.sshHosts {
		if ui.sshHosts[i].ID == ui.sshHostID {
			ui.sshHostIndex = i
			break
		}
	}
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
		if ui.drainEditors(gtx, &tab.input) {
			ui.sendSSHTabInput(tab.ID)
			return
		}
	}
	for i := range ui.sshHostButtons {
		if ui.sshHostButtons[i].Clicked(gtx) {
			ui.openSSHHostTab(i)
			return
		}
	}
	for i := range ui.sshHostEditButtons {
		if ui.sshHostEditButtons[i].Clicked(gtx) {
			ui.openSSHEditHostForm(i)
			return
		}
	}
	if ui.sshNew.Clicked(gtx) {
		ui.openSSHNewHostForm()
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
	ui.sshHostIndex = index
	ui.sshHostID = host.ID
	tab := ui.sshTabs.open(host)
	tab.size = ui.terminalSize
	ui.model.Status = ui.text("Connecting")
	ui.connectSSHTab(tab.ID, host.ID, tab.size)
}

func (ui *Window) connectSSHTab(tabID, hostID string, size image.Point) {
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
		client, term, err := dialSSHTerminal(creds, size)
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
		ui.queueSSHTabApply(func() { ui.beginFingerprintConfirmTab(tabID, hostID, fingerprint, creds, size) })
	}()
}

func (ui *Window) connectSSHTabWithFingerprint(tabID, hostID, fingerprint string, creds remote.SSHHostCredentials, size image.Point) {
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
		creds.TrustedFingerprint = fingerprint
		client, term, err := dialSSHTerminal(creds, size)
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
		ui.model.Status = ui.text("Connection failed")
	}
}

func (ui *Window) closeSSHTab(tabID string) {
	if ui.sshTabs.close(tabID) != nil {
		ui.model.Status = ui.text("SSH terminal closed.")
	}
}

func (ui *Window) retrySSHTab(tabID string) {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || !ui.sshTabs.retry(tabID) {
		return
	}
	ui.model.Error = ""
	ui.model.Status = ui.text("Connecting")
	ui.connectSSHTab(tab.ID, tab.HostID, tab.size)
}

func (ui *Window) beginFingerprintConfirmTab(tabID, hostID, fingerprint string, creds remote.SSHHostCredentials, size image.Point) {
	if ui.sshTabs.get(tabID) == nil {
		return
	}
	message := fmt.Sprintf("Host key fingerprint:\n%s\nTrust this key and connect?", fingerprint)
	ui.requestConfirm("Trust this host key?", message, func() {
		if ui.sshTabs.get(tabID) == nil {
			return
		}
		ui.connectSSHTabWithFingerprint(tabID, hostID, fingerprint, creds, size)
	})
}

func (ui *Window) attachSSHTab(tabID string, client *sshclient.Client, term ptyTerminal) {
	tab := ui.sshTabs.get(tabID)
	if tab == nil {
		_ = term.Close()
		_ = client.Close()
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
	ui.model.Status = ui.text("Connected")
	ui.readSSHTab(tabID, tab.session)
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

func (ui *Window) remoteSSHTabBar(gtx layout.Context) layout.Dimensions {
	ui.terminalTabList.Axis = layout.Horizontal
	return ui.terminalTabList.Layout(gtx, len(ui.sshTabs.tabs), func(gtx layout.Context, index int) layout.Dimensions {
		tab := ui.sshTabs.tabs[index]
		gtx.Constraints.Min.X = gtx.Dp(168)
		gtx.Constraints.Max.X = gtx.Dp(230)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &tab.tabButton, ui.sshTabTitle(index), tab.ID == ui.sshTabs.activeID)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.actionButton(gtx, &tab.closeButton, "Close", false, true)
			}),
		)
	})
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
		return fmt.Sprintf("%s #%d", tab.HostName, ordinal)
	}
	return tab.HostName
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
					label := material.Subtitle1(ui.theme, tab.HostName)
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
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
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
					return ui.remoteList.Layout(gtx, len(ui.sshHosts), func(gtx layout.Context, index int) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return ui.sshHostRow(gtx, index)
					})
				}),
			)
		})
	})
}

func (ui *Window) sshHostRow(gtx layout.Context, index int) layout.Dimensions {
	if index < 0 || index >= len(ui.sshHosts) || index >= len(ui.sshHostButtons) || index >= len(ui.sshHostEditButtons) {
		return layout.Dimensions{}
	}
	host := ui.sshHosts[index]
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshHostButtons[index], host.Name, ui.sshHostIndex == index)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshHostEditButtons[index], "Edit", false)
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
					return ui.sshHostStripList.Layout(gtx, len(ui.sshHosts), func(gtx layout.Context, index int) layout.Dimensions {
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
