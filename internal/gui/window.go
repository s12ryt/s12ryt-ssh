package gui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	coreapp "s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/database"
	"s12ryt-ssh/internal/i18n"
	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
	"s12ryt-ssh/internal/storage"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
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
	setupList          layout.List
	profileScroll      layout.List
	remoteList         layout.List
	terminalList       layout.List
	storageOutputList  layout.List
	databaseOutputList layout.List
	objectList         layout.List
	remoteObjectList   layout.List
	language           i18n.Language
	preferencesPath    string
	languageButton     widget.Clickable
	reveals            map[*widget.Editor]*editorReveal

	setupBackend          string
	setupName             widget.Editor
	setupPassword         widget.Editor
	setupS3Endpoint       widget.Editor
	setupS3Region         widget.Editor
	setupS3Access         widget.Editor
	setupS3Secret         widget.Editor
	setupS3Bucket         widget.Editor
	setupPathButton       widget.Clickable
	setupPathStyle        bool
	setupDBKind           string
	setupDBMySQLButton    widget.Clickable
	setupDBPostgresButton widget.Clickable
	setupDBHost           widget.Editor
	setupDBPort           widget.Editor
	setupDBUser           widget.Editor
	setupDBPassword       widget.Editor
	setupDBDatabase       widget.Editor
	setupDBSSL            widget.Editor
	setupS3Button         widget.Clickable
	setupSQLButton        widget.Clickable
	setupTest             widget.Clickable
	setupRegister         widget.Clickable

	loginName     widget.Editor
	loginPassword widget.Editor
	loginButton   widget.Clickable
	loginRecovery widget.Clickable

	recoveryKey      widget.Editor
	recoveryName     widget.Editor
	recoveryPassword widget.Editor
	recoverySubmit   widget.Clickable
	recoveryContinue widget.Clickable
	recoveryBack     widget.Clickable
	recoveryCopy     widget.Clickable

	sshTab      widget.Clickable
	storageTab  widget.Clickable
	databaseTab widget.Clickable
	logout      widget.Clickable
	remoteEntry widget.Clickable

	remoteURL      widget.Editor
	remoteUsername widget.Editor
	remotePassword widget.Editor
	remoteLogin    widget.Clickable
	remoteRestore  widget.Clickable
	remoteBack     widget.Clickable
	remoteRefresh  widget.Clickable

	remoteResources       []remote.Resource
	remoteResourceButtons []widget.Clickable
	remoteIndex           int

	sshNew            widget.Clickable
	sshSave           widget.Clickable
	sshConnect        widget.Clickable
	sshClose          widget.Clickable
	sshSend           widget.Clickable
	sshProfileButtons []widget.Clickable
	sshIndex          int
	sshName           widget.Editor
	sshHost           widget.Editor
	sshPort           widget.Editor
	sshUser           widget.Editor
	sshPassword       widget.Editor
	sshKeyPath        widget.Editor
	sshKeyPass        widget.Editor
	sshFingerprint    widget.Editor
	terminalInput     widget.Editor
	terminalText      string
	terminal          ptyTerminal
	ssh               *sshclient.Client
	terminalCtx       context.Context
	terminalCancel    context.CancelFunc
	terminalMu        sync.RWMutex
	terminalSize      image.Point

	confirm             confirmation
	confirmAcceptBtn    widget.Clickable
	confirmCancelBtn    widget.Clickable
	confirmScrim        widget.Clickable
	remoteObjects       []remote.S3Object
	objectButtons       []widget.Clickable
	remoteObjectButtons []widget.Clickable

	storageNew            widget.Clickable
	storageSave           widget.Clickable
	storageRefresh        widget.Clickable
	storageUpload         widget.Clickable
	storageDownload       widget.Clickable
	storageDelete         widget.Clickable
	storageProfileButtons []widget.Clickable
	storageIndex          int
	storageName           widget.Editor
	storageEndpoint       widget.Editor
	storageRegion         widget.Editor
	storageAccess         widget.Editor
	storageSecret         widget.Editor
	storageBucket         widget.Editor
	storagePathButton     widget.Clickable
	storagePathStyle      bool
	storagePrefix         widget.Editor
	storageKey            widget.Editor
	storagePath           widget.Editor
	storageData           widget.Editor
	storageText           string
	objects               []storage.Object

	databaseNew            widget.Clickable
	databaseSave           widget.Clickable
	databaseTables         widget.Clickable
	databaseQuery          widget.Clickable
	databaseExec           widget.Clickable
	databaseProfileButtons []widget.Clickable
	databaseIndex          int
	databaseName           widget.Editor
	databaseKind           string
	databaseMySQLButton    widget.Clickable
	databasePostgresButton widget.Clickable
	databaseHost           widget.Editor
	databasePort           widget.Editor
	databaseUser           widget.Editor
	databasePassword       widget.Editor
	databaseSchema         widget.Editor
	databaseSSL            widget.Editor
	databaseTLS            widget.Editor
	databaseSQL            widget.Editor
	databaseText           string

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
func NewWindow(service *coreapp.Service) *Window {
	return NewWindowWithPreferences(service, "")
}

// NewWindowWithPreferences creates a window and loads its non-sensitive language preference.
func NewWindowWithPreferences(service *coreapp.Service, preferencesPath string) *Window {
	return NewWindowWithServices(service, nil, preferencesPath)
}

