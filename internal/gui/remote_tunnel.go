package gui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// remoteTunnelSession is optional so existing remote sessions remain
// compatible while tunnel support is introduced incrementally.
type remoteTunnelSession interface {
	SSHTunnels(context.Context) ([]remote.SSHTunnelRule, error)
}

type remoteTunnelCRUDSession interface {
	remoteTunnelSession
	CreateSSHTunnel(context.Context, remote.SSHTunnelInput) (remote.SSHTunnelRule, error)
	UpdateSSHTunnel(context.Context, string, remote.SSHTunnelInput) (remote.SSHTunnelRule, error)
	DeleteSSHTunnel(context.Context, string) error
}

type remoteTunnelRuntimeSession interface {
	UpdateSSHTunnelRuntime(context.Context, string, remote.SSHTunnelRuntimeUpdate) (remote.SSHTunnelRule, error)
}

func (ui *Window) openSSHTunnelForm(id string) bool {
	if ui == nil || ui.sshTunnelFormOpen || ui.busy || ui.sshTunnels == nil {
		return false
	}
	values := sshTunnelFormValues{
		Type:       remote.SSHTunnelLocal,
		ListenHost: "127.0.0.1",
		Enabled:    true,
	}
	if id == "" {
		if len(ui.sshHosts) > 0 {
			values.HostID = ui.sshHosts[0].ID
		}
	} else {
		entry, ok := ui.sshTunnels.get(id)
		if !ok {
			return false
		}
		values = sshTunnelFormFromRule(entry.Rule)
	}
	ui.sshTunnelFormOpen = true
	ui.sshTunnelFormID = id
	ui.sshTunnelForm = values
	ui.setSSHTunnelFormEditors(values)
	ui.model.Error = ""
	return true
}

func (ui *Window) setSSHTunnelFormEditors(values sshTunnelFormValues) {
	ui.sshTunnelName.SetText(values.Name)
	ui.sshTunnelHost.SetText(values.HostID)
	ui.sshTunnelListenHost.SetText(values.ListenHost)
	ui.sshTunnelListenPort.SetText(strconv.Itoa(values.ListenPort))
	ui.sshTunnelTargetHost.SetText(values.TargetHost)
	ui.sshTunnelTargetPort.SetText(strconv.Itoa(values.TargetPort))
	ui.sshTunnelEnabled.Value = values.Enabled
	ui.sshTunnelAutoStart.Value = values.AutoStart
}

func (ui *Window) currentSSHTunnelForm() sshTunnelFormValues {
	values := ui.sshTunnelForm
	values.ID = ui.sshTunnelFormID
	values.Name = ui.sshTunnelName.Text()
	values.HostID = ui.sshTunnelHost.Text()
	values.ListenHost = ui.sshTunnelListenHost.Text()
	values.ListenPort = parseTunnelPort(ui.sshTunnelListenPort.Text())
	values.TargetHost = ui.sshTunnelTargetHost.Text()
	values.TargetPort = parseTunnelPort(ui.sshTunnelTargetPort.Text())
	values.Enabled = ui.sshTunnelEnabled.Value
	values.AutoStart = ui.sshTunnelAutoStart.Value
	return values
}

func parseTunnelPort(value string) int {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return port
}

func (ui *Window) closeSSHTunnelForm() {
	if ui == nil {
		return
	}
	ui.sshTunnelFormOpen = false
	ui.sshTunnelFormID = ""
	ui.sshTunnelForm = sshTunnelFormValues{}
	ui.sshTunnelName.SetText("")
	ui.sshTunnelHost.SetText("")
	ui.sshTunnelListenHost.SetText("")
	ui.sshTunnelListenPort.SetText("")
	ui.sshTunnelTargetHost.SetText("")
	ui.sshTunnelTargetPort.SetText("")
	ui.sshTunnelEnabled.Value = false
	ui.sshTunnelAutoStart.Value = false
	ui.sshTunnelFormClose = widget.Clickable{}
	ui.sshTunnelFormCancel = widget.Clickable{}
	ui.sshTunnelFormSave = widget.Clickable{}
	ui.sshTunnelFormDelete = widget.Clickable{}
	ui.sshTunnelFormScrim = widget.Clickable{}
}

func (ui *Window) submitSSHTunnelForm() bool {
	if ui == nil || !ui.sshTunnelFormOpen {
		return false
	}
	values := ui.currentSSHTunnelForm()
	input := values.input()
	if source := validateSSHTunnelInput(input); source != "" {
		ui.model.Error = ui.text(source)
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelCRUDSession)
	if !ok {
		ui.model.Error = ui.text("SSH tunnel service is unavailable")
		return false
	}
	id := ui.sshTunnelFormID
	ui.closeSSHTunnelForm()
	ui.async("Saving SSH tunnel...", func(ctx context.Context) (func(), error) {
		var (
			rule remote.SSHTunnelRule
			err  error
		)
		if id == "" {
			rule, err = session.CreateSSHTunnel(ctx, input)
		} else {
			rule, err = session.UpdateSSHTunnel(ctx, id, input)
		}
		if err != nil {
			return nil, err
		}
		return func() { ui.upsertSSHTunnelRule(rule) }, nil
	})
	return true
}

