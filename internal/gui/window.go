package gui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"s12ryt-ssh/internal/i18n"
	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	colorBackground = color.NRGBA{R: 13, G: 18, B: 22, A: 255}
	colorSurface    = color.NRGBA{R: 23, G: 31, B: 37, A: 255}
	colorSurface2   = color.NRGBA{R: 31, G: 42, B: 48, A: 255}
	colorTeal       = color.NRGBA{R: 55, G: 220, B: 177, A: 255}
	colorText       = color.NRGBA{R: 232, G: 244, B: 241, A: 255}
	colorMuted      = color.NRGBA{R: 157, G: 178, B: 174, A: 255}
	colorDanger     = color.NRGBA{R: 255, G: 116, B: 124, A: 255}
)

// Window is the Gio presentation layer for the application.
type Window struct {
	model              *Model
	window             *app.Window
	theme              *material.Theme
	ops                op.Ops
	remoteList         layout.List
	terminalList       layout.List
	storageOutputList  layout.List
	databaseOutputList layout.List
	remoteObjectList   layout.List
	language           i18n.Language
	preferencesPath    string
	languageButton     widget.Clickable
	reveals            map[*widget.Editor]*editorReveal

	sshTab      widget.Clickable
	storageTab  widget.Clickable
	databaseTab widget.Clickable
	logout      widget.Clickable

	remoteURL      widget.Editor
	remoteUsername widget.Editor
	remotePassword widget.Editor
	remoteLogin    widget.Clickable
	remoteRestore  widget.Clickable
	remoteRefresh  widget.Clickable

	remoteResources       []remote.Resource
	remoteResourceButtons []widget.Clickable
	remoteIndex           int

	sshNew         widget.Clickable
	sshSave        widget.Clickable
	sshConnect     widget.Clickable
	sshClose       widget.Clickable
	sshSend        widget.Clickable
	sshDelete      widget.Clickable
	sshName        widget.Editor
	sshHost        widget.Editor
	sshPort        widget.Editor
	sshUser        widget.Editor
	sshPassword    widget.Editor
	sshPrivateKey  widget.Editor
	sshKeyPass     widget.Editor
	sshFingerprint widget.Editor

	sshHosts       []remote.SSHHost
	sshHostButtons []widget.Clickable
	sshHostIndex   int
	sshHostID      string
	terminalInput  widget.Editor
	terminalText   string
	terminal       ptyTerminal
	ssh            *sshclient.Client
	terminalCtx    context.Context
	terminalCancel context.CancelFunc
	terminalMu     sync.RWMutex
	terminalSize   image.Point

	confirm             confirmation
	confirmAcceptBtn    widget.Clickable
	confirmCancelBtn    widget.Clickable
	confirmScrim        widget.Clickable
	remoteObjects       []remote.S3Object
	remoteObjectButtons []widget.Clickable

	storageRefresh  widget.Clickable
	storageUpload   widget.Clickable
	storageDownload widget.Clickable
	storageDelete   widget.Clickable
	storagePrefix   widget.Editor
	storageKey      widget.Editor
	storagePath     widget.Editor
	storageData     widget.Editor
	storageText     string

	databaseTables widget.Clickable
	databaseQuery  widget.Clickable
	databaseExec   widget.Clickable
	databaseSQL    widget.Editor
	databaseText   string

	busy   bool
	events chan asyncEvent
}

type asyncEvent struct {
	apply       func()
	applyAlways bool
	err         error
	status      string
}

// NewWindow creates a Gio application window controller.
func NewWindow(remoteService *remote.Service) *Window {
	return NewWindowWithPreferences(remoteService, "")
}

// NewWindowWithPreferences creates a window and loads its non-sensitive language preference.
func NewWindowWithPreferences(remoteService *remote.Service, preferencesPath string) *Window {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Bg = colorBackground
	th.Palette.Fg = colorText
	th.Palette.ContrastBg = colorTeal
	th.Palette.ContrastFg = colorBackground
	th.TextSize = unit.Sp(16)
	language := i18n.English
	if preferencesPath != "" {
		if prefs, err := i18n.LoadPreferences(preferencesPath); err == nil {
			language = prefs.Language
		}
	}
	ui := &Window{
		model:           NewModel(remoteService),
		theme:           th,
		remoteIndex:     -1,
		events:          make(chan asyncEvent, 8),
		language:        language,
		preferencesPath: preferencesPath,
	}
	if remoteService != nil {
		if prefs, err := remoteService.Preferences(); err == nil {
			ui.remoteURL.SetText(prefs.BaseURL)
			ui.remoteUsername.SetText(prefs.Username)
		}
	}
	return ui
}