// NewWindowWithServices creates a window with local and optional remote authentication services.
func NewWindowWithServices(service *coreapp.Service, remoteService *remote.Service, preferencesPath string) *Window {
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
		model:           NewModelWithRemote(service, remoteService),
		theme:           th,
		setupBackend:    "s3",
		setupDBKind:     dbTypeMySQL,
		databaseKind:    dbTypeMySQL,
		sshIndex:        -1,
		storageIndex:    -1,
		databaseIndex:   -1,
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

// Close releases live SSH resources and the decrypted session.
func (ui *Window) Close() error {
	if ui == nil {
		return nil
	}
	ui.closeSSH()
	var remoteErr error
	if ui.model.RemoteSession != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		remoteErr = ui.model.LogoutRemote(ctx)
		cancel()
	}
	localErr := ui.model.Logout()
	if remoteErr != nil {
		return remoteErr
	}
	return localErr
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
	case ScreenSetup:
		ui.handleSetup(gtx)
	case ScreenLogin:
		ui.handleLogin(gtx)
	case ScreenRecovery:
		ui.handleRecovery(gtx)
	case ScreenWorkspace:
		ui.handleWorkspace(gtx)
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

func (ui *Window) handleSetup(gtx layout.Context) {
	if ui.remoteEntry.Clicked(gtx) {
		ui.model.BeginRemoteLogin()
		return
	}
	if ui.drainEditors(gtx, &ui.setupName, &ui.setupPassword, &ui.setupS3Endpoint, &ui.setupS3Region, &ui.setupS3Access, &ui.setupS3Secret, &ui.setupS3Bucket, &ui.setupDBHost, &ui.setupDBPort, &ui.setupDBUser, &ui.setupDBPassword, &ui.setupDBDatabase, &ui.setupDBSSL) {
		ui.tryCreateVault()
		return
	}
	if ui.setupS3Button.Clicked(gtx) {
		ui.setupBackend = "s3"
	}
	if ui.setupSQLButton.Clicked(gtx) {
		ui.setupBackend = "sql"
	}
	if ui.setupPathButton.Clicked(gtx) {
		ui.setupPathStyle = !ui.setupPathStyle
	}
	if ui.setupDBMySQLButton.Clicked(gtx) {
		ui.setupDBKind = dbTypeMySQL
	}
	if ui.setupDBPostgresButton.Clicked(gtx) {
		ui.setupDBKind = dbTypePostgres
	}
	if ui.setupTest.Clicked(gtx) {
		bootstrap, err := ui.setupBootstrap()
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		ui.async("Testing vault connection...", func(ctx context.Context) (func(), error) {
			return nil, ui.model.Service.TestBootstrap(ctx, bootstrap)
		})
	}
	if ui.setupRegister.Clicked(gtx) {
		ui.tryCreateVault()
	}
}

func (ui *Window) handleLogin(gtx layout.Context) {
	if ui.remoteEntry.Clicked(gtx) {
		ui.model.BeginRemoteLogin()
		return
	}
	if ui.drainEditors(gtx, &ui.loginName, &ui.loginPassword) {
		ui.trySignIn()
		return
	}
	if ui.loginRecovery.Clicked(gtx) {
		ui.model.BeginRecovery()
		ui.recoveryKey.SetText("")
		ui.recoveryName.SetText(ui.loginName.Text())
		ui.recoveryPassword.SetText("")
	}
	if ui.loginButton.Clicked(gtx) {
		ui.trySignIn()
	}
}

func (ui *Window) handleRecovery(gtx layout.Context) {
	if ui.recoveryBack.Clicked(gtx) {
		ui.model.ContinueFromRecovery()
		return
	}
	if ui.recoveryContinue.Clicked(gtx) && ui.model.RecoveryKey != "" {
		ui.model.ContinueFromRecovery()
		return
	}
	if ui.recoveryCopy.Clicked(gtx) && ui.model.RecoveryKey != "" {
		gtx.Execute(clipboard.WriteCmd{
			Type: "text/plain",
			Data: io.NopCloser(strings.NewReader(ui.model.RecoveryKey)),
		})
		ui.model.Status = "Recovery key copied to clipboard."
		return
	}
	if ui.recoverySubmit.Clicked(gtx) && ui.model.RecoveryKey == "" {
		ui.tryRotateRecovery()
		return
	}
	if ui.model.RecoveryKey == "" && ui.drainEditors(gtx, &ui.recoveryKey, &ui.recoveryName, &ui.recoveryPassword) {
		ui.tryRotateRecovery()
	}
}

func (ui *Window) handleWorkspace(gtx layout.Context) {
	if ui.logout.Clicked(gtx) && !ui.busy {
		_ = ui.Close()
		return
	}
	if ui.sshTab.Clicked(gtx) {
		ui.model.SelectTab(TabSSH)
	}
	if ui.storageTab.Clicked(gtx) {
		ui.model.SelectTab(TabStorage)
	}
	if ui.databaseTab.Clicked(gtx) {
		ui.model.SelectTab(TabDatabase)
	}
	switch ui.model.Tab {
	case TabSSH:
		ui.handleSSH(gtx)
	case TabStorage:
		ui.handleStorage(gtx)
	case TabDatabase:
		ui.handleDatabase(gtx)
	}
}

func (ui *Window) handleSSH(gtx layout.Context) {
	for i := range ui.sshProfileButtons {
		if ui.sshProfileButtons[i].Clicked(gtx) {
			ui.sshIndex = i
			ui.loadSSHProfile()
		}
	}
	if ui.sshNew.Clicked(gtx) {
		ui.sshIndex = -1
		ui.clearSSHProfile()
	}
	if ui.sshSave.Clicked(gtx) {
		profile, err := ui.sshProfile()
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		ui.saveProfile(func(profiles *config.Store) { profiles.SSH = replaceSSH(profiles.SSH, ui.sshIndex, profile) }, func() { ui.sshIndex = 0 })
	}
	if ui.sshConnect.Clicked(gtx) {
		ui.trySSHConnect()
	}
	if ui.sshClose.Clicked(gtx) {
		ui.closeSSH()
		ui.model.Status = "SSH connection closed."
	}
	terminalSubmitted := ui.drainEditors(gtx, &ui.terminalInput)
	formSubmitted := ui.drainEditors(gtx, &ui.sshName, &ui.sshHost, &ui.sshPort, &ui.sshUser, &ui.sshPassword, &ui.sshKeyPath, &ui.sshKeyPass, &ui.sshFingerprint)
	if ui.sshSend.Clicked(gtx) || terminalSubmitted {
		ui.sendTerminalInput()
	} else if formSubmitted {
		ui.trySSHConnect()
	}
}

func (ui *Window) handleStorage(gtx layout.Context) {
	for i := range ui.storageProfileButtons {
		if ui.storageProfileButtons[i].Clicked(gtx) {
			ui.storageIndex = i
			ui.loadStorageProfile()
		}
	}
	for i := range ui.objectButtons {
		if ui.objectButtons[i].Clicked(gtx) {
			ui.selectObject(i)
		}
	}
	ui.drainEditors(gtx, &ui.storageName, &ui.storageEndpoint, &ui.storageRegion, &ui.storageAccess, &ui.storageSecret, &ui.storageBucket, &ui.storagePrefix, &ui.storageKey, &ui.storagePath, &ui.storageData)
	if ui.storageNew.Clicked(gtx) {
		ui.storageIndex = -1
		ui.clearStorageProfile()
	}
	if ui.storagePathButton.Clicked(gtx) {
		ui.toggleStoragePathStyle()
	}
	if ui.storageSave.Clicked(gtx) {
		profile, err := ui.storageProfile()
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		ui.saveProfile(func(profiles *config.Store) { profiles.S3 = replaceS3(profiles.S3, ui.storageIndex, profile) }, func() { ui.storageIndex = 0 })
	}
	profile, profileErr := ui.storageProfile()
	requireProfile := func() bool {
		if profileErr != nil {
			ui.model.Error = profileErr.Error()
			return false
		}
		return true
	}
	if ui.storageRefresh.Clicked(gtx) {
		if !requireProfile() {
			return
		}
		ui.async("Listing remote objects...", func(ctx context.Context) (func(), error) {
			remote, err := storage.NewS3Storage(profile)
			if err != nil {
				return nil, err
			}
			objects, err := remote.List(ctx, ui.storagePrefix.Text())
			return func() { ui.objects, ui.storageText = objects, ui.formatObjects(objects) }, err
		})
	}
	if ui.storageUpload.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		if !requireProfile() {
			return
		}
		key, path, inline := ui.storageKey.Text(), ui.storagePath.Text(), ui.storageData.Text()
		ui.async("Uploading object...", func(ctx context.Context) (func(), error) {
			var data []byte
			var err error
			if path != "" {
				data, err = os.ReadFile(path)
			} else {
				data = []byte(inline)
			}
			if err != nil {
				return nil, err
			}
			remote, err := storage.NewS3Storage(profile)
			if err != nil {
				return nil, err
			}
			return nil, remote.Put(ctx, key, data)
		})
	}
	if ui.storageDownload.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		if !requireProfile() {
			return
		}
		key, path := ui.storageKey.Text(), ui.storagePath.Text()
		ui.async("Downloading object...", func(ctx context.Context) (func(), error) {
			remote, err := storage.NewS3Storage(profile)
			if err != nil {
				return nil, err
			}
			data, err := remote.Get(ctx, key)
			if err != nil {
				return nil, err
			}
			if path != "" {
				if err := os.WriteFile(path, data, 0o600); err != nil {
					return nil, err
				}
			}
			return func() {
				ui.storageText = fmt.Sprintf("%s%d %s%s", ui.text("Downloaded "), len(data), ui.text("Bytes"), ui.downloadedTo(path))
			}, nil
		})
	}
	if ui.storageDelete.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		if !requireProfile() {
			return
		}
		key := ui.storageKey.Text()
		ui.requestConfirm("Delete object", "This permanently deletes the object. This action cannot be undone.", func() {
			ui.async("Deleting object...", func(ctx context.Context) (func(), error) {
				remote, err := storage.NewS3Storage(profile)
				if err != nil {
					return nil, err
				}
				return nil, remote.Delete(ctx, key)
			})
		})
	}
}

