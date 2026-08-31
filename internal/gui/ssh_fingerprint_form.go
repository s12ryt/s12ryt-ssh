package gui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *Window) handleManualSSHHostFingerprint(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sshFingerprintManualEditor) {
		ui.submitManualSSHHostFingerprint()
		return
	}
	if ui.sshFingerprintManualClose.Clicked(gtx) || ui.sshFingerprintManualCancel.Clicked(gtx) ||
		ui.sshFingerprintManualScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeManualSSHHostFingerprint()
		return
	}
	if ui.sshFingerprintManualSave.Clicked(gtx) {
		ui.submitManualSSHHostFingerprint()
	}
}

func (ui *Window) manualSSHHostFingerprintModal(gtx layout.Context) {
	scrim := color.NRGBA{A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshFingerprintManualScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
			return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									title := material.H6(ui.theme, ui.text("Manual host fingerprint"))
									title.Color = colorText
									return title.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.button(gtx, &ui.sshFingerprintManualClose, "Close", false)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.labeledField(gtx, &ui.sshFingerprintManualEditor, "Fingerprint", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.button(gtx, &ui.sshFingerprintManualCancel, "Cancel", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.button(gtx, &ui.sshFingerprintManualSave, "Trust manual fingerprint", true)
								}),
							)
						}),
					)
				})
			})
		}),
	)
}