func (ui *Window) deleteSSHTunnel(id string) bool {
	if ui == nil || ui.sshTunnels == nil || ui.busy {
		return false
	}
	if _, ok := ui.sshTunnels.get(id); !ok {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelCRUDSession)
	if !ok {
		ui.model.Error = ui.text("SSH tunnel service is unavailable")
		return false
	}
	ui.requestConfirm(ui.text("Delete tunnel?"), ui.text("This tunnel will be permanently deleted."), func() {
		ui.async("Deleting SSH tunnel...", func(ctx context.Context) (func(), error) {
			if err := session.DeleteSSHTunnel(ctx, id); err != nil {
				return nil, err
			}
			return func() {
				ui.stopSSHTunnel(id)
				ui.sshTunnels.remove(id)
			}, nil
		})
	})
	return true
}

func sshTunnelTypeOptions() []remote.SSHTunnelType {
	return []remote.SSHTunnelType{
		remote.SSHTunnelLocal,
		remote.SSHTunnelRemote,
		remote.SSHTunnelDynamic,
	}
}

func (ui *Window) handleSSHTunnelForm(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sshTunnelName, &ui.sshTunnelHost, &ui.sshTunnelListenHost, &ui.sshTunnelListenPort, &ui.sshTunnelTargetHost, &ui.sshTunnelTargetPort) {
		ui.submitSSHTunnelForm()
		return
	}
	for index, tunnelType := range sshTunnelTypeOptions() {
		if ui.sshTunnelTypeButtons[index].Clicked(gtx) {
			ui.sshTunnelForm.Type = tunnelType
			ui.model.Error = ""
			return
		}
	}
	ui.sshTunnelEnabled.Update(gtx)
	ui.sshTunnelAutoStart.Update(gtx)
	if ui.sshTunnelFormClose.Clicked(gtx) || ui.sshTunnelFormCancel.Clicked(gtx) || ui.sshTunnelFormScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHTunnelForm()
		return
	}
	if ui.sshTunnelFormDelete.Clicked(gtx) {
		id := ui.sshTunnelFormID
		ui.closeSSHTunnelForm()
		ui.deleteSSHTunnel(id)
		return
	}
	if ui.sshTunnelFormSave.Clicked(gtx) {
		ui.submitSSHTunnelForm()
	}
}

func (ui *Window) upsertSSHTunnelRule(rule remote.SSHTunnelRule) {
	if ui == nil || ui.sshTunnels == nil {
		return
	}
	entries := ui.sshTunnels.snapshot()
	rules := make([]remote.SSHTunnelRule, 0, len(entries)+1)
	found := false
	for _, entry := range entries {
		if entry.Rule.ID == rule.ID {
			rules = append(rules, rule)
			found = true
			continue
		}
		rules = append(rules, entry.Rule)
	}
	if !found {
		rules = append(rules, rule)
	}
	ui.sshTunnels.replace(rules)
}

func (ui *Window) refreshSSHTunnels() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelSession)
	if !ok {
		ui.model.Error = "SSH tunnel service is unavailable"
		return false
	}
	if ui.sshTunnels == nil {
		ui.sshTunnels = newSSHTunnelStore()
	}
	if ui.busy {
		return false
	}
	ui.async("Loading SSH tunnels...", func(ctx context.Context) (func(), error) {
		rules, err := session.SSHTunnels(ctx)
		if err != nil {
			return nil, err
		}
		return func() {
			ui.sshTunnels.replace(rules)
			for _, rule := range rules {
				if rule.Enabled && rule.AutoStart {
					ui.startSSHTunnel(rule.ID)
				}
			}
		}, nil
	})
	return true
}

func (ui *Window) optionalTunnelSession() (remoteTunnelSession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote session is not active")
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelSession)
	if !ok {
		return nil, errors.New("SSH tunnel service is unavailable")
	}
	return session, nil
}

func sshTunnelForwardSpec(rule remote.SSHTunnelRule) (sshclient.ForwardSpec, error) {
	forwardType := sshclient.ForwardLocal
	switch rule.Type {
	case remote.SSHTunnelLocal:
		forwardType = sshclient.ForwardLocal
	case remote.SSHTunnelRemote:
		forwardType = sshclient.ForwardRemote
	case remote.SSHTunnelDynamic:
		forwardType = sshclient.ForwardDynamic
	default:
		return sshclient.ForwardSpec{}, fmt.Errorf("unsupported SSH tunnel type %q", rule.Type)
	}
	return sshclient.ForwardSpec{
		Type:       forwardType,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
	}, nil
}