func (ui *Window) text(source string) string { return i18n.Text(ui.language, source) }

func (ui *Window) toggleLanguage() error {
	if ui.language == i18n.TraditionalChinese {
		ui.language = i18n.English
	} else {
		ui.language = i18n.TraditionalChinese
	}
	if ui.preferencesPath != "" {
		if err := i18n.SavePreferences(ui.preferencesPath, i18n.Preferences{Language: ui.language}); err != nil {
			ui.model.Error = i18n.T(i18n.English, i18n.KeyPreferenceSave) + err.Error()
			return err
		}
	}
	return nil
}

// Run attaches the controller to a Gio window and processes its event loop.
func (ui *Window) Run(window *app.Window) error {
	ui.window = window
	window.Option(app.Title("s12ryt SSH"), app.Size(unit.Dp(1180), unit.Dp(760)), app.MinSize(unit.Dp(900), unit.Dp(620)))
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return ui.Close()
		case app.FrameEvent:
			gtx := app.NewContext(&ui.ops, e)
			ui.pump()
			ui.handle(gtx)
			ui.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// Model returns the current testable state model.
func (ui *Window) Model() *Model { return ui.model }

// Close releases live SSH resources and the remote session.
func (ui *Window) Close() error {
	if ui == nil {
		return nil
	}
	ui.closeSSH()
	if ui.model.RemoteSession != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		remoteErr := ui.model.LogoutRemote(ctx)
		cancel()
		return remoteErr
	}
	return nil
}

func (ui *Window) closeSSH() {
	if ui.terminalCancel != nil {
		ui.terminalCancel()
		ui.terminalCancel = nil
	}
	ui.terminalCtx = nil
	if ui.terminal != nil {
		_ = ui.terminal.Close()
		ui.terminal = nil
	}
	if ui.ssh != nil {
		_ = ui.ssh.Close()
		ui.ssh = nil
	}
}

func (ui *Window) pump() {
	for {
		select {
		case event := <-ui.events:
			ui.busy = false
			if event.apply != nil && (event.err == nil || event.applyAlways) {
				event.apply()
			}
			if event.err != nil {
				ui.model.Error = event.err.Error()
				ui.model.Status = "Operation failed."
			} else {
				ui.model.Error = ""
				if event.status != "" {
					ui.model.Status = event.status
				}
			}
		default:
			return
		}
	}
}

func (ui *Window) asyncAlways(status string, work func(context.Context) (func(), error)) {
	if ui.busy {
		return
	}
	ui.busy = true
	ui.model.Status = status
	ui.model.Error = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		apply, err := work(ctx)
		ui.events <- asyncEvent{apply: apply, applyAlways: true, err: err, status: "Ready."}
		if ui.window != nil {
			ui.window.Invalidate()
		}
	}()
}

func (ui *Window) async(status string, work func(context.Context) (func(), error)) {
	if ui.busy {
		return
	}
	ui.busy = true
	ui.model.Status = status
	ui.model.Error = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		apply, err := work(ctx)
		ui.events <- asyncEvent{apply: apply, err: err, status: "Ready."}
		if ui.window != nil {
			ui.window.Invalidate()
		}
	}()
}

func (ui *Window) handle(gtx layout.Context) {
	if ui.languageButton.Clicked(gtx) {
		_ = ui.toggleLanguage()
	}
	if ui.confirm.active {
		if ui.confirmScrim.Clicked(gtx) || ui.confirmCancelBtn.Clicked(gtx) {
			ui.confirm.cancel()
		}
		if ui.confirmAcceptBtn.Clicked(gtx) {
			ui.confirm.accept()
		}
		return
	}
	switch ui.model.Screen {
	case ScreenRemoteLogin:
		ui.handleRemoteLogin(gtx)
	case ScreenRemoteWorkspace:
		ui.handleRemoteWorkspace(gtx)
	}
}

// drainEditors consumes pending editor events and reports whether any of them
// was a submit. Gio v0.10 delivers editor events through Update; draining them
// every frame keeps stale presses from leaking into later actions.
func (ui *Window) drainEditors(gtx layout.Context, editors ...*widget.Editor) bool {
	submitted := false
	for _, editor := range editors {
		events := make([]widget.EditorEvent, 0, 4)
		for {
			event, ok := editor.Update(gtx)
			if !ok {
				break
			}
			events = append(events, event)
		}
		if consumeSubmit(events) {
			submitted = true
		}
	}
	return submitted
}

// requestConfirm opens the destructive-action modal unless work is in flight.
func (ui *Window) requestConfirm(title, message string, action func()) {
	if ui.busy {
		return
	}
	ui.confirm.request(title, message, action)
}

