package gui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/remote"
)

func (ui *Window) handleSSHHostFingerprints(gtx layout.Context) bool {
	if ui == nil || ui.sshFingerprints == nil {
		return true
	}
	entries := filterSSHHostFingerprintEntries(ui.sshFingerprints.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHHostFingerprintButtons(entries)
	copyIndex := 0
	for index, entry := range entries {
		if ui.sshFingerprintManualBtns[index].Clicked(gtx) {
			ui.openManualSSHHostFingerprint(entry.Host.ID)
			return true
		}
		if ui.sshFingerprintClearBtns[index].Clicked(gtx) {
			ui.clearTrustedSSHHostFingerprint(entry.Host.ID)
			return true
		}
		for range entry.History {
			if ui.sshFingerprintCopyBtns[copyIndex].Clicked(gtx) {
				ui.copySSHHostFingerprint(gtx, ui.sshFingerprintCopyValues[copyIndex])
				return true
			}
			copyIndex++
		}
	}
	return true
}

func (ui *Window) copySSHHostFingerprint(gtx layout.Context, fingerprint string) bool {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "text/plain",
		Data: io.NopCloser(strings.NewReader(fingerprint)),
	})
	return true
}

func (ui *Window) sshHostFingerprintPage(gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.sshFingerprints == nil {
		return ui.sshHostFingerprintEmpty(gtx, "No host fingerprints yet.")
	}
	entries := filterSSHHostFingerprintEntries(ui.sshFingerprints.snapshot(), ui.workspaceSearch.Text())
	ui.syncSSHHostFingerprintButtons(entries)
	if len(entries) == 0 {
		message := "No host fingerprints yet."
		if strings.TrimSpace(ui.workspaceSearch.Text()) != "" {
			message = "No host fingerprints match this search."
		}
		return ui.sshHostFingerprintEmpty(gtx, message)
	}
	ui.sshFingerprintList.Axis = layout.Vertical
	return ui.sshFingerprintList.Layout(gtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshHostFingerprintCard(gtx, entries, index)
		})
	})
}

func (ui *Window) sshHostFingerprintEmpty(gtx layout.Context, source string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(ui.theme, ui.text(source))
		label.Color = colorMuted
		return label.Layout(gtx)
	})
}

func (ui *Window) sshHostFingerprintCard(gtx layout.Context, entries []sshHostFingerprintEntry, index int) layout.Dimensions {
	entry := entries[index]
	copyIndex := sshHostFingerprintCopyOffset(entries, index)
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									title := material.Subtitle1(ui.theme, entry.Host.Name)
									title.Color = colorText
									return title.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									address := material.Caption(ui.theme, fmt.Sprintf("%s:%d", entry.Host.Host, entry.Host.Port))
									address.Color = colorMuted
									return address.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshFingerprintManualBtns[index], "Trust manual fingerprint", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !sshHostFingerprintHasActive(entry) {
								return layout.Dimensions{}
							}
							return ui.actionButton(gtx, &ui.sshFingerprintClearBtns[index], "Clear trust", false, true)
						}),
					)
				}),
			}
			if len(entry.History) == 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(ui.theme, ui.text("No host fingerprints yet."))
					label.Color = colorMuted
					return label.Layout(gtx)
				}))
			} else {
				for historyIndex, fingerprint := range entry.History {
					historyIndex, fingerprint := historyIndex, fingerprint
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(rowGap)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.sshHostFingerprintHistoryRow(gtx, fingerprint, &ui.sshFingerprintCopyBtns[copyIndex+historyIndex])
						})
					}))
				}
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx, children...)
		})
	})
}

func (ui *Window) sshHostFingerprintHistoryRow(gtx layout.Context, fingerprint remote.SSHHostFingerprint, copyButton *widget.Clickable) layout.Dimensions {
	status := "Retired"
	statusColor := colorMuted
	if fingerprint.Active {
		status = "Current"
		statusColor = colorTeal
	}
	source := "TOFU"
	if fingerprint.Source == remote.SSHHostFingerprintManual {
		source = "Manual"
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, ui.text(status))
					label.Color = statusColor
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, ui.text(source))
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, fmt.Sprintf("%s: %s", ui.text("Algorithm"), fingerprint.Algorithm))
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, copyButton, "Copy fingerprint", false)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Label(ui.theme, unit.Sp(13), fingerprint.Fingerprint)
			label.Font.Typeface = monoTypeface
			label.Color = colorText
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			metadata := fmt.Sprintf("%s: %s", ui.text("Observed"), formatFingerprintTimestamp(fingerprint.ObservedAt))
			if fingerprint.RetiredAt != nil {
				metadata += fmt.Sprintf("  %s: %s", ui.text("Retired at"), formatFingerprintTimestamp(*fingerprint.RetiredAt))
			}
			label := material.Caption(ui.theme, metadata)
			label.Color = colorMuted
			return label.Layout(gtx)
		}),
	)
}

func sshHostFingerprintCopyOffset(entries []sshHostFingerprintEntry, hostIndex int) int {
	offset := 0
	for index := 0; index < hostIndex && index < len(entries); index++ {
		offset += len(entries[index].History)
	}
	return offset
}

func sshHostFingerprintHasActive(entry sshHostFingerprintEntry) bool {
	for _, fingerprint := range entry.History {
		if fingerprint.Active {
			return true
		}
	}
	return strings.TrimSpace(entry.Host.TrustedFingerprint) != ""
}

func formatFingerprintTimestamp(value int64) string {
	if value <= 0 {
		return "-"
	}
	stamp := time.Unix(value, 0)
	if value > 1_000_000_000_000 {
		stamp = time.UnixMilli(value)
	}
	return stamp.Local().Format("2006-01-02 15:04:05")
}
