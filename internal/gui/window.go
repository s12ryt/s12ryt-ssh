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
	colorEdge       = color.NRGBA{R: 38, G: 52, B: 60, A: 255}
	colorTeal       = color.NRGBA{R: 55, G: 220, B: 177, A: 255}
	colorText       = color.NRGBA{R: 232, G: 244, B: 241, A: 255}
	colorMuted      = color.NRGBA{R: 157, G: 178, B: 174, A: 255}
	colorDanger     = color.NRGBA{R: 255, G: 116, B: 124, A: 255}
)

// Design tokens keep spacing and sizing consistent across every view. The
// rhythm sticks to a 4dp base scale: 4 / 8 / 12 / 16 / 20 / 24 / 32.
const (
	labelTextSize       = 12
	fieldGap            = 4
	rowGap              = 8
	sectionGap          = 12
	cardGap             = 16
	cardPadding         = 20
	cardPaddingLoose    = 28
	pagePadding         = 24
	loginCardWidth      = 440
	surfaceCornerRadius = 10
	privateKeyMinHeight = 88
)

// Window is the Gio presentation layer for the application.
type Window struct {
	model                               *Model
	window                              *app.Window
	theme                               *material.Theme
	terminalAppearance                  terminalAppearance
	terminalAppearanceOpen              bool
	terminalAppearanceForm              terminalAppearanceFormValues
	terminalAppearanceList              layout.List
	terminalAppearanceFont              widget.Editor
	terminalAppearanceFontSize          widget.Editor
	terminalAppearanceForeground        widget.Editor
	terminalAppearanceBackground        widget.Editor
	terminalAppearanceUseAccountDefault widget.Bool
	terminalAppearanceClose             widget.Clickable
	terminalAppearanceCancel            widget.Clickable
	terminalAppearanceSave              widget.Clickable
	terminalAppearanceScrim             widget.Clickable
	ops                                 op.Ops
	remoteList                          layout.List
	sshFormList                         layout.List
	terminalList                        layout.List
	terminalTabList                     layout.List
	sshHostStripList                    layout.List
	sshHostHomeList                     layout.List
	sshTunnelList                       layout.List
	sshTunnelFormList                   layout.List
	language                            i18n.Language
	preferencesPath                     string
	languageButton                      widget.Clickable
	reveals                             map[*widget.Editor]*editorReveal

	logout widget.Clickable

	remoteURL                widget.Editor
	remoteUsername           widget.Editor
	remotePassword           widget.Editor
	remoteRememberPassword   widget.Bool
	remoteAutoLoginPending   bool
	remoteAutoLoginStarted   bool
	remoteAutoLoginInFlight  bool
	remoteLogin              widget.Clickable
	remoteRestore            widget.Clickable
	workspaceNavButtons      []widget.Clickable
	workspaceSearch          widget.Editor
	workspaceSearchClear     widget.Clickable
	workspaceRefresh         widget.Clickable
	localTerminal            widget.Clickable
	terminalAppearanceButton widget.Clickable
	workspaceModule          sshWorkspaceModule

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

	sshHosts                       []remote.SSHHost
	sshHostIndices                 []int
	sshHostQuery                   string
	sshHostButtons                 []widget.Clickable
	sshHostEditButtons             []widget.Clickable
	sshRecentHostIndices           []int
	sshRecentButtons               []widget.Clickable
	sshTunnelNew                   widget.Clickable
	sshTunnelActionButtons         []widget.Clickable
	sshTunnelActionIDs             []string
	sshTunnelEditButtons           []widget.Clickable
	sshTunnelDeleteButtons         []widget.Clickable
	sshHostIndex                   int
	sshHostID                      string
	sshTabs                        sshTabStore
	sshTunnels                     *sshTunnelStore
	sshPool                        *sshConnectionPool
	sshTransportFactory            sshTransportFactory
	transfers                      *transferManager
	sftpFiles                      sftpFileDialog
	sftpFileDialogBusy             bool
	workspaceFiles                 workspaceFileDialog
	workspaceFileDialogBusy        bool
	workspaceImportPath            string
	workspaceExportPath            string
	workspaceExport                sshWorkspaceExportState
	workspaceImport                *sshWorkspaceImportState
	workspaceImportPassword        string
	workspaceExportButton          widget.Clickable
	workspaceImportButton          widget.Clickable
	workspaceExportOpen            bool
	workspaceExportIncludeSecrets  widget.Bool
	workspaceExportPassword        widget.Editor
	workspaceExportClose           widget.Clickable
	workspaceExportCancel          widget.Clickable
	workspaceExportSubmit          widget.Clickable
	workspaceExportScrim           widget.Clickable
	workspaceImportOpen            bool
	workspaceImportPasswordEditor  widget.Editor
	workspaceImportList            layout.List
	workspaceImportClose           widget.Clickable
	workspaceImportCancel          widget.Clickable
	workspaceImportPreview         widget.Clickable
	workspaceImportApply           widget.Clickable
	workspaceImportScrim           widget.Clickable
	workspaceImportConflictKeys    []string
	workspaceImportConflictButtons [][3]widget.Clickable
	sftpUploadConflictOpen         bool
	sftpUploadConflicts            []sftpUploadConflict
	sftpUploadOverwrite            widget.Clickable
	sftpUploadSkip                 widget.Clickable
	sftpUploadKeepBoth             widget.Clickable
	sftpUploadConflictScrim        widget.Clickable
	transferPanelOpen              bool
	transferToggle                 widget.Clickable
	transferList                   layout.List
	transferActionButtons          []widget.Clickable
	transferActionIDs              []string
	sshFormOpen                    bool
	sshFormOriginal                sshFormValues
	sshFormCloseButton             widget.Clickable
	sshFormCancelButton            widget.Clickable
	sshFormScrim                   widget.Clickable
	sshTabActionList               layout.List
	sshTabActionButtons            []widget.Clickable
	sshTabDrag                     sshTabDragState
	sshTabRenameOpen               bool
	sshTabRenameID                 string
	sshTabRenameEditor             widget.Editor
	sshTabRenameClose              widget.Clickable
	sshTabRenameCancel             widget.Clickable
	sshTabRenameSave               widget.Clickable
	sshTabRenameScrim              widget.Clickable
	sftpOperationOpen              bool
	sftpOperationAction            string
	sftpOperationTabID             string
	sftpOperationFirst             widget.Editor
	sftpOperationSecond            widget.Editor
	sftpOperationClose             widget.Clickable
	sftpOperationCancel            widget.Clickable
	sftpOperationSave              widget.Clickable
	sftpOperationScrim             widget.Clickable
	sshTunnelFormOpen              bool
	sshTunnelFormID                string
	sshTunnelForm                  sshTunnelFormValues
	sshTunnelName                  widget.Editor
	sshTunnelHost                  widget.Editor
	sshTunnelTypeButtons           [3]widget.Clickable
	sshTunnelListenHost            widget.Editor
	sshTunnelListenPort            widget.Editor
	sshTunnelTargetHost            widget.Editor
	sshTunnelTargetPort            widget.Editor
	sshTunnelEnabled               widget.Bool
	sshTunnelAutoStart             widget.Bool
	sshTunnelFormClose             widget.Clickable
	sshTunnelFormCancel            widget.Clickable
	sshTunnelFormSave              widget.Clickable
	sshTunnelFormDelete            widget.Clickable
	sshTunnelFormScrim             widget.Clickable
	sshSnippets                    *sshCommandSnippetStore
	sshSnippetList                 layout.List
	sshSnippetVisibleIDs           []string
	sshSnippetExecuteBtns          []widget.Clickable
	sshSnippetNew                  widget.Clickable
	sshSnippetEditBtns             []widget.Clickable
	sshSnippetDeleteBtns           []widget.Clickable
	sshSnippetExecutionOpen        bool
	sshSnippetExecutionID          string
	sshSnippetVariableNames        []string
	sshSnippetVariableEditors      []widget.Editor
	sshSnippetExecutionList        layout.List
	sshSnippetExecutionClose       widget.Clickable
	sshSnippetExecutionCancel      widget.Clickable
	sshSnippetExecutionRun         widget.Clickable
	sshSnippetExecutionScrim       widget.Clickable
	sshSnippetFormOpen             bool
	sshSnippetFormID               string
	sshSnippetForm                 sshCommandSnippetFormValues
	sshSnippetFormList             layout.List
	sshSnippetName                 widget.Editor
	sshSnippetCommand              widget.Editor
	sshSnippetVariables            widget.Editor
	sshSnippetSecrets              widget.Editor
	sshSnippetSavedSecretNames     string
	sshSnippetClearSecrets         widget.Bool
	sshSnippetEnabled              widget.Bool
	sshSnippetFormClose            widget.Clickable
	sshSnippetFormCancel           widget.Clickable
	sshSnippetFormSave             widget.Clickable
	sshSnippetFormDelete           widget.Clickable
	sshSnippetFormScrim            widget.Clickable
	sshKeys                        *sshKeyIdentityStore
	sshKeyList                     layout.List
	sshKeyNew                      widget.Clickable
	sshKeyVisibleIDs               []string
	sshKeyEditBtns                 []widget.Clickable
	sshKeyDeleteBtns               []widget.Clickable
	sshKeyFormOpen                 bool
	sshKeyFormID                   string
	sshKeyForm                     sshKeyIdentityFormValues
	sshKeyFormList                 layout.List
	sshKeyName                     widget.Editor
	sshKeyPublicKey                widget.Editor
	sshKeyFingerprint              widget.Editor
	sshKeyPrivateKey               widget.Editor
	sshKeyPassphrase               widget.Editor
	sshKeyClearSecrets             widget.Bool
	sshKeyEnabled                  widget.Bool
	sshKeyFormClose                widget.Clickable
	sshKeyFormCancel               widget.Clickable
	sshKeyFormSave                 widget.Clickable
	sshKeyFormDelete               widget.Clickable
	sshKeyFormScrim                widget.Clickable
	sshFingerprints                *sshHostFingerprintStore
	sshFingerprintList             layout.List
	sshFingerprintVisibleIDs       []string
	sshFingerprintManualBtns       []widget.Clickable
	sshFingerprintClearBtns        []widget.Clickable
	sshFingerprintCopyBtns         []widget.Clickable
	sshFingerprintCopyValues       []string
	sshFingerprintManualOpen       bool
	sshFingerprintManualHostID     string
	sshFingerprintManualEditor     widget.Editor
	sshFingerprintManualClose      widget.Clickable
	sshFingerprintManualCancel     widget.Clickable
	sshFingerprintManualSave       widget.Clickable
	sshFingerprintManualScrim      widget.Clickable
	sshHistory                     *sshSessionHistoryStore
	sshHistoryList                 layout.List
	terminalInput                  widget.Editor
	terminalText                   string
	terminal                       ptyTerminal
	ssh                            *sshclient.Client
	terminalCtx                    context.Context
	terminalCancel                 context.CancelFunc
	terminalMu                     sync.RWMutex
	terminalSize                   image.Point

	confirm          confirmation
	confirmAcceptBtn widget.Clickable
	confirmCancelBtn widget.Clickable
	confirmScrim     widget.Clickable

	busy              bool
	events            chan asyncEvent
	closed            chan struct{}
	closeOnce         sync.Once
	eventMu           sync.Mutex
	closing           bool
	nextSSHApplyID    uint64
	pendingSSHApplies map[uint64]func()
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
		model:               NewModel(remoteService),
		theme:               th,
		terminalAppearance:  normalizeTerminalAppearance(terminalAppearance{Font: terminalFontBuiltin, FontSize: 13}),
		events:              make(chan asyncEvent, 8),
		closed:              make(chan struct{}),
		language:            language,
		preferencesPath:     preferencesPath,
		workspaceModule:     sshWorkspaceHosts,
		sshTunnels:          newSSHTunnelStore(),
		sshSnippets:         newSSHCommandSnippetStore(),
		sshKeys:             newSSHKeyIdentityStore(),
		sshFingerprints:     newSSHHostFingerprintStore(),
		sshHistory:          newSSHSessionHistoryStore(),
		sshPool:             newSSHConnectionPool(),
		sshTransportFactory: newSSHClientTransport,
		transferPanelOpen:   true,
	}
	ui.transfers = newTransferManager(3, ui.executeSFTPTransfer)
	ui.transfers.onChange = func() {
		if ui.window != nil {
			ui.window.Invalidate()
		}
	}
	if remoteService != nil {
		if prefs, err := remoteService.Preferences(); err == nil {
			ui.remoteURL.SetText(prefs.BaseURL)
			ui.remoteUsername.SetText(prefs.Username)
			if password, err := remoteService.RememberedPassword(); err == nil {
				ui.remotePassword.SetText(password)
				ui.remoteRememberPassword.Value = true
				ui.remoteAutoLoginPending = true
			}
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
	if ui.sftpFiles == nil {
		ui.sftpFiles = newNativeSFTPFileDialog(window)
	}
	if ui.workspaceFiles == nil {
		if dialog, ok := ui.sftpFiles.(workspaceFileDialog); ok {
			ui.workspaceFiles = dialog
		}
	}
	ui.startRemoteAutoLogin()
	window.Option(app.Title("s12ryt SSH"), app.Size(unit.Dp(1180), unit.Dp(760)), app.MinSize(unit.Dp(680), unit.Dp(560)))
	for {
		current := window.Event()
		if listener, ok := ui.sftpFiles.(sftpFileDialogEventListener); ok {
			listener.ListenEvents(current)
		}
		switch e := current.(type) {
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
	ui.closeOnce.Do(func() {
		ui.eventMu.Lock()
		ui.closing = true
		if ui.closed != nil {
			close(ui.closed)
		}
		pending := ui.pendingSSHApplies
		ui.pendingSSHApplies = nil
		ui.eventMu.Unlock()
		for _, cleanup := range pending {
			if cleanup != nil {
				cleanup()
			}
		}
	})
	ui.closeSSHTabRename()
	ui.closeSFTPOperation()
	ui.closeSFTPUploadConflicts()
	ui.closeSSHTunnelForm()
	ui.closeSSHCommandSnippetExecution()
	ui.closeSSHCommandSnippetForm()
	ui.closeSSHKeyIdentityForm()
	ui.closeManualSSHHostFingerprint()
	ui.closeSSHWorkspaceImportExport()
	ui.closeTerminalAppearanceForm()
	if ui.sshTunnels != nil {
		ui.stopAllSSHTunnels()
		ui.sshTunnels.closeAll()
	}
	if ui.transfers != nil {
		ui.transfers.close()
	}
	ui.closeSSH()
	if ui.model.RemoteSession != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		remoteErr := ui.model.LogoutRemote(ctx)
		cancel()
		return remoteErr
	}
	return nil
}

func (ui *Window) registerSSHTabApply(cleanup func()) (uint64, bool) {
	ui.eventMu.Lock()
	defer ui.eventMu.Unlock()
	if ui.closing {
		return 0, false
	}
	if ui.pendingSSHApplies == nil {
		ui.pendingSSHApplies = make(map[uint64]func())
	}
	ui.nextSSHApplyID++
	id := ui.nextSSHApplyID
	ui.pendingSSHApplies[id] = cleanup
	return id, true
}

func (ui *Window) claimSSHTabApply(id uint64) bool {
	ui.eventMu.Lock()
	defer ui.eventMu.Unlock()
	if _, ok := ui.pendingSSHApplies[id]; !ok {
		return false
	}
	delete(ui.pendingSSHApplies, id)
	return true
}

func (ui *Window) discardSSHTabApply(id uint64) {
	ui.eventMu.Lock()
	cleanup, ok := ui.pendingSSHApplies[id]
	if ok {
		delete(ui.pendingSSHApplies, id)
	}
	ui.eventMu.Unlock()
	if ok && cleanup != nil {
		cleanup()
	}
}

func (ui *Window) closeSSH() {
	ui.finishAllSSHTabHistory()
	ui.sshTabs.closeAll()
	if ui.sshPool != nil {
		ui.sshPool.closeAll()
	}
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
	if ui.confirm.active {
		if ui.confirmScrim.Clicked(gtx) || ui.confirmCancelBtn.Clicked(gtx) {
			ui.confirm.cancel()
		}
		if ui.confirmAcceptBtn.Clicked(gtx) {
			ui.confirm.accept()
		}
		return
	}
	if ui.sftpUploadConflictOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSFTPUploadConflict(gtx)
		}
		return
	}
	if ui.terminalAppearanceOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleTerminalAppearanceForm(gtx)
		}
		return
	}
	if ui.workspaceExportOpen || ui.workspaceImportOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			if ui.workspaceExportOpen {
				ui.handleSSHWorkspaceExportForm(gtx)
			} else {
				ui.handleSSHWorkspaceImportForm(gtx)
			}
		}
		return
	}
	if ui.sshFormOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHHostForm(gtx)
		}
		return
	}
	if ui.sshTabRenameOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHTabRename(gtx)
		}
		return
	}
	if ui.sftpOperationOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSFTPOperation(gtx)
		}
		return
	}
	if ui.sshTunnelFormOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHTunnelForm(gtx)
		}
		return
	}
	if ui.sshSnippetExecutionOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHCommandSnippetExecution(gtx)
		}
		return
	}
	if ui.sshSnippetFormOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHCommandSnippetForm(gtx)
		}
		return
	}
	if ui.sshKeyFormOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleSSHKeyIdentityForm(gtx)
		}
		return
	}
	if ui.sshFingerprintManualOpen {
		if ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
			ui.handleManualSSHHostFingerprint(gtx)
		}
		return
	}
	if ui.languageButton.Clicked(gtx) {
		_ = ui.toggleLanguage()
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
	layout.Inset{Top: unit.Dp(pagePadding), Bottom: unit.Dp(pagePadding), Left: unit.Dp(pagePadding), Right: unit.Dp(pagePadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.header(gtx) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.content(gtx) }),
		)
	})
	if ui.sshFormOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshHostFormModal(gtx)
	}
	if ui.terminalAppearanceOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.terminalAppearanceModal(gtx)
	}
	if ui.sshTabRenameOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshTabRenameModal(gtx)
	}
	if ui.sftpOperationOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sftpOperationModal(gtx)
	}
	if ui.sftpUploadConflictOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sftpUploadConflictModal(gtx)
	}
	if (ui.workspaceExportOpen || ui.workspaceImportOpen) && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.workspaceImportExportModal(gtx)
	}
	if ui.sshTunnelFormOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshTunnelFormModal(gtx)
	}
	if ui.sshSnippetExecutionOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshCommandSnippetExecutionModal(gtx)
	}
	if ui.sshSnippetFormOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshCommandSnippetFormModal(gtx)
	}
	if ui.sshKeyFormOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.sshKeyIdentityFormModal(gtx)
	}
	if ui.sshFingerprintManualOpen && ui.model.Screen == ScreenRemoteWorkspace && ui.model.SSHEnabled {
		ui.manualSSHHostFingerprintModal(gtx)
	}
	if ui.confirm.active {
		ui.confirmModal(gtx)
	}
}

