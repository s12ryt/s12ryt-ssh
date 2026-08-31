package gui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *Window) handleSSHKeyIdentityForm(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sshKeyName, &ui.sshKeyPublicKey, &ui.sshKeyFingerprint, &ui.sshKeyPrivateKey, &ui.sshKeyPassphrase) {
		ui.submitSSHKeyIdentityForm()
		return
	}
	ui.sshKeyClearSecrets.Update(gtx)
	ui.sshKeyEnabled.Update(gtx)
	if ui.sshKeyFormClose.Clicked(gtx) || ui.sshKeyFormCancel.Clicked(gtx) || ui.sshKeyFormScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHKeyIdentityForm()
		return
	}
	if ui.sshKeyFormDelete.Clicked(gtx) && ui.sshKeyFormID != "" {
		id := ui.sshKeyFormID
		ui.closeSSHKeyIdentityForm()
		ui.deleteSSHKeyIdentity(id)
		return
	}
	if ui.sshKeyFormSave.Clicked(gtx) {
		ui.submitSSHKeyIdentityForm()
	}
}

func (ui *Window) sshKeyIdentityFormModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshKeyFormScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshKeyIdentityFormDialog(gtx)
		}),
	)
}

func (ui *Window) sshKeyIdentityFormDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(620))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(680))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			titleSource := "New key"
			if ui.sshKeyFormID != "" {
				titleSource = "Edit key"
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, ui.text(titleSource))
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshKeyFormClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ui.sshKeyFormList.Axis = layout.Vertical
					return ui.sshKeyFormList.Layout(gtx, 8, func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(sectionGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							switch index {
							case 0:
								return ui.labeledField(gtx, &ui.sshKeyName, "Key name", true, false)
							case 1:
								return ui.labeledField(gtx, &ui.sshKeyPublicKey, "Public key", false, false)
							case 2:
								return ui.labeledField(gtx, &ui.sshKeyFingerprint, "Fingerprint", true, false)
							case 3:
								return ui.sshKeyMaterialField(gtx)
							case 4:
								return ui.labeledField(gtx, &ui.sshKeyPassphrase, "Key passphrase", true, true)
							case 5:
								if !ui.sshKeyForm.HasPassphrase {
									return layout.Dimensions{}
								}
								label := material.Caption(ui.theme, ui.text("Saved key has a passphrase."))
								label.Color = colorMuted
								return label.Layout(gtx)
							case 6:
								return material.CheckBox(ui.theme, &ui.sshKeyClearSecrets, ui.text("Clear saved key material")).Layout(gtx)
							case 7:
								return material.CheckBox(ui.theme, &ui.sshKeyEnabled, ui.text("Enabled")).Layout(gtx)
							}
							return layout.Dimensions{}
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshKeyFormCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshKeyFormSave, "Save key", true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.sshKeyFormID == "" {
								return layout.Dimensions{}
							}
							return ui.actionButton(gtx, &ui.sshKeyFormDelete, "Delete key?", false, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshKeyMaterialField(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.fieldLabel(gtx, "Private key material")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ui.sshKeyPrivateKey.SingleLine = false
			ui.sshKeyPrivateKey.Submit = false
			gtx.Constraints.Min.Y = gtx.Dp(privateKeyMinHeight)
			return ui.field(gtx, &ui.sshKeyPrivateKey, "Private key material", false, true)
		}),
	)
}
