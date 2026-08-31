package gui

import (
	"context"
	"errors"
	"strconv"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"s12ryt-ssh/internal/remote"
)

type remoteWorkspacePreferencesSession interface {
	SSHWorkspacePreferences(context.Context) (remote.SSHWorkspacePreferences, error)
	UpdateSSHWorkspacePreferences(context.Context, remote.SSHWorkspacePreferencesInput) (remote.SSHWorkspacePreferences, error)
}

func (ui *Window) refreshTerminalAppearance() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil || ui.busy {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteWorkspacePreferencesSession)
	if !ok {
		return false
	}
	ui.async("Loading terminal appearance...", func(ctx context.Context) (func(), error) {
		preferences, err := session.SSHWorkspacePreferences(ctx)
		if err != nil {
			return nil, err
		}
		return func() {
			ui.terminalAppearance = terminalAppearanceFromRemote(preferences.TerminalAppearance)
			if len(ui.sshHosts) == 0 {
				ui.refreshSSHHosts()
			}
		}, nil
	})
	return true
}

func (ui *Window) openTerminalAppearanceForm(hostID string) bool {
	if ui == nil || ui.terminalAppearanceOpen || ui.busy {
		return false
	}
	appearance := normalizeTerminalAppearance(ui.terminalAppearance)
	values := terminalAppearanceFormValues{
		Scope: terminalAppearanceAccount, Font: string(appearance.Font),
		FontSize:   strconv.FormatFloat(float64(appearance.FontSize), 'f', -1, 32),
		Foreground: appearance.Foreground, Background: appearance.Background,
	}
	if hostID != "" {
		for _, host := range ui.sshHosts {
			if host.ID != hostID {
				continue
			}
			values.Scope = terminalAppearanceHost
			values.HostID = hostID
			values.UseAccountDefault = host.Settings.TerminalAppearance == nil
			if !values.UseAccountDefault {
				appearance = terminalAppearanceForHost(appearance, host)
				values.Font = string(appearance.Font)
				values.FontSize = strconv.FormatFloat(float64(appearance.FontSize), 'f', -1, 32)
				values.Foreground, values.Background = appearance.Foreground, appearance.Background
			}
			break
		}
	}
	ui.terminalAppearanceForm = values
	ui.terminalAppearanceOpen = true
	ui.setTerminalAppearanceFormEditors(values)
	return true
}

func (ui *Window) setTerminalAppearanceFormEditors(values terminalAppearanceFormValues) {
	ui.terminalAppearanceFont.SetText(values.Font)
	ui.terminalAppearanceFontSize.SetText(values.FontSize)
	ui.terminalAppearanceForeground.SetText(values.Foreground)
	ui.terminalAppearanceBackground.SetText(values.Background)
	ui.terminalAppearanceUseAccountDefault.Value = values.UseAccountDefault
}

func (ui *Window) currentTerminalAppearanceForm() terminalAppearanceFormValues {
	values := ui.terminalAppearanceForm
	values.Font = ui.terminalAppearanceFont.Text()
	values.FontSize = ui.terminalAppearanceFontSize.Text()
	values.Foreground = ui.terminalAppearanceForeground.Text()
	values.Background = ui.terminalAppearanceBackground.Text()
	values.UseAccountDefault = ui.terminalAppearanceUseAccountDefault.Value
	return values
}

func (ui *Window) closeTerminalAppearanceForm() {
	ui.terminalAppearanceOpen = false
	ui.terminalAppearanceForm = terminalAppearanceFormValues{}
	ui.terminalAppearanceFont.SetText("")
	ui.terminalAppearanceFontSize.SetText("")
	ui.terminalAppearanceForeground.SetText("")
	ui.terminalAppearanceBackground.SetText("")
	ui.terminalAppearanceUseAccountDefault.Value = false
	ui.terminalAppearanceClose = widget.Clickable{}
	ui.terminalAppearanceCancel = widget.Clickable{}
	ui.terminalAppearanceSave = widget.Clickable{}
	ui.terminalAppearanceScrim = widget.Clickable{}
}

