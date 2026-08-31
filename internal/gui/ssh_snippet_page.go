package gui

import (
	"image/color"
	"strings"

	"s12ryt-ssh/internal/remote"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func filteredSSHCommandSnippetEntries(entries []sshCommandSnippetEntry, query string) []sshCommandSnippetEntry {
	filtered := make([]sshCommandSnippetEntry, 0, len(entries))
	for _, entry := range entries {
		if len(filterSSHCommandSnippets([]remote.SSHCommandSnippet{entry.Snippet}, query)) == 0 {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func (ui *Window) syncSSHCommandSnippetButtons(entries []sshCommandSnippetEntry) {
	ids := make([]string, len(entries))
	for index, entry := range entries {
		ids[index] = entry.Snippet.ID
	}
	if len(ids) == len(ui.sshSnippetVisibleIDs) && len(ids) == len(ui.sshSnippetExecuteBtns) && len(ids) == len(ui.sshSnippetEditBtns) && len(ids) == len(ui.sshSnippetDeleteBtns) {
		matches := true
		for index, id := range ids {
			if ui.sshSnippetVisibleIDs[index] != id {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	ui.sshSnippetVisibleIDs = ids
	ui.sshSnippetExecuteBtns = make([]widget.Clickable, len(entries))
	ui.sshSnippetEditBtns = make([]widget.Clickable, len(entries))
	ui.sshSnippetDeleteBtns = make([]widget.Clickable, len(entries))
}

func (ui *Window) handleSSHCommandSnippets(gtx layout.Context) bool {
	if ui.sshSnippets == nil {
		return true
	}
	entries := filteredSSHCommandSnippetEntries(ui.sshSnippets.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHCommandSnippetButtons(entries)
	if ui.sshSnippetNew.Clicked(gtx) {
		ui.openSSHCommandSnippetForm("")
		return true
	}
	for index, entry := range entries {
		if entry.Snippet.Enabled && ui.sshSnippetExecuteBtns[index].Clicked(gtx) {
			ui.openSSHCommandSnippetExecution(entry.Snippet.ID)
			return true
		}
		if ui.sshSnippetEditBtns[index].Clicked(gtx) {
			ui.openSSHCommandSnippetForm(entry.Snippet.ID)
			return true
		}
		if ui.sshSnippetDeleteBtns[index].Clicked(gtx) {
			ui.deleteSSHCommandSnippet(entry.Snippet.ID)
			return true
		}
	}
	return true
}

func (ui *Window) sshCommandSnippetPage(gtx layout.Context) layout.Dimensions {
	if ui.sshSnippets == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.Body1(ui.theme, ui.text("No command snippets yet.")).Layout(gtx)
		})
	}
	entries := filteredSSHCommandSnippetEntries(ui.sshSnippets.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHCommandSnippetButtons(entries)
	if len(entries) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(ui.theme, ui.text("No command snippets yet."))
			label.Color = colorMuted
			return label.Layout(gtx)
		})
	}
	ui.sshSnippetList.Axis = layout.Vertical
	return ui.sshSnippetList.Layout(gtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshCommandSnippetCard(gtx, entries[index], &ui.sshSnippetExecuteBtns[index], &ui.sshSnippetEditBtns[index], &ui.sshSnippetDeleteBtns[index])
		})
	})
}

func (ui *Window) sshCommandSnippetCard(gtx layout.Context, entry sshCommandSnippetEntry, execute, edit, delete *widget.Clickable) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.Subtitle1(ui.theme, entry.Snippet.Name)
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if entry.Snippet.Enabled {
								return ui.button(gtx, execute, "Execute", true)
							}
							label := material.Caption(ui.theme, ui.text("Snippet is disabled."))
							label.Color = colorDanger
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, edit, "Edit snippet", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, delete, "Delete snippet?", false, true)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					command := material.Label(ui.theme, unit.Sp(14), entry.Snippet.Command)
					command.Font.Typeface = monoTypeface
					command.Color = colorTeal
					command.MaxLines = 4
					return command.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					values := make([]string, 0, len(entry.Snippet.Variables)+len(entry.Snippet.SecretNames))
					values = append(values, entry.Snippet.Variables...)
					values = append(values, entry.Snippet.SecretNames...)
					if len(values) == 0 {
						return layout.Dimensions{}
					}
					label := material.Caption(ui.theme, strings.Join(values, "  "))
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
			)
		})
	})
}

func (ui *Window) handleSSHCommandSnippetExecution(gtx layout.Context) {
	if ui.drainEditors(gtx, ui.sshSnippetVariableEditorPointers()...) {
		ui.submitSSHCommandSnippetExecution()
		return
	}
	if ui.sshSnippetExecutionClose.Clicked(gtx) || ui.sshSnippetExecutionCancel.Clicked(gtx) || ui.sshSnippetExecutionScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHCommandSnippetExecution()
		return
	}
	if ui.sshSnippetExecutionRun.Clicked(gtx) {
		ui.submitSSHCommandSnippetExecution()
	}
}

func (ui *Window) sshSnippetVariableEditorPointers() []*widget.Editor {
	editors := make([]*widget.Editor, len(ui.sshSnippetVariableEditors))
	for index := range ui.sshSnippetVariableEditors {
		editors[index] = &ui.sshSnippetVariableEditors[index]
	}
	return editors
}

func (ui *Window) submitSSHCommandSnippetExecution() {
	id := ui.sshSnippetExecutionID
	if id == "" {
		return
	}
	started := ui.executeSSHCommandSnippet(id)
	if started {
		ui.closeSSHCommandSnippetExecution()
	}
}

func (ui *Window) sshCommandSnippetExecutionModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshSnippetExecutionScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshCommandSnippetExecutionDialog(gtx)
		}),
	)
}

func (ui *Window) sshCommandSnippetExecutionDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(520))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(560))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, ui.text("Run command snippet"))
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSnippetExecutionClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := material.Caption(ui.theme, ui.text("Secret values are loaded only when executing."))
					message.Color = colorMuted
					return message.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ui.sshSnippetExecutionList.Axis = layout.Vertical
					return ui.sshSnippetExecutionList.Layout(gtx, len(ui.sshSnippetVariableNames), func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(sectionGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := material.Caption(ui.theme, ui.sshSnippetVariableNames[index])
									label.Color = colorText
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.editorField(gtx, &ui.sshSnippetVariableEditors[index], "Variable value")
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSnippetExecutionCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSnippetExecutionRun, "Execute", true)
						}),
					)
				}),
			)
		})
	})
}
