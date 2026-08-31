package gui

import (
	"fmt"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/remote"
)

func (ui *Window) handleSSHWorkspaceExportForm(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.workspaceExportPassword) {
		ui.currentSSHWorkspaceExportForm()
		ui.submitOrChooseSSHWorkspaceExport()
		return
	}
	ui.workspaceExportIncludeSecrets.Update(gtx)
	if !ui.workspaceExportIncludeSecrets.Value {
		ui.workspaceExportPassword.SetText("")
	}
	if ui.workspaceExportClose.Clicked(gtx) || ui.workspaceExportCancel.Clicked(gtx) || ui.workspaceExportScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHWorkspaceExportForm()
		return
	}
	if ui.workspaceExportSubmit.Clicked(gtx) {
		ui.currentSSHWorkspaceExportForm()
		ui.submitOrChooseSSHWorkspaceExport()
	}
}

func (ui *Window) submitOrChooseSSHWorkspaceExport() {
	if ui.workspaceExportPath == "" {
		ui.requestSSHWorkspaceExport()
		return
	}
	ui.submitSSHWorkspaceExport()
}

func (ui *Window) handleSSHWorkspaceImportForm(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.workspaceImportPasswordEditor) {
		ui.syncSSHWorkspaceImportPassword()
		ui.previewSSHWorkspaceImport()
		return
	}
	ui.syncSSHWorkspaceImportPassword()
	if ui.workspaceImportClose.Clicked(gtx) || ui.workspaceImportCancel.Clicked(gtx) || ui.workspaceImportScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSSHWorkspaceImportForm()
		return
	}
	if ui.workspaceImportPreview.Clicked(gtx) {
		ui.previewSSHWorkspaceImport()
		return
	}
	ui.syncSSHWorkspaceImportConflictButtons()
	for index, key := range ui.workspaceImportConflictKeys {
		if index >= len(ui.workspaceImportConflictButtons) {
			break
		}
		for buttonIndex, action := range []remote.SSHWorkspaceImportDecision{
			remote.SSHWorkspaceImportOverwrite,
			remote.SSHWorkspaceImportSkip,
			remote.SSHWorkspaceImportCopy,
		} {
			if ui.workspaceImportConflictButtons[index][buttonIndex].Clicked(gtx) {
				ui.setSSHWorkspaceImportResolution(key, action)
				return
			}
		}
	}
	if ui.workspaceImportApply.Clicked(gtx) {
		ui.applySSHWorkspaceImport()
	}
}

func (ui *Window) setSSHWorkspaceImportResolution(key string, action remote.SSHWorkspaceImportDecision) {
	if ui == nil || ui.workspaceImport == nil {
		return
	}
	for _, conflict := range ui.workspaceImport.Conflicts {
		if conflict.Conflict && sshWorkspaceImportDecisionKey(conflict.Kind, conflict.Name) == key {
			ui.workspaceImport.setResolution(conflict.Kind, conflict.Name, action)
			return
		}
	}
}

func (ui *Window) workspaceImportExportModal(gtx layout.Context) layout.Dimensions {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			var scrimButton *widget.Clickable
			if ui.workspaceExportOpen {
				scrimButton = &ui.workspaceExportScrim
			} else {
				scrimButton = &ui.workspaceImportScrim
			}
			return scrimButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if ui.workspaceExportOpen {
				return ui.sshWorkspaceExportDialog(gtx)
			}
			return ui.sshWorkspaceImportDialog(gtx)
		}),
	)
}