func (ui *Window) submitTerminalAppearanceForm() bool {
	values := ui.currentTerminalAppearanceForm()
	appearance, err := values.input()
	if err != nil {
		ui.model.Error = ui.text("Terminal appearance is invalid.")
		return false
	}
	ui.closeTerminalAppearanceForm()
	if values.Scope == terminalAppearanceHost {
		for _, host := range ui.sshHosts {
			if host.ID != values.HostID {
				continue
			}
			input := remote.SSHHostInput{Name: host.Name, Host: host.Host, Port: host.Port, Username: host.Username, Settings: &host.Settings}
			input.Settings.TerminalAppearance = &remote.SSHTerminalAppearanceOverride{
				Font: appearance.Font, FontSize: appearance.FontSize, Foreground: appearance.Foreground, Background: appearance.Background,
			}
			if values.clearOverride() {
				input.ClearTerminalAppearance = true
			}
			ui.async(ui.text("Saving terminal appearance..."), func(ctx context.Context) (func(), error) {
				updated, err := ui.model.RemoteSession.UpdateSSHHost(ctx, host.ID, input)
				if err != nil {
					return nil, err
				}
				return func() { ui.replaceSSHHost(updated) }, nil
			})
			return true
		}
		return false
	}
	session, err := ui.optionalWorkspacePreferencesSession()
	if err != nil {
		ui.model.Error = err.Error()
		return false
	}
	ui.async(ui.text("Saving terminal appearance..."), func(ctx context.Context) (func(), error) {
		updated, err := session.UpdateSSHWorkspacePreferences(ctx, remote.SSHWorkspacePreferencesInput{TerminalAppearance: appearance})
		if err != nil {
			return nil, err
		}
		return func() { ui.terminalAppearance = terminalAppearanceFromRemote(updated.TerminalAppearance) }, nil
	})
	return true
}

func (ui *Window) handleTerminalAppearanceForm(gtx layout.Context) {
	ui.terminalAppearanceUseAccountDefault.Update(gtx)
	ui.drainEditors(gtx, &ui.terminalAppearanceFont, &ui.terminalAppearanceFontSize, &ui.terminalAppearanceForeground, &ui.terminalAppearanceBackground)
	if ui.terminalAppearanceClose.Clicked(gtx) || ui.terminalAppearanceCancel.Clicked(gtx) || ui.terminalAppearanceScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeTerminalAppearanceForm()
		return
	}
	if ui.terminalAppearanceSave.Clicked(gtx) {
		ui.submitTerminalAppearanceForm()
	}
}

func (ui *Window) terminalAppearanceModal(gtx layout.Context) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			ui.terminalAppearanceScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(520)
			gtx.Constraints.Max.Y = gtx.Dp(520)
			return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(16), Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.H5(ui.theme, ui.text("Terminal appearance")).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Body2(ui.theme, ui.text("Account default")).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.labeledField(gtx, &ui.terminalAppearanceFont, "Font", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.labeledField(gtx, &ui.terminalAppearanceFontSize, "Font size", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.labeledField(gtx, &ui.terminalAppearanceForeground, "Foreground", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.labeledField(gtx, &ui.terminalAppearanceBackground, "Background", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.CheckBox(ui.theme, &ui.terminalAppearanceUseAccountDefault, ui.text("Use account default")).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.buttonBlock(gtx, &ui.terminalAppearanceCancel, "Cancel", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.actionButtonBlock(gtx, &ui.terminalAppearanceSave, "Save appearance", true, false)
								}),
							)
						}),
					)
				})
			})
		}),
	)
}

func (ui *Window) replaceSSHHost(updated remote.SSHHost) {
	for i := range ui.sshHosts {
		if ui.sshHosts[i].ID == updated.ID {
			ui.sshHosts[i] = updated
			break
		}
	}
}

func (ui *Window) optionalWorkspacePreferencesSession() (remoteWorkspacePreferencesSession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote session is not active")
	}
	session, ok := ui.model.RemoteSession.(remoteWorkspacePreferencesSession)
	if !ok {
		return nil, errors.New("workspace preferences service is unavailable")
	}
	return session, nil
}