func (ui *Window) handleDatabase(gtx layout.Context) {
	for i := range ui.databaseProfileButtons {
		if ui.databaseProfileButtons[i].Clicked(gtx) {
			ui.databaseIndex = i
			ui.loadDatabaseProfile()
		}
	}
	ui.drainEditors(gtx, &ui.databaseName, &ui.databaseHost, &ui.databasePort, &ui.databaseUser, &ui.databasePassword, &ui.databaseSchema, &ui.databaseSSL, &ui.databaseTLS, &ui.databaseSQL)
	if ui.databaseMySQLButton.Clicked(gtx) {
		ui.databaseKind = dbTypeMySQL
	}
	if ui.databasePostgresButton.Clicked(gtx) {
		ui.databaseKind = dbTypePostgres
	}
	if ui.databaseNew.Clicked(gtx) {
		ui.databaseIndex = -1
		ui.clearDatabaseProfile()
	}
	if ui.databaseSave.Clicked(gtx) {
		profile, err := ui.databaseProfile()
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		ui.saveProfile(func(profiles *config.Store) { profiles.DB = replaceDB(profiles.DB, ui.databaseIndex, profile) }, func() { ui.databaseIndex = 0 })
	}
	profile, profileErr := ui.databaseProfile()
	requireProfile := func() bool {
		if profileErr != nil {
			ui.model.Error = profileErr.Error()
			return false
		}
		return true
	}
	if ui.databaseTables.Clicked(gtx) {
		if !requireProfile() {
			return
		}
		ui.async("Loading database tables...", func(ctx context.Context) (func(), error) {
			client, err := database.NewDBClient(profile)
			if err != nil {
				return nil, err
			}
			defer client.Close()
			tables, err := client.Tables(ctx)
			return func() { ui.databaseText = strings.Join(tables, "\n") }, err
		})
	}
	if ui.databaseQuery.Clicked(gtx) {
		if err := requireSQLStatement(ui.databaseSQL.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		if !requireProfile() {
			return
		}
		query := ui.databaseSQL.Text()
		ui.async("Running database query...", func(ctx context.Context) (func(), error) {
			client, err := database.NewDBClient(profile)
			if err != nil {
				return nil, err
			}
			defer client.Close()
			rows, err := client.Query(ctx, query)
			return func() { ui.databaseText = ui.formatRows(rows) }, err
		})
	}
	if ui.databaseExec.Clicked(gtx) {
		if err := requireSQLStatement(ui.databaseSQL.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		if !requireProfile() {
			return
		}
		query := ui.databaseSQL.Text()
		ui.requestConfirm("Execute SQL statement", "This runs a statement that can modify data. Continue?", func() {
			ui.async("Executing database statement...", func(ctx context.Context) (func(), error) {
				client, err := database.NewDBClient(profile)
				if err != nil {
					return nil, err
				}
				defer client.Close()
				result, err := client.Exec(ctx, query)
				return func() {
					ui.databaseText = fmt.Sprintf("%s%d\n%s%d", ui.text("Rows affected: "), result.RowsAffected, ui.text("Last insert ID: "), result.LastInsertID)
				}, err
			})
		})
	}
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
			if ui.model.Screen == ScreenWorkspace || ui.model.Screen == ScreenRemoteWorkspace {
				return ui.button(gtx, &ui.logout, ui.text("Log out"), false)
			}
			return layout.Dimensions{}
		}),
	)
}

