package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/remote"
)

const (
	workspaceSidebarWidth = 220
	workspaceCardWidth    = 260
)

func (ui *Window) ensureWorkspaceNavButtons() {
	want := len(sshWorkspaceNavigation())
	if len(ui.workspaceNavButtons) != want {
		ui.workspaceNavButtons = make([]widget.Clickable, want)
	}
}

func (ui *Window) handleSSHWorkspace(gtx layout.Context) {
	if ui.handleTransferPanel(gtx) {
		return
	}
	ui.ensureWorkspaceNavButtons()
	ui.drainEditors(gtx, &ui.workspaceSearch)
	if ui.workspaceSearchClear.Clicked(gtx) {
		ui.workspaceSearch.SetText("")
		ui.rebuildSSHHostFilter()
		return
	}
	if ui.workspaceRefresh.Clicked(gtx) {
		switch ui.workspaceModule {
		case sshWorkspaceTunnels:
			ui.refreshSSHTunnels()
		case sshWorkspaceSnippets:
			ui.refreshSSHCommandSnippets()
		case sshWorkspaceKeys:
			ui.refreshSSHKeyIdentities()
		case sshWorkspaceFingerprints:
			ui.refreshSSHHostFingerprints()
		case sshWorkspaceHistory:
			ui.refreshSSHSessionHistory()
		default:
			ui.refreshSSHHosts()
		}
		return
	}
	for index, item := range sshWorkspaceNavigation() {
		if ui.workspaceNavButtons[index].Clicked(gtx) {
			ui.setSSHWorkspaceModule(item.ID)
			switch item.ID {
			case sshWorkspaceTunnels:
				ui.refreshSSHTunnels()
			case sshWorkspaceSnippets:
				ui.refreshSSHCommandSnippets()
			case sshWorkspaceKeys:
				ui.refreshSSHKeyIdentities()
			case sshWorkspaceFingerprints:
				ui.refreshSSHHostFingerprints()
			case sshWorkspaceHistory:
				ui.refreshSSHSessionHistory()
			}
			return
		}
	}
	if ui.localTerminal.Clicked(gtx) {
		ui.startLocalTerminalTab()
		return
	}
	if ui.workspaceModule == sshWorkspaceHosts {
		if ui.terminalAppearanceButton.Clicked(gtx) {
			ui.openTerminalAppearanceForm(ui.sshHostID)
			return
		}
		if ui.workspaceExportButton.Clicked(gtx) {
			ui.openSSHWorkspaceExportForm()
			return
		}
		if ui.workspaceImportButton.Clicked(gtx) {
			ui.requestSSHWorkspaceImport()
			return
		}
	}
	if ui.workspaceModule == sshWorkspaceTunnels {
		if ui.sshTunnelNew.Clicked(gtx) {
			ui.openSSHTunnelForm("")
			return
		}
		if ui.handleSSHTunnels(gtx) {
			return
		}
	}
	if ui.workspaceModule == sshWorkspaceSnippets {
		if ui.handleSSHCommandSnippets(gtx) {
			return
		}
	}
	if ui.workspaceModule == sshWorkspaceKeys {
		if ui.handleSSHKeyIdentities(gtx) {
			return
		}
	}
	if ui.workspaceModule == sshWorkspaceFingerprints {
		if ui.handleSSHHostFingerprints(gtx) {
			return
		}
	}
	ui.handleRemoteSSH(gtx)
}

func (ui *Window) startLocalTerminalTab() {
	tab := ui.openLocalTerminalTab()
	if tab == nil {
		return
	}
	ui.startLocalTerminalForTab(tab)
}

func (ui *Window) startLocalTerminalForTab(tab *sshTab) {
	if tab == nil || ui.sshTabs.get(tab.ID) == nil {
		return
	}
	tab.size = ui.terminalSize
	ui.model.Status = ui.text("Connecting")
	go func(tabID string, size image.Point) {
		command, args := localShellCommand()
		terminal, err := startLocalShell(command, args)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTab(tabID, err) })
			return
		}
		ui.queueSSHTabApply(func() { ui.attachSSHTab(tabID, nil, terminal) }, func() { _ = terminal.Close() })
	}(tab.ID, tab.size)
}

func (ui *Window) workspaceShell(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(cardGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.workspaceSidebar(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.workspaceMain(gtx)
		}),
	)
}

func (ui *Window) workspaceSidebar(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(workspaceSidebarWidth)
		gtx.Constraints.Max.X = gtx.Dp(workspaceSidebarWidth)
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, "s12ryt")
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							caption := material.Caption(ui.theme, ui.text("SSH workspace"))
							caption.Color = colorMuted
							return caption.Layout(gtx)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ui.ensureWorkspaceNavButtons()
					items := sshWorkspaceNavigation()
					children := make([]layout.FlexChild, 0, len(items))
					for index, item := range items {
						index, item := index, item
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.workspaceNavButton(gtx, &ui.workspaceNavButtons[index], item)
						}))
					}
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx, children...)
				}),
			)
		})
	})
}