func (ui *Window) sftpUploadConflictModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sftpUploadConflictScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sftpUploadConflictDialog(gtx)
		}),
	)
}

func (ui *Window) sftpUploadConflictDialog(gtx layout.Context) layout.Dimensions {
	if len(ui.sftpUploadConflicts) == 0 {
		return layout.Dimensions{}
	}
	conflict := ui.sftpUploadConflicts[0]
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(520))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H6(ui.theme, ui.text("Resolve upload conflict"))
					title.Color = colorText
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					warning := material.Body1(ui.theme, ui.text("A remote file with this name already exists."))
					warning.Color = colorMuted
					return warning.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					file := material.Body1(ui.theme, conflict.Candidate.RemotePath)
					file.Color = colorTeal
					return file.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpUploadSkip, "Skip", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpUploadKeepBoth, "Keep both", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpUploadOverwrite, "Overwrite", true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshHostFormModal(gtx layout.Context) layout.Dimensions {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshFormScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshHostFormDialog(gtx)
		}),
	)
}

func (ui *Window) sshHostFormDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(680))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(680))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, ui.text("Edit SSH host"))
							if ui.sshHostID == "" {
								title = material.H6(ui.theme, ui.text("New SSH host"))
							}
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshFormCloseButton, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.sshHostFormFields(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshFormCancelButton, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSave, "Save host", true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.sshHostID == "" {
								return layout.Dimensions{}
							}
							return ui.actionButton(gtx, &ui.sshDelete, "Delete host", false, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshTabRenameModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshTabRenameScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshTabRenameDialog(gtx)
		}),
	)
}