func (ui *Window) content(gtx layout.Context) layout.Dimensions {
	switch ui.model.Screen {
	case ScreenSetup:
		return ui.setupView(gtx)
	case ScreenLogin:
		return ui.loginView(gtx)
	case ScreenRecovery:
		return ui.recoveryView(gtx)
	case ScreenWorkspace:
		return ui.workspaceView(gtx)
	case ScreenRemoteLogin:
		return ui.remoteLoginView(gtx)
	case ScreenRemoteWorkspace:
		return ui.remoteWorkspaceView(gtx)
	default:
		return layout.Dimensions{}
	}
}

func (ui *Window) setupView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24), Left: unit.Dp(28), Right: unit.Dp(28)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.setupList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(12)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H5(ui.theme, ui.text("Create encrypted vault")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Body2(ui.theme, ui.text("Bootstrap credentials are protected by Windows DPAPI; profiles are encrypted before upload.")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.backendSelector(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.setupName, "Vault name", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.setupPassword, "Vault password", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if ui.setupBackend == "s3" {
							return ui.s3BootstrapFields(gtx)
						}
						return ui.sqlBootstrapFields(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(10)}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.actionButton(gtx, &ui.setupTest, "Test connection", false, false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.actionButton(gtx, &ui.setupRegister, "Create vault", true, false)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.remoteEntry, "Remote sign in", false)
					}),
				)
			})
		})
	})
}

func (ui *Window) loginView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(36), Bottom: unit.Dp(36), Left: unit.Dp(42), Right: unit.Dp(42)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(14)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H5(ui.theme, ui.text("Unlock your vault")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Body2(ui.theme, ui.text("Your profiles stay encrypted at rest and are decrypted only after sign-in.")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.loginName, "Vault name", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.loginPassword, "Vault password", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.actionButton(gtx, &ui.loginButton, "Sign in", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.loginRecovery, "Use recovery key", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.remoteEntry, "Remote sign in", false)
					}),
				)
			})
		})
	})
}

func (ui *Window) recoveryView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(26), Bottom: unit.Dp(26), Left: unit.Dp(30), Right: unit.Dp(30)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(12)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.model.RecoveryKey != "" {
						return material.H5(ui.theme, ui.text("Save your recovery key")).Layout(gtx)
					}
					return material.H5(ui.theme, ui.text("Recover vault access")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.model.RecoveryKey != "" {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8), Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.secretLabel(gtx, ui.model.RecoveryKey)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.button(gtx, &ui.recoveryCopy, "Copy recovery key", false)
							}),
						)
					}
					return ui.field(gtx, &ui.recoveryKey, "One-time recovery key", true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.model.RecoveryKey != "" {
						return ui.button(gtx, &ui.recoveryContinue, "Continue to sign in", true)
					}
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(12)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.field(gtx, &ui.recoveryName, "New vault name", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.field(gtx, &ui.recoveryPassword, "New vault password", true, true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.recoverySubmit, "Rotate credentials", true, false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.recoveryBack, "Back to sign in", false)
				}),
			)
		})
	})
}

func (ui *Window) workspaceView(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(12)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.tabs(gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(12)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.profileSidebar(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					switch ui.model.Tab {
					case TabSSH:
						return ui.sshView(gtx)
					case TabStorage:
						return ui.storageView(gtx)
					default:
						return ui.databaseView(gtx)
					}
				}),
			)
		}),
	)
}