func (ui *Window) workspaceNavButton(gtx layout.Context, click *widget.Clickable, item sshWorkspaceNavItem) layout.Dimensions {
	style := material.Button(ui.theme, click, ui.text(item.LabelSource))
	style.CornerRadius = unit.Dp(6)
	style.Inset = layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}
	if ui.workspaceModule == item.ID {
		style.Background = colorTeal
		style.Color = colorBackground
	} else {
		style.Background = colorSurface
		style.Color = colorMuted
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return style.Layout(gtx)
}

func (ui *Window) workspaceMain(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.workspaceToolbar(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if ui.workspaceModule == sshWorkspaceHosts && len(ui.sshTabs.tabs) > 0 {
				return ui.remoteSSHTabsView(gtx)
			}
			if ui.workspaceModule == sshWorkspaceHosts {
				return ui.sshHostHomeView(gtx)
			}
			if ui.workspaceModule == sshWorkspaceTunnels {
				return ui.sshTunnelPage(gtx)
			}
			if ui.workspaceModule == sshWorkspaceSnippets {
				return ui.sshCommandSnippetPage(gtx)
			}
			if ui.workspaceModule == sshWorkspaceKeys {
				return ui.sshKeyIdentityPage(gtx)
			}
			if ui.workspaceModule == sshWorkspaceFingerprints {
				return ui.sshHostFingerprintPage(gtx)
			}
			if ui.workspaceModule == sshWorkspaceHistory {
				return ui.sshSessionHistoryPage(gtx)
			}
			return ui.workspaceModuleUnavailable(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.transferPanel(gtx)
		}),
	)
}

func (ui *Window) workspaceToolbar(gtx layout.Context) layout.Dimensions {
	if ui.workspaceModule == sshWorkspaceTunnels {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H6(ui.theme, ui.text("Port forwarding"))
				title.Color = colorText
				return title.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.workspaceRefresh, "Refresh tunnels", false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.sshTunnelNew, "New tunnel", true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.terminalAppearanceButton, "Terminal appearance", false)
			}),
		)
	}
	if ui.workspaceModule == sshWorkspaceSnippets {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.fieldLabel(gtx, "Search command snippets")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.editorField(gtx, &ui.workspaceSearch, "Search command snippets")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.workspaceSearchClear, "Clear", false)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.workspaceRefresh, "Refresh", false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.sshSnippetNew, "New snippet", true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
			}),
		)
	}
	if ui.workspaceModule == sshWorkspaceKeys {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.fieldLabel(gtx, "Search key identities")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.editorField(gtx, &ui.workspaceSearch, "Search key identities")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.workspaceSearchClear, "Clear", false)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.workspaceRefresh, "Refresh keys", false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.sshKeyNew, "New key", true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
			}),
		)
	}
	if ui.workspaceModule == sshWorkspaceFingerprints {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.fieldLabel(gtx, "Search host fingerprints")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.editorField(gtx, &ui.workspaceSearch, "Search host fingerprints")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.workspaceSearchClear, "Clear", false)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.workspaceRefresh, "Refresh fingerprints", false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
			}),
		)
	}
	if ui.workspaceModule == sshWorkspaceHistory {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.fieldLabel(gtx, "Search session history")
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.editorField(gtx, &ui.workspaceSearch, "Search session history")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.workspaceSearchClear, "Clear", false)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.workspaceRefresh, "Refresh history", false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.fieldLabel(gtx, "Search hosts")
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.editorField(gtx, &ui.workspaceSearch, "Enter a host, IP address, or group")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.workspaceSearchClear, "Clear", false)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.workspaceRefresh, "Refresh", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshNew, "New host", true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.localTerminal, "Local terminal", true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.workspaceImportButton, "Import workspace", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.workspaceExportButton, "Export workspace", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.terminalAppearanceButton, "Terminal appearance", false)
		}),
	)
}

func (ui *Window) sshHostHomeView(gtx layout.Context) layout.Dimensions {
	ui.rebuildSSHHostFilterIfNeeded()
	return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := material.H5(ui.theme, ui.text("Hosts"))
						title.Color = colorText
						return title.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Body2(ui.theme, fmt.Sprintf("%d", len(ui.sshHostIndices))).Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.sshHostRecentSection(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.workspaceHomeSectionTitle(gtx, "Groups")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.sshHostGroups(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.sshHostCards(gtx)
			}),
		)
	})
}

func (ui *Window) sshHostRecentSection(gtx layout.Context) layout.Dimensions {
	if len(ui.sshRecentHostIndices) == 0 {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.workspaceHomeSectionTitle(gtx, "Recent connections")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			list := layout.List{Axis: layout.Horizontal}
			return list.Layout(gtx, len(ui.sshRecentHostIndices), func(gtx layout.Context, index int) layout.Dimensions {
				return ui.sshRecentHostCard(gtx, index)
			})
		}),
	)
}

