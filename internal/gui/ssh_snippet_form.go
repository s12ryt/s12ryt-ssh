package gui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *Window) handleSSHCommandSnippetForm(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sshSnippetName, &ui.sshSnippetCommand, &ui.sshSnippetVariables, &ui.sshSnippetSecrets) {
		ui.submitSSHCommandSnippetForm()
		return
	}
	ui.sshSnippetClearSecrets.Update(gtx)
	ui.sshSnippetEnabled.Update(gtx)
	if ui.sshSnippetFormClose.Clicked(gtx) || ui.sshSnippetFormCancel.Clicked(gtx) || ui.sshSnippetFormScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHCommandSnippetForm()
		return
	}
	if ui.sshSnippetFormDelete.Clicked(gtx) && ui.sshSnippetFormID != "" {
		id := ui.sshSnippetFormID
		ui.closeSSHCommandSnippetForm()
		ui.deleteSSHCommandSnippet(id)
		return
	}
	if ui.sshSnippetFormSave.Clicked(gtx) {
		ui.submitSSHCommandSnippetForm()
	}
}

func (ui *Window) sshCommandSnippetFormModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshSnippetFormScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshCommandSnippetFormDialog(gtx)
		}),
	)
}

func (ui *Window) sshCommandSnippetFormDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(620))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			titleSource := "New snippet"
			if ui.sshSnippetFormID != "" {
				titleSource = "Edit snippet"
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
							return ui.button(gtx, &ui.sshSnippetFormClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ui.sshSnippetFormList.Axis = layout.Vertical
					return ui.sshSnippetFormList.Layout(gtx, 6, func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(sectionGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							switch index {
							case 0:
								return ui.labeledField(gtx, &ui.sshSnippetName, "Snippet name", true, false)
							case 1:
								return ui.snippetMultilineField(gtx, &ui.sshSnippetCommand, "Command", false)
							case 2:
								return ui.labeledField(gtx, &ui.sshSnippetVariables, "Variables (comma-separated)", false, false)
							case 3:
								return ui.labeledField(gtx, &ui.sshSnippetSecrets, "Secret values (NAME=value)", false, true)
							case 4:
								return ui.sshSnippetSavedSecretsField(gtx)
							case 5:
								return material.CheckBox(ui.theme, &ui.sshSnippetEnabled, ui.text("Enabled")).Layout(gtx)
							}
							return layout.Dimensions{}
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSnippetFormCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSnippetFormSave, "Save snippet", true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.sshSnippetFormID == "" {
								return layout.Dimensions{}
							}
							return ui.actionButton(gtx, &ui.sshSnippetFormDelete, "Delete snippet?", false, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) snippetMultilineField(gtx layout.Context, editor *widget.Editor, hint string, password bool) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.fieldLabel(gtx, hint) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			editor.SingleLine = false
			editor.Submit = false
			gtx.Constraints.Min.Y = gtx.Dp(96)
			return ui.field(gtx, editor, hint, false, password)
		}),
	)
}

func (ui *Window) sshSnippetSavedSecretsField(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.fieldLabel(gtx, "Saved secret names") }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(ui.theme, ui.sshSnippetSavedSecretNames)
			label.Color = colorMuted
			if ui.sshSnippetSavedSecretNames == "" {
				label = material.Caption(ui.theme, "-")
				label.Color = colorMuted
			}
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(ui.theme, &ui.sshSnippetClearSecrets, ui.text("Clear saved secrets")).Layout(gtx)
		}),
	)
}