func (ui *Window) tabs(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.sshTab, "SSH terminal", ui.model.Tab == TabSSH)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.storageTab, "S3 / R2", ui.model.Tab == TabStorage)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.databaseTab, "SQL database", ui.model.Tab == TabDatabase)
		}),
	)
}

func (ui *Window) profileSidebar(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(230)
		gtx.Constraints.Max.X = gtx.Dp(270)
		return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(14), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text(ui.profileTitle())).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.profileList(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.profileNewButton(gtx) }),
			)
		})
	})
}

func (ui *Window) profileList(gtx layout.Context) layout.Dimensions {
	labels := ui.profileLabels()
	if len(labels) == 0 {
		hint := material.Body2(ui.theme, ui.text("No profiles yet. Create one below."))
		hint.Color = colorMuted
		return hint.Layout(gtx)
	}
	return ui.profileScroll.Layout(gtx, len(labels), func(gtx layout.Context, index int) layout.Dimensions {
		var button *widget.Clickable
		switch ui.model.Tab {
		case TabSSH:
			button = &ui.sshProfileButtons[index]
		case TabStorage:
			button = &ui.storageProfileButtons[index]
		default:
			button = &ui.databaseProfileButtons[index]
		}
		return ui.button(gtx, button, labels[index], ui.profileIndex() == index)
	})
}

func (ui *Window) sshView(gtx layout.Context) layout.Dimensions {
	ui.ensureSSHButtons()
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("SSH profile")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Name", &ui.sshName, "Host", &ui.sshHost)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Port", &ui.sshPort, "User", &ui.sshUser)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Password", &ui.sshPassword, "Key path", &ui.sshKeyPath)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Key passphrase", &ui.sshKeyPass, "Host fingerprint", &ui.sshFingerprint)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshNew, "New", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.sshSave, "Save profile", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.sshConnect, "Connect", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshClose, "Close", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.field(gtx, &ui.terminalInput, "Terminal input", true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.actionButton(gtx, &ui.sshSend, "Send to terminal", true, false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.outputList(gtx, &ui.terminalList, ui.terminalSnapshot(), "Terminal output", true)
				}),
			)
		})
	})
}

func (ui *Window) storageView(gtx layout.Context) layout.Dimensions {
	ui.ensureStorageButtons()
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("S3 / R2 profile")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Name", &ui.storageName, "Endpoint", &ui.storageEndpoint)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Region", &ui.storageRegion, "Bucket", &ui.storageBucket)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Access key", &ui.storageAccess, "Secret key", &ui.storageSecret)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "Path-style requests: off"
					if ui.storagePathStyle {
						label = "Path-style requests: on"
					}
					return ui.button(gtx, &ui.storagePathButton, label, ui.storagePathStyle)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.storageNew, "New", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.storageSave, "Save profile", true, false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "List prefix", &ui.storagePrefix, "Object key", &ui.storageKey)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Local path", &ui.storagePath, "Inline upload data", &ui.storageData)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.storageRefresh, "Refresh list", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.storageUpload, "Upload", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.storageDownload, "Download", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.storageDelete, "Delete", false, true)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.objectBrowser(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.outputList(gtx, &ui.storageOutputList, ui.storageText, "Objects and operation output", false)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) databaseView(gtx layout.Context) layout.Dimensions {
	ui.ensureDatabaseButtons()
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("SQL profile")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.field(gtx, &ui.databaseName, "Name", true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.dbTypeSelector(gtx, &ui.databaseMySQLButton, &ui.databasePostgresButton, ui.databaseKind)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Host", &ui.databaseHost, "Port", &ui.databasePort)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "User", &ui.databaseUser, "Password", &ui.databasePassword)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Database", &ui.databaseSchema, "SSL mode", &ui.databaseSSL)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.field(gtx, &ui.databaseTLS, "MySQL TLS mode", true, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.databaseNew, "New", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.databaseSave, "Save profile", true, false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.field(gtx, &ui.databaseSQL, "SQL query or statement", false, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.databaseTables, "List tables", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.databaseQuery, "Run query", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.databaseExec, "Run exec", false, true)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.outputList(gtx, &ui.databaseOutputList, ui.databaseText, "Database output", true)
				}),
			)
		})
	})
}

func (ui *Window) backendSelector(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.setupS3Button, "R2 / S3 vault", ui.setupBackend == "s3")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.setupSQLButton, "SQL vault", ui.setupBackend == "sql")
		}),
	)
}

func (ui *Window) s3BootstrapFields(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.editorRow(gtx, "Endpoint", &ui.setupS3Endpoint, "Region", &ui.setupS3Region)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.editorRow(gtx, "Access key", &ui.setupS3Access, "Secret key", &ui.setupS3Secret)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.field(gtx, &ui.setupS3Bucket, "Vault bucket", true, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Path-style requests: off"
			if ui.setupPathStyle {
				label = "Path-style requests: on"
			}
			return ui.button(gtx, &ui.setupPathButton, label, ui.setupPathStyle)
		}),
	)
}

func (ui *Window) sqlBootstrapFields(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.dbTypeSelector(gtx, &ui.setupDBMySQLButton, &ui.setupDBPostgresButton, ui.setupDBKind)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.field(gtx, &ui.setupDBHost, "Host", true, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.editorRow(gtx, "Port", &ui.setupDBPort, "User", &ui.setupDBUser)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.editorRow(gtx, "Password", &ui.setupDBPassword, "Database", &ui.setupDBDatabase)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.field(gtx, &ui.setupDBSSL, "PostgreSQL SSL mode (default require)", true, false)
		}),
	)
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