func (ui *Window) sshTabRenameDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(440))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(240))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, ui.text("Rename terminal tab"))
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshTabRenameClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ui.sshTabRenameEditor.SingleLine = true
					ui.sshTabRenameEditor.Submit = true
					return ui.labeledField(gtx, &ui.sshTabRenameEditor, "Tab name", true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshTabRenameCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshTabRenameSave, "Save name", true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sftpOperationModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sftpOperationScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sftpOperationDialog(gtx)
		}),
	)
}

func (ui *Window) sftpOperationDialog(gtx layout.Context) layout.Dimensions {
	spec, ok := sftpOperationDialogSpec(ui.sftpOperationAction)
	if !ok {
		return layout.Dimensions{}
	}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(480))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(360))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(ui.theme, ui.text(ui.sftpOperationAction))
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpOperationClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.labeledField(gtx, &ui.sftpOperationFirst, spec.fieldSources[0], true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(spec.fieldSources) < 2 {
						return layout.Dimensions{}
					}
					return ui.labeledField(gtx, &ui.sftpOperationSecond, spec.fieldSources[1], true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpOperationCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sftpOperationSave, spec.submitSource, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshTunnelFormModal(gtx layout.Context) {
	scrim := color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.sshTunnelFormScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.sshTunnelFormDialog(gtx)
		}),
	)
}

