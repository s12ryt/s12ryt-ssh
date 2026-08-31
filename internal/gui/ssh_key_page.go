package gui

import (
	"s12ryt-ssh/internal/remote"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *Window) handleSSHKeyIdentities(gtx layout.Context) bool {
	if ui == nil || ui.sshKeys == nil {
		return true
	}
	entries := filteredSSHKeyIdentityEntries(ui.sshKeys.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHKeyIdentityButtons(entries)
	if ui.sshKeyNew.Clicked(gtx) {
		ui.openSSHKeyIdentityForm("")
		return true
	}
	for index, entry := range entries {
		if ui.sshKeyEditBtns[index].Clicked(gtx) {
			ui.openSSHKeyIdentityForm(entry.Key.ID)
			return true
		}
		if ui.sshKeyDeleteBtns[index].Clicked(gtx) {
			ui.deleteSSHKeyIdentity(entry.Key.ID)
			return true
		}
	}
	return true
}

func (ui *Window) sshKeyIdentityPage(gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.sshKeys == nil {
		return ui.sshKeyIdentityEmpty(gtx, "No key identities yet.")
	}
	entries := filteredSSHKeyIdentityEntries(ui.sshKeys.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHKeyIdentityButtons(entries)
	if len(entries) == 0 {
		message := "No key identities yet."
		if ui.workspaceSearch.Text() != "" {
			message = "No key identities match this search."
		}
		return ui.sshKeyIdentityEmpty(gtx, message)
	}
	ui.sshKeyList.Axis = layout.Vertical
	return ui.sshKeyList.Layout(gtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshKeyIdentityCard(gtx, entries[index], &ui.sshKeyEditBtns[index], &ui.sshKeyDeleteBtns[index])
		})
	})
}

func (ui *Window) sshKeyIdentityEmpty(gtx layout.Context, source string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(ui.theme, ui.text(source))
		label.Color = colorMuted
		return label.Layout(gtx)
	})
}

func (ui *Window) sshKeyIdentityCard(gtx layout.Context, entry sshKeyIdentityEntry, edit, delete *widget.Clickable) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.Subtitle1(ui.theme, entry.Key.Name)
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.keyIdentityStatus(gtx, entry.Key)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.keyIdentityMetadata(gtx, entry.Key)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if entry.Error == "" {
						return layout.Dimensions{}
					}
					label := material.Caption(ui.theme, entry.Error)
					label.Color = colorDanger
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, edit, "Edit key", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, delete, "Delete key?", false, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) keyIdentityStatus(gtx layout.Context, key remote.SSHKeyIdentity) layout.Dimensions {
	if !key.Enabled {
		label := material.Caption(ui.theme, ui.text("Key identity is disabled."))
		label.Color = colorDanger
		return label.Layout(gtx)
	}
	if key.HasPassphrase {
		label := material.Caption(ui.theme, ui.text("Saved key has a passphrase."))
		label.Color = colorTeal
		return label.Layout(gtx)
	}
	return layout.Dimensions{}
}

func (ui *Window) keyIdentityMetadata(gtx layout.Context, key remote.SSHKeyIdentity) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.fieldLabel(gtx, "Public key")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Label(ui.theme, unit.Sp(13), key.PublicKey)
			label.Font.Typeface = monoTypeface
			label.Color = colorText
			label.MaxLines = 2
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.fieldLabel(gtx, "Fingerprint")
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, key.Fingerprint)
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
			)
		}),
	)
}