func (ui *Window) sshRecentHostCard(gtx layout.Context, visibleIndex int) layout.Dimensions {
	if visibleIndex < 0 || visibleIndex >= len(ui.sshRecentHostIndices) {
		return layout.Dimensions{}
	}
	originalIndex := ui.sshRecentHostIndices[visibleIndex]
	if originalIndex < 0 || originalIndex >= len(ui.sshHosts) {
		return layout.Dimensions{}
	}
	host := ui.sshHosts[originalIndex]
	gtx.Constraints.Min.X = gtx.Dp(220)
	gtx.Constraints.Max.X = gtx.Dp(220)
	return layout.Inset{Right: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(rowGap), Bottom: unit.Dp(rowGap), Left: unit.Dp(rowGap), Right: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.sshRecentButtons[visibleIndex], host.Name, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(ui.theme, fmt.Sprintf("%s:%d", host.Host, host.Port))
						label.Color = colorMuted
						return label.Layout(gtx)
					}),
				)
			})
		})
	})
}

func (ui *Window) workspaceHomeSectionTitle(gtx layout.Context, source string) layout.Dimensions {
	label := material.Subtitle1(ui.theme, ui.text(source))
	label.Color = colorText
	return label.Layout(gtx)
}

func (ui *Window) sshHostGroups(gtx layout.Context) layout.Dimensions {
	groups := groupSSHHosts(filterSSHHosts(ui.sshHosts, ui.sshHostQuery))
	if len(groups) == 0 {
		return material.Body2(ui.theme, ui.text("No SSH hosts yet.")).Layout(gtx)
	}
	list := layout.List{Axis: layout.Horizontal}
	return list.Layout(gtx, len(groups), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(150)
			gtx.Constraints.Max.X = gtx.Dp(190)
			return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(rowGap), Bottom: unit.Dp(rowGap), Left: unit.Dp(rowGap), Right: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					group := groups[index]
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Body1(ui.theme, group.Name)
							label.Color = colorText
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Caption(ui.theme, fmt.Sprintf("%d %s", group.Count, ui.text("hosts")))
							label.Color = colorMuted
							return label.Layout(gtx)
						}),
					)
				})
			})
		})
	})
}

func (ui *Window) sshHostCards(gtx layout.Context) layout.Dimensions {
	ui.rebuildSSHHostFilterIfNeeded()
	if len(ui.sshHostIndices) == 0 {
		message := "No SSH hosts yet."
		if ui.sshHostQuery != "" {
			message = "No SSH hosts match this search."
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body1(ui.theme, ui.text(message))
			label.Color = colorMuted
			return label.Layout(gtx)
		})
	}
	ui.sshHostHomeList.Axis = layout.Vertical
	return ui.sshHostHomeList.Layout(gtx, len(ui.sshHostIndices), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshHostCard(gtx, index)
		})
	})
}

func (ui *Window) sshHostCard(gtx layout.Context, visibleIndex int) layout.Dimensions {
	if visibleIndex < 0 || visibleIndex >= len(ui.sshHostIndices) {
		return layout.Dimensions{}
	}
	originalIndex := ui.sshHostIndices[visibleIndex]
	if originalIndex < 0 || originalIndex >= len(ui.sshHosts) {
		return layout.Dimensions{}
	}
	host := ui.sshHosts[originalIndex]
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(cardGap)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									label := material.Subtitle1(ui.theme, host.Name)
									label.Color = colorText
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.hostStateBadge(gtx, host)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(ui.theme, fmt.Sprintf("%s@%s:%d", host.Username, host.Host, host.Port))
							label.Color = colorMuted
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.button(gtx, &ui.sshHostButtons[visibleIndex], "Connect", true)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.button(gtx, &ui.sshHostEditButtons[visibleIndex], "Edit", false)
								}),
							)
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{}
		}),
	)
}

func (ui *Window) hostStateBadge(gtx layout.Context, host remote.SSHHost) layout.Dimensions {
	labelText := "Enabled"
	color := colorTeal
	if !host.Enabled {
		labelText = "Disabled"
		color = colorDanger
	}
	label := material.Caption(ui.theme, ui.text(labelText))
	label.Color = color
	return label.Layout(gtx)
}

func (ui *Window) workspaceModuleUnavailable(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H5(ui.theme, ui.text(sshWorkspaceModuleTitle(ui.workspaceModule)))
					title.Color = colorText
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := material.Body1(ui.theme, ui.text("This workspace module is not available yet."))
					message.Color = colorMuted
					return message.Layout(gtx)
				}),
			)
		})
	})
}

func recentSSHHosts(hosts []remote.SSHHost, limit int) []remote.SSHHost {
	result := append([]remote.SSHHost(nil), hosts...)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].UpdatedAt > result[j].UpdatedAt
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (ui *Window) localTerminalConnect(ctx context.Context, tabID string) error {
	if ctx == nil {
		return errors.New("local terminal context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if ui.sshTabs.get(tabID) == nil {
		return errors.New("local terminal tab is not available")
	}
	return nil
}