func (ui *Window) sshTunnelFormDialog(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(640))
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(620))
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			titleSource := "New tunnel"
			if ui.sshTunnelFormID != "" {
				titleSource = "Edit tunnel"
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
							return ui.button(gtx, &ui.sshTunnelFormClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ui.sshTunnelFormList.Axis = layout.Vertical
					return ui.sshTunnelFormList.Layout(gtx, 6, func(gtx layout.Context, index int) layout.Dimensions {
						switch index {
						case 0:
							return ui.labeledField(gtx, &ui.sshTunnelName, "Tunnel name", true, false)
						case 1:
							return ui.labeledField(gtx, &ui.sshTunnelHost, "Tunnel host", true, false)
						case 2:
							return ui.sshTunnelTypeField(gtx)
						case 3:
							return ui.editorRow(gtx, "Listen host", &ui.sshTunnelListenHost, "Listen port", &ui.sshTunnelListenPort)
						case 4:
							return ui.editorRow(gtx, "Target host", &ui.sshTunnelTargetHost, "Target port", &ui.sshTunnelTargetPort)
						case 5:
							return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(cardGap)}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.CheckBox(ui.theme, &ui.sshTunnelEnabled, ui.text("Enabled")).Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.CheckBox(ui.theme, &ui.sshTunnelAutoStart, ui.text("Auto-start")).Layout(gtx)
								}),
							)
						}
						return layout.Dimensions{}
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshTunnelFormCancel, "Cancel", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshTunnelFormSave, "Save tunnel", true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.sshTunnelFormID == "" {
								return layout.Dimensions{}
							}
							return ui.actionButton(gtx, &ui.sshTunnelFormDelete, "Delete tunnel", false, true)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) sshTunnelTypeField(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.fieldLabel(gtx, "Tunnel type")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			options := sshTunnelTypeOptions()
			children := make([]layout.FlexChild, 0, len(options))
			for index, tunnelType := range options {
				index, tunnelType := index, tunnelType
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.sshTunnelTypeButtons[index], sshTunnelDirectionSource(tunnelType), ui.sshTunnelForm.Type == tunnelType)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx, children...)
		}),
	)
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
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
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
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H5(ui.theme, "s12ryt SSH").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.fieldLabel(gtx, "Secure remote workspace")
				}),
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
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