func (ui *Window) startSSHTunnel(id string) bool {
	if ui == nil || ui.sshTunnels == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	entry, ok := ui.sshTunnels.get(id)
	if !ok || !entry.Rule.Enabled || !ui.sshHostEnabled(entry.Rule.HostID) || entry.Runtime != nil || entry.Starting {
		return false
	}
	spec, err := sshTunnelForwardSpec(entry.Rule)
	if err != nil {
		ui.sshTunnels.setError(id, err.Error())
		return false
	}
	factory := ui.sshTransportFactory
	if factory == nil {
		factory = newSSHClientTransport
	}
	pool := ui.sshPool
	if pool == nil {
		pool = newSSHConnectionPool()
		ui.sshPool = pool
	}
	session := ui.model.RemoteSession
	if !ui.sshTunnels.setStarting(id) {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		credentials, credentialsErr := session.SSHHostCredentials(ctx, entry.Rule.HostID)
		cancel()
		if credentialsErr != nil {
			ui.queueSSHTabApply(func() { ui.sshTunnels.setStartingError(id, credentialsErr.Error()) })
			return
		}
		runtime, runtimeErr := openPooledSSHForward(context.Background(), pool, credentials, spec, factory)
		if runtimeErr != nil {
			ui.queueSSHTabApply(func() { ui.sshTunnels.setStartingError(id, runtimeErr.Error()) })
			return
		}
		ui.queueSSHTabApply(
			func() {
				if !ui.sshHostEnabled(entry.Rule.HostID) {
					_ = runtime.Close()
					ui.sshTunnels.stop(id)
					return
				}
				if ui.sshTunnels.attachStartingRuntime(id, runtime) {
					ui.syncSSHTunnelRuntime(id, true)
				}
			},
			func() { _ = runtime.Close() },
		)
	}()
	return true
}

func (ui *Window) syncSSHTunnelRuntime(id string, force bool) bool {
	if ui == nil || ui.sshTunnels == nil || ui.model == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelRuntimeSession)
	if !ok {
		return false
	}
	now := time.Now()
	update, ok := ui.sshTunnels.prepareRuntimeSync(id, now, force, true)
	if !ok {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := session.UpdateSSHTunnelRuntime(ctx, id, update)
		cancel()
		ui.queueSSHTabApply(func() {
			ui.sshTunnels.completeRuntimeSync(id, update, now, err == nil)
		})
	}()
	return true
}

func (ui *Window) reportSSHTunnelRuntime(id string, update remote.SSHTunnelRuntimeUpdate) bool {
	if ui == nil || ui.model == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteTunnelRuntimeSession)
	if !ok {
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = session.UpdateSSHTunnelRuntime(ctx, id, update)
		cancel()
	}()
	return true
}

func (ui *Window) stopSSHTunnel(id string) bool {
	if ui == nil || ui.sshTunnels == nil {
		return false
	}
	update, ok := ui.sshTunnels.stopWithRuntimeUpdate(id)
	if !ok {
		return false
	}
	ui.reportSSHTunnelRuntime(id, update)
	return true
}

func (ui *Window) stopSSHHostTunnels(hostID string) int {
	if ui == nil || ui.sshTunnels == nil {
		return 0
	}
	stopped := 0
	for _, entry := range ui.sshTunnels.snapshot() {
		if entry.Rule.HostID == hostID && (entry.Runtime != nil || entry.Starting || entry.Rule.Running) && ui.stopSSHTunnel(entry.Rule.ID) {
			stopped++
		}
	}
	return stopped
}

func (ui *Window) stopAllSSHTunnels() int {
	if ui == nil || ui.sshTunnels == nil {
		return 0
	}
	stopped := 0
	for _, entry := range ui.sshTunnels.snapshot() {
		if (entry.Runtime != nil || entry.Starting || entry.Rule.Running) && ui.stopSSHTunnel(entry.Rule.ID) {
			stopped++
		}
	}
	return stopped
}