func (ui *Window) formatObjects(objects []storage.Object) string {
	if len(objects) == 0 {
		return ui.text("No objects found.")
	}
	var b strings.Builder
	for _, object := range objects {
		fmt.Fprintf(&b, "%s  (%d %s)\n", object.Key, object.Size, ui.text("Bytes"))
	}
	return b.String()
}

func (ui *Window) formatRows(rows []database.Row) string {
	if len(rows) == 0 {
		return ui.text("No rows returned.")
	}
	return formatRows(rows)
}

func (ui *Window) downloadedTo(path string) string {
	if path == "" {
		return ui.text(" (preview available in output)")
	}
	return ui.text(" to ") + path
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

// objectBrowser renders the clickable listing of the current S3 objects;
// selecting an entry copies its key into the object key editor.
func (ui *Window) objectBrowser(gtx layout.Context) layout.Dimensions {
	ui.ensureObjectButtons()
	if len(ui.objects) == 0 {
		muted := material.Body2(ui.theme, ui.objectsHeader(0))
		muted.Color = colorMuted
		return muted.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(4)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			header := material.Body2(ui.theme, ui.objectsHeader(len(ui.objects)))
			header.Color = colorMuted
			return header.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.objectList.Layout(gtx, len(ui.objects), func(gtx layout.Context, index int) layout.Dimensions {
				label := fmt.Sprintf("%s (%d %s)", ui.objects[index].Key, ui.objects[index].Size, ui.text("Bytes"))
				return ui.button(gtx, &ui.objectButtons[index], label, false)
			})
		}),
	)
}

func (ui *Window) ensureObjectButtons() {
	if len(ui.objectButtons) != len(ui.objects) {
		ui.objectButtons = make([]widget.Clickable, len(ui.objects))
	}
}

func (ui *Window) ensureRemoteObjectButtons() {
	if len(ui.remoteObjectButtons) != len(ui.remoteObjects) {
		ui.remoteObjectButtons = make([]widget.Clickable, len(ui.remoteObjects))
	}
}

// dbTypeSelector renders the MySQL / PostgreSQL picker for the given kind.
func (ui *Window) dbTypeSelector(gtx layout.Context, mysqlButton, postgresButton *widget.Clickable, kind string) layout.Dimensions {
	choices := dbTypeChoices()
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, mysqlButton, choices[0], kind == dbTypeMySQL)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, postgresButton, choices[1], kind == dbTypePostgres)
		}),
	)
}

func (ui *Window) surface(gtx layout.Context, child layout.Widget) layout.Dimensions {
	defer clip.UniformRRect(image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), 8).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorSurface)
	return child(gtx)
}

func (ui *Window) setupBootstrap() (coreapp.Bootstrap, error) {
	if ui.setupBackend == "s3" {
		endpoint, region, access, secret, bucket := strings.TrimSpace(ui.setupS3Endpoint.Text()), strings.TrimSpace(ui.setupS3Region.Text()), ui.setupS3Access.Text(), ui.setupS3Secret.Text(), strings.TrimSpace(ui.setupS3Bucket.Text())
		if endpoint == "" || bucket == "" || strings.TrimSpace(access) == "" || strings.TrimSpace(secret) == "" {
			return coreapp.Bootstrap{}, fmt.Errorf("S3 endpoint, bucket, access key, and secret key are required")
		}
		return coreapp.Bootstrap{Backend: "s3", S3: config.S3Profile{
			Endpoint: endpoint, Region: region, AccessKey: access, SecretKey: secret, Bucket: bucket, UsePathStyle: ui.setupPathStyle,
		}}, nil
	}
	kind, err := normalizeDBType(ui.setupDBKind)
	if err != nil {
		return coreapp.Bootstrap{}, err
	}
	host, user, password, databaseName := strings.TrimSpace(ui.setupDBHost.Text()), strings.TrimSpace(ui.setupDBUser.Text()), ui.setupDBPassword.Text(), strings.TrimSpace(ui.setupDBDatabase.Text())
	if host == "" || strings.TrimSpace(ui.setupDBPort.Text()) == "" || user == "" || password == "" || databaseName == "" {
		return coreapp.Bootstrap{}, fmt.Errorf("SQL host, port, user, password, and database are required")
	}
	port, err := parsePort(ui.setupDBPort.Text())
	if err != nil {
		return coreapp.Bootstrap{}, err
	}
	return coreapp.Bootstrap{Backend: "sql", DB: config.DBProfile{
		Type: kind, Host: host, Port: port, User: user, Password: password, Database: databaseName, SSLMode: strings.TrimSpace(ui.setupDBSSL.Text()),
	}}, nil
}

func (ui *Window) refreshProfiles() {
	if ui.model.Session == nil {
		return
	}
	profiles, err := ui.model.Session.Profiles()
	if err != nil {
		ui.model.Error = err.Error()
		return
	}
	if len(profiles.SSH) > 0 && ui.sshIndex < 0 {
		ui.sshIndex = 0
	}
	if len(profiles.S3) > 0 && ui.storageIndex < 0 {
		ui.storageIndex = 0
	}
	if len(profiles.DB) > 0 && ui.databaseIndex < 0 {
		ui.databaseIndex = 0
	}
	ui.syncProfileButtons(profiles)
	ui.loadSSHProfileFrom(profiles)
	ui.loadStorageProfileFrom(profiles)
	ui.loadDatabaseProfileFrom(profiles)
}

func (ui *Window) syncProfileButtons(profiles *config.Store) {
	ui.sshProfileButtons = make([]widget.Clickable, len(profiles.SSH))
	ui.storageProfileButtons = make([]widget.Clickable, len(profiles.S3))
	ui.databaseProfileButtons = make([]widget.Clickable, len(profiles.DB))
}

