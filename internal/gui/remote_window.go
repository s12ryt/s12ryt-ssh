package gui

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *Window) handleRemoteLogin(gtx layout.Context) {
	ui.remoteRememberPassword.Update(gtx)
	if ui.drainEditors(gtx, &ui.remoteURL, &ui.remoteUsername, &ui.remotePassword) {
		ui.remoteAutoLoginPending = false
		ui.remoteAutoLoginInFlight = false
		ui.tryRemoteSignIn()
		return
	}
	if ui.remoteLogin.Clicked(gtx) {
		ui.remoteAutoLoginPending = false
		ui.remoteAutoLoginInFlight = false
		ui.tryRemoteSignIn()
	}
	if ui.remoteRestore.Clicked(gtx) {
		if ui.model.RemoteService == nil {
			ui.model.Error = "Remote authentication service is unavailable"
			return
		}
		service := ui.model.RemoteService
		ui.async("Restoring remote session...", func(ctx context.Context) (func(), error) {
			session, err := service.Restore(ctx)
			if err != nil {
				return nil, err
			}
			overview, err := session.ResourcesOverview(ctx)
			if err != nil {
				_ = session.Logout(ctx)
				return nil, err
			}
			return func() { ui.activateRemoteSession(session, overview.SSHEnabled) }, nil
		})
	}
}

func (ui *Window) activateRemoteSession(session RemoteSession, sshEnabled bool) {
	ui.model.SetRemoteSession(session, sshEnabled)
	if sshEnabled {
		if _, ok := session.(remoteWorkspacePreferencesSession); ok {
			ui.refreshTerminalAppearance()
			return
		}
		if len(ui.sshHosts) == 0 {
			ui.refreshSSHHosts()
		}
		return
	}
	ui.model.Status = ui.text("SSH access is not enabled for this account.")
}

func (ui *Window) handleRemoteWorkspace(gtx layout.Context) {
	if ui.logout.Clicked(gtx) {
		ui.stopAllSSHTunnels()
		ui.closeSSH()
		ui.closeSSHHostForm()
		ui.closeSSHTabRename()
		ui.closeSFTPOperation()
		ui.closeSFTPUploadConflicts()
		ui.closeSSHTunnelForm()
		ui.closeSSHCommandSnippetExecution()
		ui.closeSSHCommandSnippetForm()
		ui.closeSSHKeyIdentityForm()
		ui.closeManualSSHHostFingerprint()
		ui.closeSSHWorkspaceImportExport()
		ui.closeTerminalAppearanceForm()
		if ui.sshTunnels != nil {
			ui.sshTunnels.closeAll()
		}
		session := ui.model.RemoteSession
		ui.asyncAlways("Signing out...", func(ctx context.Context) (func(), error) {
			var err error
			if session != nil {
				err = session.Logout(ctx)
			}
			return func() {
				ui.model.finishRemoteLogout()
				ui.sshHosts = nil
				ui.sshHostButtons = nil
				ui.sshHostEditButtons = nil
				ui.clearSSHKeyIdentityView()
				ui.clearSSHHostFingerprintView()
				ui.clearSSHSessionHistoryView()
			}, err
		})
		return
	}
	if ui.model.SSHEnabled {
		ui.handleSSHWorkspace(gtx)
	}
}

func (ui *Window) remoteLoginView(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(loginCardWidth)
		return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(cardPaddingLoose), Bottom: unit.Dp(cardPaddingLoose), Left: unit.Dp(cardPaddingLoose), Right: unit.Dp(cardPaddingLoose)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H5(ui.theme, ui.text("Sign in with authentication service")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(ui.theme, ui.text("The password is saved only when Remember password is enabled."))
						label.Color = colorMuted
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.remoteURL, "Authentication service URL", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.remoteUsername, "Account", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.remotePassword, "Password", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(ui.theme, &ui.remoteRememberPassword, ui.text("Remember password")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(ui.theme, ui.text("The password is protected by Windows and kept after sign-out."))
						label.Color = colorMuted
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.actionButtonBlock(gtx, &ui.remoteLogin, "Sign in remotely", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.buttonBlock(gtx, &ui.remoteRestore, "Restore saved session", false)
					}),
				)
			})
		})
	})
}

func (ui *Window) remoteWorkspaceView(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap), Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.fieldLabel(gtx, "Remote account: ")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(ui.theme, ui.model.RemoteAccountName)
					label.Color = colorText
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !ui.model.SSHEnabled {
				return ui.remoteSSHDisabledView(gtx)
			}
			return ui.workspaceShell(gtx)
		}),
	)
}

func (ui *Window) remoteSSHDisabledView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24), Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Body1(ui.theme, ui.text("SSH access is not enabled for this account.")).Layout(gtx)
			})
		})
	})
}

func validateRemoteCredentials(rawURL, username, password string) error {
	if strings.TrimSpace(rawURL) == "" || strings.TrimSpace(username) == "" || password == "" {
		return fmt.Errorf("Remote sign-in URL, account, and password are required")
	}
	return nil
}