func (ui *Window) layout(gtx layout.Context) {
	paint.Fill(gtx.Ops, colorBackground)
	layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24), Left: unit.Dp(28), Right: unit.Dp(28)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(16)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.header(gtx) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.content(gtx) }),
		)
	})
	if ui.confirm.active {
		ui.confirmModal(gtx)
	}
}

// confirmModal overlays a dimmed scrim with the destructive-action dialog.
// Clicking the scrim cancels, matching the explicit Cancel button.
func (ui *Window) confirmModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 110}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.confirmScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.confirmDialog(gtx)
		}),
	)
}

func (ui *Window) confirmDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(440))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(18), Left: unit.Dp(22), Right: unit.Dp(22)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(14)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(ui.theme, ui.text(ui.confirm.title))
					title.Color = colorDanger
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := material.Body1(ui.theme, ui.text(ui.confirm.message))
					message.Color = colorText
					return message.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.confirmCancelBtn, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.confirmAcceptBtn, "Confirm", true, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) header(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(12)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H4(ui.theme, "s12ryt SSH").Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.Body2(ui.theme, ui.text("Secure remote workspace")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.languageButton, i18n.T(ui.language, i18n.KeyLanguageToggle), false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if ui.model.Screen == ScreenRemoteWorkspace {
				return ui.button(gtx, &ui.logout, ui.text("Log out"), false)
			}
			return layout.Dimensions{}
		}),
	)
}

func (ui *Window) content(gtx layout.Context) layout.Dimensions {
	switch ui.model.Screen {
	case ScreenRemoteLogin:
		return ui.remoteLoginView(gtx)
	case ScreenRemoteWorkspace:
		return ui.remoteWorkspaceView(gtx)
	default:
		return layout.Dimensions{}
	}
}

func (ui *Window) status(gtx layout.Context) layout.Dimensions {
	if ui.model.Error != "" {
		style := material.Body2(ui.theme, ui.text(ui.model.Error))
		style.Color = colorDanger
		return style.Layout(gtx)
	}
	if ui.busy {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(8)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				loader := material.Loader(ui.theme)
				loader.Color = colorTeal
				return loader.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				style := material.Body2(ui.theme, ui.text("Working...")+" "+ui.text(ui.model.Status))
				style.Color = colorMuted
				return style.Layout(gtx)
			}),
		)
	}
	style := material.Body2(ui.theme, ui.text(ui.model.Status))
	style.Color = colorMuted
	return style.Layout(gtx)
}

func (ui *Window) remoteResourceIndices(tab Tab) []int {
	indices := make([]int, 0, len(ui.remoteResources))
	for index, resource := range ui.remoteResources {
		if !resource.Enabled {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(resource.Kind))
		if tab == TabStorage && kind == "s3" {
			indices = append(indices, index)
		}
		if tab == TabDatabase && (kind == "mysql" || kind == "postgres" || kind == "postgresql") {
			indices = append(indices, index)
		}
	}
	return indices
}

func (ui *Window) remoteAllows(operation remote.Operation) bool {
	if ui.remoteIndex < 0 || ui.remoteIndex >= len(ui.remoteResources) {
		return false
	}
	return ui.remoteResources[ui.remoteIndex].Enabled && ui.remoteResources[ui.remoteIndex].Allows(operation)
}

func (ui *Window) secretLabel(gtx layout.Context, text string) layout.Dimensions {
	style := material.Body1(ui.theme, text)
	style.Color = colorTeal
	style.WrapPolicy = 0
	return style.Layout(gtx)
}

func (ui *Window) field(gtx layout.Context, editor *widget.Editor, hint string, singleLine, password bool) layout.Dimensions {
	if editor == nil {
		return layout.Dimensions{}
	}
	editor.SingleLine = singleLine
	editor.Submit = singleLine
	if !password {
		editor.Mask = 0
		return ui.editorField(gtx, editor, hint)
	}
	reveal := ui.revealFor(editor)
	if reveal.button.Clicked(gtx) {
		reveal.state.toggle()
	}
	editor.Mask = reveal.state.mask()
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(6)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.editorField(gtx, editor, hint)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &reveal.button, revealLabel(reveal.state.shown), false)
		}),
	)
}

func (ui *Window) editorField(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	style := material.Editor(ui.theme, editor, ui.text(hint))
	style.Color = colorText
	style.HintColor = colorMuted
	return style.Layout(gtx)
}

func (ui *Window) revealFor(editor *widget.Editor) *editorReveal {
	if ui.reveals == nil {
		ui.reveals = make(map[*widget.Editor]*editorReveal)
	}
	reveal, ok := ui.reveals[editor]
	if !ok {
		reveal = &editorReveal{}
		ui.reveals[editor] = reveal
	}
	return reveal
}

