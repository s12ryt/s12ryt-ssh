package gui

import (
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/remote"
)

func (ui *Window) sshSessionHistoryPage(gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.sshHistory == nil {
		return ui.sshSessionHistoryEmpty(gtx, "No session history yet.")
	}
	records := filterSSHSessionHistory(ui.sshHistory.snapshot(), ui.workspaceSearch.Text())
	if len(records) == 0 {
		message := "No session history yet."
		if strings.TrimSpace(ui.workspaceSearch.Text()) != "" {
			message = "No session history matches this search."
		}
		return ui.sshSessionHistoryEmpty(gtx, message)
	}
	ui.sshHistoryList.Axis = layout.Vertical
	return ui.sshHistoryList.Layout(gtx, len(records), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshSessionHistoryCard(gtx, records[index])
		})
	})
}

func (ui *Window) sshSessionHistoryEmpty(gtx layout.Context, source string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(ui.theme, ui.text(source))
		label.Color = colorMuted
		return label.Layout(gtx)
	})
}

func (ui *Window) sshSessionHistoryCard(gtx layout.Context, record remote.SSHSessionHistory) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding),
			Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.Subtitle1(ui.theme, sshSessionHistoryHostName(record))
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							status := material.Caption(ui.theme, ui.text(sshSessionHistoryStatusSource(record.Status)))
							status.Color = sshSessionHistoryStatusColor(record.Status)
							return status.Layout(gtx)
						}),
					)
				}),
			}
			for _, detail := range sshSessionHistoryDetails(record) {
				detail := detail
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(ui.theme, ui.text(detail.Source)+": "+detail.Value)
					label.Color = colorMuted
					if detail.Danger {
						label.Color = colorDanger
					}
					return label.Layout(gtx)
				}))
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx, children...)
		})
	})
}

func sshSessionHistoryHostName(record remote.SSHSessionHistory) string {
	if name := strings.TrimSpace(record.HostName); name != "" {
		return name
	}
	if hostID := strings.TrimSpace(record.HostID); hostID != "" {
		return hostID
	}
	return "-"
}

func sshSessionHistoryStatusColor(status remote.SSHSessionHistoryStatus) color.NRGBA {
	switch status {
	case remote.SSHSessionConnected:
		return colorTeal
	case remote.SSHSessionFailed:
		return colorDanger
	case remote.SSHSessionClosed:
		return colorMuted
	default:
		return colorText
	}
}