func (ui *Window) ensureSSHButtons() {
	if ui.model.Session == nil {
		return
	}
	profiles, _ := ui.model.Session.Profiles()
	if len(ui.sshProfileButtons) != len(profiles.SSH) {
		ui.syncProfileButtons(profiles)
	}
}

func (ui *Window) ensureStorageButtons() {
	if ui.model.Session == nil {
		return
	}
	profiles, _ := ui.model.Session.Profiles()
	if len(ui.storageProfileButtons) != len(profiles.S3) {
		ui.syncProfileButtons(profiles)
	}
}

func (ui *Window) ensureDatabaseButtons() {
	if ui.model.Session == nil {
		return
	}
	profiles, _ := ui.model.Session.Profiles()
	if len(ui.databaseProfileButtons) != len(profiles.DB) {
		ui.syncProfileButtons(profiles)
	}
}

func (ui *Window) profileTitle() string {
	switch ui.model.Tab {
	case TabSSH:
		return "SSH profiles"
	case TabStorage:
		return "Storage profiles"
	default:
		return "Database profiles"
	}
}

func (ui *Window) profileLabels() []string {
	if ui.model.Session == nil {
		return nil
	}
	profiles, _ := ui.model.Session.Profiles()
	switch ui.model.Tab {
	case TabSSH:
		labels := make([]string, len(profiles.SSH))
		for i := range profiles.SSH {
			labels[i] = profiles.SSH[i].Name
		}
		return labels
	case TabStorage:
		labels := make([]string, len(profiles.S3))
		for i := range profiles.S3 {
			labels[i] = profiles.S3[i].Name
		}
		return labels
	default:
		labels := make([]string, len(profiles.DB))
		for i := range profiles.DB {
			labels[i] = profiles.DB[i].Name
		}
		return labels
	}
}

func (ui *Window) profileIndex() int {
	switch ui.model.Tab {
	case TabSSH:
		return ui.sshIndex
	case TabStorage:
		return ui.storageIndex
	default:
		return ui.databaseIndex
	}
}

func (ui *Window) profileNewButton(gtx layout.Context) layout.Dimensions {
	switch ui.model.Tab {
	case TabSSH:
		return ui.button(gtx, &ui.sshNew, "New profile", false)
	case TabStorage:
		return ui.button(gtx, &ui.storageNew, "New profile", false)
	default:
		return ui.button(gtx, &ui.databaseNew, "New profile", false)
	}
}

func (ui *Window) loadSSHProfile() {
	profiles, _ := ui.model.Session.Profiles()
	ui.loadSSHProfileFrom(profiles)
}
func (ui *Window) loadSSHProfileFrom(profiles *config.Store) {
	if ui.sshIndex < 0 || ui.sshIndex >= len(profiles.SSH) {
		return
	}
	p := profiles.SSH[ui.sshIndex]
	ui.sshName.SetText(p.Name)
	ui.sshHost.SetText(p.Host)
	ui.sshPort.SetText(strconv.Itoa(p.Port))
	ui.sshUser.SetText(p.User)
	ui.sshPassword.SetText(p.Password)
	ui.sshKeyPath.SetText(p.KeyPath)
	ui.sshKeyPass.SetText(p.KeyPassphrase)
	ui.sshFingerprint.SetText(p.HostKeyFingerprint)
}
func (ui *Window) clearSSHProfile() {
	for _, editor := range []*widget.Editor{&ui.sshName, &ui.sshHost, &ui.sshPort, &ui.sshUser, &ui.sshPassword, &ui.sshKeyPath, &ui.sshKeyPass, &ui.sshFingerprint} {
		editor.SetText("")
	}
	ui.sshPort.SetText("22")
}
func (ui *Window) sshProfile() (config.SSHProfile, error) {
	port, err := parsePort(ui.sshPort.Text())
	if err != nil {
		return config.SSHProfile{}, err
	}
	p := config.SSHProfile{Name: ui.sshName.Text(), Host: ui.sshHost.Text(), Port: port, User: ui.sshUser.Text(), Password: ui.sshPassword.Text(), KeyPath: ui.sshKeyPath.Text(), KeyPassphrase: ui.sshKeyPass.Text(), HostKeyFingerprint: ui.sshFingerprint.Text()}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Host) == "" || strings.TrimSpace(p.User) == "" {
		return config.SSHProfile{}, fmt.Errorf("SSH name, host, and user are required")
	}
	if p.Password == "" && p.KeyPath == "" {
		return config.SSHProfile{}, fmt.Errorf("SSH password or key path is required")
	}
	return p, nil
}

func (ui *Window) loadStorageProfile() {
	profiles, _ := ui.model.Session.Profiles()
	ui.loadStorageProfileFrom(profiles)
}
func (ui *Window) loadStorageProfileFrom(profiles *config.Store) {
	if ui.storageIndex < 0 || ui.storageIndex >= len(profiles.S3) {
		return
	}
	p := profiles.S3[ui.storageIndex]
	ui.storageName.SetText(p.Name)
	ui.storageEndpoint.SetText(p.Endpoint)
	ui.storageRegion.SetText(p.Region)
	ui.storageAccess.SetText(p.AccessKey)
	ui.storageSecret.SetText(p.SecretKey)
	ui.storageBucket.SetText(p.Bucket)
	ui.storagePathStyle = p.UsePathStyle
}
func (ui *Window) clearStorageProfile() {
	for _, editor := range []*widget.Editor{&ui.storageName, &ui.storageEndpoint, &ui.storageRegion, &ui.storageAccess, &ui.storageSecret, &ui.storageBucket} {
		editor.SetText("")
	}
	ui.storagePathStyle = false
}
func (ui *Window) toggleStoragePathStyle() {
	ui.storagePathStyle = !ui.storagePathStyle
}
func (ui *Window) storageProfile() (config.S3Profile, error) {
	p := config.S3Profile{Name: ui.storageName.Text(), Endpoint: ui.storageEndpoint.Text(), Region: ui.storageRegion.Text(), AccessKey: ui.storageAccess.Text(), SecretKey: ui.storageSecret.Text(), Bucket: ui.storageBucket.Text(), UsePathStyle: ui.storagePathStyle}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Endpoint) == "" || p.AccessKey == "" || p.SecretKey == "" || strings.TrimSpace(p.Bucket) == "" {
		return config.S3Profile{}, fmt.Errorf("storage name, endpoint, access key, secret key, and bucket are required")
	}
	return p, nil
}