func (ui *Window) sshWorkspaceExportDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(420))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.workspaceModalTitle(gtx, "Export workspace", &ui.workspaceExportClose)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(ui.theme, &ui.workspaceExportIncludeSecrets, ui.text("Include secrets")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !ui.workspaceExportIncludeSecrets.Value {
						return layout.Dimensions{}
					}
					return ui.labeledField(gtx, &ui.workspaceExportPassword, "Export password", true, true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.workspacePackagePath(gtx, "Export workspace package", ui.workspaceExportPath)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.workspaceExportCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.workspaceExportSubmit, "Export workspace", true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshWorkspaceImportDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(640))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(620))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.workspaceModalTitle(gtx, "Import workspace", &ui.workspaceImportClose)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.workspacePackagePath(gtx, "Import workspace package", ui.workspaceImportPath)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.labeledField(gtx, &ui.workspaceImportPasswordEditor, "Import password", true, true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.workspaceImportPreview, "Preview import", false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if ui.workspaceImport == nil {
						return layout.Dimensions{}
					}
					ui.syncSSHWorkspaceImportConflictButtons()
					ui.workspaceImportList.Axis = layout.Vertical
					return ui.workspaceImportList.Layout(gtx, 2+len(ui.workspaceImportConflictKeys), func(gtx layout.Context, index int) layout.Dimensions {
						if index == 0 {
							return ui.workspaceImportCounts(gtx)
						}
						if index == 1 {
							if len(ui.workspaceImportConflictKeys) == 0 {
								return material.Body2(ui.theme, ui.text("No import conflicts.")).Layout(gtx)
							}
							return material.Subtitle2(ui.theme, ui.text("Resolve import conflicts")).Layout(gtx)
						}
						return ui.workspaceImportConflictRow(gtx, index-2)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.workspaceImportCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.workspaceImport == nil {
								return layout.Dimensions{}
							}
							return ui.button(gtx, &ui.workspaceImportApply, "Apply import", true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) workspaceModalTitle(gtx layout.Context, source string, close *widget.Clickable) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			title := material.H6(ui.theme, ui.text(source))
			title.Color = colorText
			return title.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, close, "Close", false)
		}),
	)
}

func (ui *Window) workspacePackagePath(gtx layout.Context, source, path string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.fieldLabel(gtx, source) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if path == "" {
				path = "-"
			}
			label := material.Caption(ui.theme, path)
			label.Color = colorMuted
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

func (ui *Window) workspaceImportCounts(gtx layout.Context) layout.Dimensions {
	counts := ui.workspaceImportCountsText()
	label := material.Caption(ui.theme, counts)
	label.Color = colorMuted
	return label.Layout(gtx)
}

func (ui *Window) workspaceImportCountsText() string {
	if ui == nil || ui.workspaceImport == nil {
		return ""
	}
	return fmt.Sprintf("%s %d  %s %d  %s %d  %s %d", ui.text("Hosts"), ui.workspaceImportPreviewCount(remote.SSHWorkspaceImportHost), ui.text("Port forwarding"), ui.workspaceImportPreviewCount(remote.SSHWorkspaceImportTunnel), ui.text("Command snippets"), ui.workspaceImportPreviewCount(remote.SSHWorkspaceImportSnippet), ui.text("Key management"), ui.workspaceImportPreviewCount(remote.SSHWorkspaceImportKey))
}

func (ui *Window) workspaceImportPreviewCount(kind remote.SSHWorkspaceImportKind) int {
	if ui == nil || ui.workspaceImport == nil {
		return 0
	}
	switch kind {
	case remote.SSHWorkspaceImportHost:
		return ui.workspaceImport.Counts.Hosts
	case remote.SSHWorkspaceImportTunnel:
		return ui.workspaceImport.Counts.Tunnels
	case remote.SSHWorkspaceImportSnippet:
		return ui.workspaceImport.Counts.Snippets
	case remote.SSHWorkspaceImportKey:
		return ui.workspaceImport.Counts.Keys
	default:
		return 0
	}
}

func (ui *Window) workspaceImportConflictRow(gtx layout.Context, index int) layout.Dimensions {
	if ui == nil || ui.workspaceImport == nil || index < 0 || index >= len(ui.workspaceImportConflictKeys) {
		return layout.Dimensions{}
	}
	key := ui.workspaceImportConflictKeys[index]
	var conflict remote.SSHWorkspaceImportConflict
	for _, candidate := range ui.workspaceImport.Conflicts {
		if sshWorkspaceImportDecisionKey(candidate.Kind, candidate.Name) == key {
			conflict = candidate
			break
		}
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(ui.theme, fmt.Sprintf("%s: %s", ui.text(workspaceImportKindSource(conflict.Kind)), conflict.Name))
			label.Color = colorText
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.workspaceImportConflictButtons[index][0], "Overwrite", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.workspaceImportConflictButtons[index][1], "Skip", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.workspaceImportConflictButtons[index][2], "Copy", true)
				}),
			)
		}),
	)
}

func workspaceImportKindSource(kind remote.SSHWorkspaceImportKind) string {
	switch kind {
	case remote.SSHWorkspaceImportHost:
		return "Hosts"
	case remote.SSHWorkspaceImportTunnel:
		return "Port forwarding"
	case remote.SSHWorkspaceImportSnippet:
		return "Command snippets"
	case remote.SSHWorkspaceImportKey:
		return "Key management"
	default:
		return "Import workspace"
	}
}