func (ui *Window) secretLabel(gtx layout.Context, text string) layout.Dimensions {
	style := material.Body1(ui.theme, text)
	style.Color = colorTeal
	style.WrapPolicy = 0
	return style.Layout(gtx)
}

// fieldLabel renders the small muted caption that keeps a filled-in editor
// identifiable after its placeholder hint disappears.
func (ui *Window) fieldLabel(gtx layout.Context, label string) layout.Dimensions {
	style := material.Label(ui.theme, unit.Sp(labelTextSize), ui.text(label))
	style.Color = colorMuted
	return style.Layout(gtx)
}

// labeledField pairs a visible caption with an editor field.
func (ui *Window) labeledField(gtx layout.Context, editor *widget.Editor, hint string, singleLine, password bool) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(fieldGap)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.fieldLabel(gtx, hint) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.field(gtx, editor, hint, singleLine, password)
		}),
	)
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
		return ui.labeledField(gtx, leftEditor, left, true, isSecretHint(left))
	}
	rightField := func(gtx layout.Context) layout.Dimensions {
		if rightEditor == nil {
			return layout.Dimensions{}
		}
		return ui.labeledField(gtx, rightEditor, right, true, isSecretHint(right))
	}
	if useStackedRow(int(float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp)) {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
			layout.Rigid(leftField),
			layout.Rigid(rightField),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
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

// buttonBlock stretches a secondary button to the full width of its slot.
func (ui *Window) buttonBlock(gtx layout.Context, click *widget.Clickable, text string, primary bool) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return ui.button(gtx, click, text, primary)
}

// actionButtonBlock stretches an operation button to the full width of its slot.
func (ui *Window) actionButtonBlock(gtx layout.Context, click *widget.Clickable, text string, primary, danger bool) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return ui.actionButton(gtx, click, text, primary, danger)
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

// surface draws a rounded card with a hairline edge so panels read as
// elevated layers over the page background.
func (ui *Window) surface(gtx layout.Context, child layout.Widget) layout.Dimensions {
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rrect := clip.UniformRRect(bounds, surfaceCornerRadius)
	paint.FillShape(gtx.Ops, colorSurface, rrect.Op(gtx.Ops))
	stroke := clip.Stroke{Path: rrect.Path(gtx.Ops), Width: 1}
	paint.FillShape(gtx.Ops, colorEdge, stroke.Op())
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