func (ui *Window) editorRow(gtx layout.Context, left string, leftEditor *widget.Editor, right string, rightEditor *widget.Editor) layout.Dimensions {
	leftField := func(gtx layout.Context) layout.Dimensions {
		return ui.field(gtx, leftEditor, left, true, isSecretHint(left))
	}
	rightField := func(gtx layout.Context) layout.Dimensions {
		if rightEditor == nil {
			return layout.Dimensions{}
		}
		return ui.field(gtx, rightEditor, right, true, isSecretHint(right))
	}
	if useStackedRow(int(float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp)) {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(6)}.Layout(gtx,
			layout.Rigid(leftField),
			layout.Rigid(rightField),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(10)}.Layout(gtx,
		layout.Flexed(1, leftField),
		layout.Flexed(1, rightField),
	)
}

func (ui *Window) button(gtx layout.Context, click *widget.Clickable, text string, primary bool) layout.Dimensions {
	style := material.Button(ui.theme, click, ui.text(text))
	style.CornerRadius = unit.Dp(6)
	style.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}
	if primary {
		style.Background = colorTeal
		style.Color = colorBackground
	} else {
		style.Background = colorSurface2
		style.Color = colorText
	}
	return style.Layout(gtx)
}

// actionButton renders an operation button. Busy work dims it so blocked
// actions never look clickable, and destructive actions render in danger red.
func (ui *Window) actionButton(gtx layout.Context, click *widget.Clickable, text string, primary, danger bool) layout.Dimensions {
	style := material.Button(ui.theme, click, ui.text(text))
	style.CornerRadius = unit.Dp(6)
	style.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}
	switch {
	case ui.busy:
		background, foreground := buttonColors(true, false)
		style.Background, style.Color = background, foreground
	case danger:
		background, foreground := buttonColors(false, true)
		style.Background, style.Color = background, foreground
	case primary:
		style.Background, style.Color = colorTeal, colorBackground
	default:
		background, foreground := buttonColors(false, false)
		style.Background, style.Color = background, foreground
	}
	return style.Layout(gtx)
}

// outputList renders scrollable operation output that sticks to the bottom as
// new content arrives, bounded to the most recent lines.
func (ui *Window) outputList(gtx layout.Context, list *layout.List, content, hint string, mono bool) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if content == "" {
			muted := material.Body2(ui.theme, ui.text(hint))
			muted.Color = colorMuted
			return muted.Layout(gtx)
		}
		if !list.Position.BeforeEnd {
			list.Position.Offset = math.MaxInt32
		}
		return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			style := material.Body1(ui.theme, tailLines(content, outputMaxLines))
			style.Color = colorText
			if mono {
				style.Font.Typeface = monoTypeface
			}
			return style.Layout(gtx)
		})
	})
}

func (ui *Window) ensureRemoteObjectButtons() {
	if len(ui.remoteObjectButtons) != len(ui.remoteObjects) {
		ui.remoteObjectButtons = make([]widget.Clickable, len(ui.remoteObjects))
	}
}

func (ui *Window) surface(gtx layout.Context, child layout.Widget) layout.Dimensions {
	defer clip.UniformRRect(image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), 8).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorSurface)
	return child(gtx)
}

func (ui *Window) appendTerminal(text string) {
	ui.terminalMu.Lock()
	ui.terminalText = appendTerminalFilter(ui.terminalText, text, terminalMaxRunes)
	ui.terminalMu.Unlock()
}
func (ui *Window) terminalSnapshot() string {
	ui.terminalMu.RLock()
	defer ui.terminalMu.RUnlock()
	return ui.terminalText
}
func (ui *Window) readTerminal(terminal ptyTerminal) {
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := terminal.Read(buf)
			if n > 0 {
				ui.appendTerminal(string(buf[:n]))
				if ui.window != nil {
					ui.window.Invalidate()
				}
			}
			if err != nil {
				return
			}
		}
	}()
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

func formatRemoteRows(result remote.SQLQueryResult) string {
	if len(result.Rows) == 0 {
		return "No rows returned."
	}
	var b strings.Builder
	for rowIndex, row := range result.Rows {
		fmt.Fprintf(&b, "%d: ", rowIndex+1)
		for columnIndex, column := range result.Columns {
			if columnIndex > 0 {
				b.WriteString(" | ")
			}
			var value any
			if columnIndex < len(row) {
				value = row[columnIndex]
			}
			fmt.Fprintf(&b, "%s=%v", column, value)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (ui *Window) downloadedTo(path string) string {
	if path == "" {
		return ui.text(" (preview available in output)")
	}
	return ui.text(" to ") + path
}
