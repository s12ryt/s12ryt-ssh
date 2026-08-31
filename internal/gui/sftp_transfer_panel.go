package gui

import (
	"fmt"
	"path"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *Window) handleTransferPanel(gtx layout.Context) bool {
	if ui == nil || ui.transfers == nil {
		return false
	}
	items := ui.transfers.items()
	ui.syncTransferActionButtons(items)
	if len(items) == 0 {
		return false
	}
	if ui.transferToggle.Clicked(gtx) {
		ui.transferPanelOpen = !ui.transferPanelOpen
		return true
	}
	for index := range items {
		if !ui.transferActionButtons[index].Clicked(gtx) {
			continue
		}
		switch items[index].Status {
		case transferRunning, transferQueued:
			ui.transfers.pause(items[index].ID)
		case transferPaused:
			ui.transfers.resume(items[index].ID)
		case transferFailed:
			ui.transfers.retry(items[index].ID)
		}
		return true
	}
	return false
}

func (ui *Window) syncTransferActionButtons(items []transferItem) {
	if ui == nil {
		return
	}
	matched := len(items) == len(ui.transferActionIDs)
	if matched {
		for index := range items {
			if items[index].ID != ui.transferActionIDs[index] {
				matched = false
				break
			}
		}
	}
	if matched {
		return
	}
	ui.transferActionButtons = make([]widget.Clickable, len(items))
	ui.transferActionIDs = make([]string, len(items))
	for index := range items {
		ui.transferActionIDs[index] = items[index].ID
	}
}

func (ui *Window) transferPanel(gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.transfers == nil {
		return layout.Dimensions{}
	}
	items := ui.transfers.items()
	if len(items) == 0 {
		return layout.Dimensions{}
	}
	ui.syncTransferActionButtons(items)
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 12, Bottom: 12, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.transferPanelHeader(gtx, len(items))
				}),
			}
			if ui.transferPanelOpen {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(190))
					ui.transferList.Axis = layout.Vertical
					return ui.transferList.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.transferPanelItem(gtx, items[index], &ui.transferActionButtons[index])
						})
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx, children...)
		})
	})
}

func (ui *Window) transferPanelHeader(gtx layout.Context, count int) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.H6(ui.theme, ui.text("Transfers"))
			title.Color = colorText
			return title.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			caption := material.Caption(ui.theme, fmt.Sprintf("%d", count))
			caption.Color = colorMuted
			return caption.Layout(gtx)
		}),
		layout.Flexed(1, func(layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			source := "Show transfers"
			if ui.transferPanelOpen {
				source = "Hide transfers"
			}
			return ui.button(gtx, &ui.transferToggle, source, false)
		}),
	)
}

func (ui *Window) transferPanelItem(gtx layout.Context, item transferItem, action *widget.Clickable) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(46)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			direction := material.Caption(ui.theme, ui.text(transferDirectionSource(item.Direction)))
			direction.Color = colorTeal
			return direction.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					name := material.Body1(ui.theme, transferDisplayName(item))
					name.Color = colorText
					name.MaxLines = 1
					return name.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					detail := transferProgressText(item)
					if item.Error != "" {
						detail += " - " + item.Error
					}
					caption := material.Caption(ui.theme, detail)
					caption.Color = colorMuted
					caption.MaxLines = 1
					return caption.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			status := material.Caption(ui.theme, ui.text(string(item.Status)))
			status.Color = colorMuted
			if item.Status == transferCompleted {
				status.Color = colorTeal
			} else if item.Status == transferFailed {
				status.Color = colorDanger
			}
			return status.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			source := transferActionSource(item.Status)
			if source == "" || action == nil {
				return layout.Dimensions{}
			}
			return ui.button(gtx, action, source, false)
		}),
	)
}

func transferDirectionSource(direction transferDirection) string {
	if direction == transferDownload {
		return "Download"
	}
	return "Upload"
}

func transferDisplayName(item transferItem) string {
	if item.Direction == transferDownload {
		if name := path.Base(item.Source); name != "." && name != "/" {
			return name
		}
	}
	if name := filepath.Base(item.Source); name != "." && name != string(filepath.Separator) {
		return name
	}
	return item.Source
}

func transferProgressText(item transferItem) string {
	percentage := int64(0)
	if item.Size > 0 {
		percentage = item.Transferred * 100 / item.Size
	} else if item.Status == transferCompleted {
		percentage = 100
	}
	text := fmt.Sprintf("%d / %d bytes (%d%%)", item.Transferred, item.Size, percentage)
	metrics := calculateTransferMetrics(item)
	if metrics.BytesPerSecond > 0 {
		text += fmt.Sprintf(" %.1f B/s", metrics.BytesPerSecond)
	}
	if metrics.HasETA {
		text += fmt.Sprintf(" ETA %ds", metrics.RemainingSeconds)
	}
	return text
}