func (ui *Window) loadDatabaseProfile() {
	profiles, _ := ui.model.Session.Profiles()
	ui.loadDatabaseProfileFrom(profiles)
}
func (ui *Window) loadDatabaseProfileFrom(profiles *config.Store) {
	if ui.databaseIndex < 0 || ui.databaseIndex >= len(profiles.DB) {
		return
	}
	p := profiles.DB[ui.databaseIndex]
	ui.databaseName.SetText(p.Name)
	ui.databaseKind = applyDatabaseKind(ui.databaseKind, p.Type)
	ui.databaseHost.SetText(p.Host)
	ui.databasePort.SetText(strconv.Itoa(p.Port))
	ui.databaseUser.SetText(p.User)
	ui.databasePassword.SetText(p.Password)
	ui.databaseSchema.SetText(p.Database)
	ui.databaseSSL.SetText(p.SSLMode)
	ui.databaseTLS.SetText(p.TLSMode)
}
func (ui *Window) clearDatabaseProfile() {
	for _, editor := range []*widget.Editor{&ui.databaseName, &ui.databaseHost, &ui.databasePort, &ui.databaseUser, &ui.databasePassword, &ui.databaseSchema, &ui.databaseSSL, &ui.databaseTLS} {
		editor.SetText("")
	}
	ui.databaseKind = dbTypeMySQL
}
func (ui *Window) databaseProfile() (config.DBProfile, error) {
	kind, err := normalizeDBType(ui.databaseKind)
	if err != nil {
		return config.DBProfile{}, err
	}
	port, err := parsePort(ui.databasePort.Text())
	if err != nil {
		return config.DBProfile{}, err
	}
	p := config.DBProfile{Name: ui.databaseName.Text(), Type: kind, Host: ui.databaseHost.Text(), Port: port, User: ui.databaseUser.Text(), Password: ui.databasePassword.Text(), Database: ui.databaseSchema.Text(), SSLMode: ui.databaseSSL.Text(), TLSMode: ui.databaseTLS.Text()}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Host) == "" || p.User == "" || p.Password == "" || p.Database == "" {
		return config.DBProfile{}, fmt.Errorf("database name, host, user, password, and database are required")
	}
	return p, nil
}

func (ui *Window) saveProfile(update func(*config.Store), after func()) {
	if ui.model.Session == nil {
		return
	}
	current, err := ui.model.Session.Profiles()
	if err != nil {
		ui.model.Error = err.Error()
		return
	}
	update(current)
	session := ui.model.Session
	ui.async("Saving encrypted profiles...", func(ctx context.Context) (func(), error) {
		if err := session.SaveProfiles(ctx, current); err != nil {
			return nil, err
		}
		return func() { after(); ui.refreshProfiles() }, nil
	})
}

func replaceSSH(profiles []config.SSHProfile, index int, value config.SSHProfile) []config.SSHProfile {
	if index < 0 || index >= len(profiles) {
		return append(profiles, value)
	}
	profiles[index] = value
	return profiles
}
func replaceS3(profiles []config.S3Profile, index int, value config.S3Profile) []config.S3Profile {
	if index < 0 || index >= len(profiles) {
		return append(profiles, value)
	}
	profiles[index] = value
	return profiles
}
func replaceDB(profiles []config.DBProfile, index int, value config.DBProfile) []config.DBProfile {
	if index < 0 || index >= len(profiles) {
		return append(profiles, value)
	}
	profiles[index] = value
	return profiles
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

func validateVaultCredentials(name, password string) error {
	if strings.TrimSpace(name) == "" || password == "" {
		return fmt.Errorf("vault name and password are required")
	}
	return nil
}

func validateLoginCredentials(name, password string) error {
	return validateVaultCredentials(name, password)
}

func validateRecoveryCredentials(key, name, password string) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(name) == "" || password == "" {
		return fmt.Errorf("recovery key, new vault name, and new vault password are required")
	}
	return nil
}
func formatObjects(objects []storage.Object) string {
	if len(objects) == 0 {
		return "No objects found."
	}
	var b strings.Builder
	for _, object := range objects {
		fmt.Fprintf(&b, "%s  (%d Bytes)\n", object.Key, object.Size)
	}
	return b.String()
}
func formatRows(rows []database.Row) string {
	if len(rows) == 0 {
		return "No rows returned."
	}
	var b strings.Builder
	for i, row := range rows {
		fmt.Fprintf(&b, "%d: ", i+1)
		first := true
		for key, value := range row {
			if !first {
				b.WriteString(" | ")
			}
			first = false
			fmt.Fprintf(&b, "%s=%v", key, value)
		}
		b.WriteByte('\n')
	}
	return b.String()
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
func downloadedTo(path string) string {
	if path == "" {
		return " (preview available in output)"
	}
	return " to " + path
}