func (ui *Window) syncSSHTunnelActionButtons(entries []sshTunnelEntry) {
	if len(ui.sshTunnelActionIDs) == len(entries) {
		matches := true
		for index, entry := range entries {
			if ui.sshTunnelActionIDs[index] != entry.Rule.ID {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	ui.sshTunnelActionIDs = make([]string, len(entries))
	ui.sshTunnelActionButtons = make([]widget.Clickable, len(entries))
	ui.sshTunnelEditButtons = make([]widget.Clickable, len(entries))
	ui.sshTunnelDeleteButtons = make([]widget.Clickable, len(entries))
	for index, entry := range entries {
		ui.sshTunnelActionIDs[index] = entry.Rule.ID
	}
}

func (ui *Window) handleSSHTunnels(gtx layout.Context) bool {
	if ui == nil || ui.sshTunnels == nil {
		return false
	}
	entries := ui.sshTunnels.snapshot()
	ui.syncSSHTunnelActionButtons(entries)
	for index, entry := range entries {
		if index < len(ui.sshTunnelEditButtons) && ui.sshTunnelEditButtons[index].Clicked(gtx) {
			ui.openSSHTunnelForm(entry.Rule.ID)
			return true
		}
		if index < len(ui.sshTunnelDeleteButtons) && ui.sshTunnelDeleteButtons[index].Clicked(gtx) {
			ui.deleteSSHTunnel(entry.Rule.ID)
			return true
		}
		if index < len(ui.sshTunnelActionButtons) && ui.sshTunnelActionButtons[index].Clicked(gtx) {
			if entry.Runtime != nil {
				ui.stopSSHTunnel(entry.Rule.ID)
			} else {
				ui.startSSHTunnel(entry.Rule.ID)
			}
			return true
		}
	}
	return false
}

func (ui *Window) sshTunnelPage(gtx layout.Context) layout.Dimensions {
	if ui.sshTunnels == nil {
		return layout.Dimensions{}
	}
	entries := ui.sshTunnels.snapshot()
	for _, entry := range entries {
		if entry.Runtime != nil {
			ui.syncSSHTunnelRuntime(entry.Rule.ID, false)
		}
	}
	ui.syncSSHTunnelActionButtons(entries)
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if len(entries) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					message := material.Body1(ui.theme, ui.text("No tunnels configured."))
					message.Color = colorMuted
					return message.Layout(gtx)
				})
			}
			ui.sshTunnelList.Axis = layout.Vertical
			return ui.sshTunnelList.Layout(gtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
				return ui.sshTunnelRow(gtx, entries[index], index)
			})
		})
	})
}

func (ui *Window) sshTunnelRow(gtx layout.Context, entry sshTunnelEntry, index int) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Subtitle1(ui.theme, entry.Rule.Name)
						label.Color = colorText
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(ui.theme, ui.text(sshTunnelDirectionSource(entry.Rule.Type)))
						label.Color = colorMuted
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						statusSource := sshTunnelEntryStatusSource(entry)
						label := material.Caption(ui.theme, ui.text(statusSource))
						if statusSource == "Failed" {
							label.Color = colorDanger
						} else if statusSource == "Running" {
							label.Color = colorTeal
						} else {
							label.Color = colorMuted
						}
						return label.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if index >= len(ui.sshTunnelActionButtons) {
							return layout.Dimensions{}
						}
						return ui.actionButton(gtx, &ui.sshTunnelActionButtons[index], sshTunnelActionSource(entry), false, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if index >= len(ui.sshTunnelEditButtons) {
							return layout.Dimensions{}
						}
						return ui.button(gtx, &ui.sshTunnelEditButtons[index], "Edit tunnel", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if index >= len(ui.sshTunnelDeleteButtons) {
							return layout.Dimensions{}
						}
						return ui.actionButton(gtx, &ui.sshTunnelDeleteButtons[index], "Delete tunnel", false, true)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				listen := fmt.Sprintf("%s: %d", ui.text("Listen"), entry.Rule.ListenPort)
				if entry.Rule.ListenHost != "" {
					listen = fmt.Sprintf("%s: %s:%d", ui.text("Listen"), entry.Rule.ListenHost, entry.Rule.ListenPort)
				}
				label := material.Body2(ui.theme, listen)
				label.Color = colorMuted
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if entry.Rule.Type == remote.SSHTunnelDynamic {
					return layout.Dimensions{}
				}
				target := fmt.Sprintf("%s: %s:%d", ui.text("Target"), entry.Rule.TargetHost, entry.Rule.TargetPort)
				label := material.Body2(ui.theme, target)
				label.Color = colorMuted
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if entry.Runtime == nil {
					if entry.Error == "" {
						return layout.Dimensions{}
					}
					label := material.Caption(ui.theme, entry.Error)
					label.Color = colorDanger
					return label.Layout(gtx)
				}
				up, down := entry.Runtime.Traffic()
				traffic := fmt.Sprintf("%s: %s %d B, %s %d B", ui.text("Traffic"), ui.text("Up"), up, ui.text("Down"), down)
				label := material.Caption(ui.theme, traffic)
				label.Color = colorMuted
				return label.Layout(gtx)
			}),
		)
	})
}
