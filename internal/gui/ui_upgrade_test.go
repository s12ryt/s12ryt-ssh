package gui

import (
	"io"
	"strings"
	"testing"

	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/storage"

	"gioui.org/widget"
)

type fakeTerminal struct {
	written []byte
	closed  bool
}

func (f *fakeTerminal) Read(p []byte) (int, error) { return 0, io.EOF }
func (f *fakeTerminal) Write(p []byte) (int, error) {
	f.written = append(f.written, p...)
	return len(p), nil
}
func (f *fakeTerminal) Close() error {
	f.closed = true
	return nil
}

func TestStripANSIRemovesTerminalEscapeSequences(t *testing.T) {
	cases := map[string]string{
		"\x1b[31mred\x1b[0m plain":  "red plain",
		"\x1b[2J\x1b[Hcleared":      "cleared",
		"\x1b]0;window title\x07ok": "ok",
		"no escapes":                "no escapes",
		"\x1b[?25hvisible\x1b[?25l": "visible",
	}
	for input, want := range cases {
		if got := stripANSI(input); got != want {
			t.Fatalf("stripANSI(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAppendTerminalFiltersANSIAndCapsBufferSize(t *testing.T) {
	if got := appendTerminalFilter("", "\x1b[31mred\x1b[0m\n", 1000); got != "red\n" {
		t.Fatalf("appendTerminalFilter filtered = %q, want %q", got, "red\n")
	}
	current := strings.Repeat("a", 100)
	got := appendTerminalFilter(current, "tail", 50)
	if len([]rune(got)) != 50 {
		t.Fatalf("appendTerminalFilter length = %d runes, want 50", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "tail") {
		t.Fatalf("appendTerminalFilter lost the newest output: %q", got)
	}
}

func TestTailLinesKeepsMostRecentLines(t *testing.T) {
	if got := tailLines("1\n2\n3\n4\n5\n", 3); got != "3\n4\n5\n" {
		t.Fatalf("tailLines truncated = %q, want %q", got, "3\n4\n5\n")
	}
	if got := tailLines("1\n2\n", 5); got != "1\n2\n" {
		t.Fatalf("tailLines short = %q, want unchanged", got)
	}
	if got := tailLines("4\n5", 3); got != "4\n5" {
		t.Fatalf("tailLines without trailing newline = %q, want %q", got, "4\n5")
	}
}

func TestConsumeSubmitDetectsOnlySubmitEvents(t *testing.T) {
	if !consumeSubmit([]widget.EditorEvent{widget.SubmitEvent{}}) {
		t.Fatal("consumeSubmit must detect a submit event")
	}
	if consumeSubmit([]widget.EditorEvent{widget.ChangeEvent{}}) {
		t.Fatal("consumeSubmit must ignore change events")
	}
	if consumeSubmit(nil) {
		t.Fatal("consumeSubmit must ignore empty events")
	}
}

func TestConfirmationRequestAcceptCancelLifecycle(t *testing.T) {
	var c confirmation
	runs := 0
	c.request("Delete object", "Are you sure?", func() { runs++ })
	if !c.active || c.title != "Delete object" || c.message != "Are you sure?" {
		t.Fatalf("confirmation request state = active %v title %q message %q", c.active, c.title, c.message)
	}
	c.cancel()
	if c.active || runs != 0 {
		t.Fatalf("cancel must close without running: active %v runs %d", c.active, runs)
	}
	c.accept()
	if runs != 0 {
		t.Fatal("accept after cancel must not run the action")
	}

	c.request("first", "message", func() { runs += 1 })
	c.request("second", "message", func() { runs += 10 })
	c.accept()
	if runs != 10 {
		t.Fatalf("latest request must win: runs = %d, want 10", runs)
	}
	if c.active {
		t.Fatal("accept must deactivate the confirmation")
	}
	runs = 0
	c.accept()
	if runs != 0 {
		t.Fatal("accept twice must not re-run the action")
	}
}

func TestNormalizeDBTypeCanonicalizesAliasesAndRejectsUnsupported(t *testing.T) {
	for input, want := range map[string]string{
		"mysql":      "mysql",
		"MySQL":      "mysql",
		" MySQL ":    "mysql",
		"postgres":   "postgres",
		"PostgreSQL": "postgres",
		"pg":         "postgres",
		" PG":        "postgres",
	} {
		got, err := normalizeDBType(input)
		if err != nil || got != want {
			t.Fatalf("normalizeDBType(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "   ", "sqlite", "oracle", "mariadb"} {
		if _, err := normalizeDBType(input); err == nil {
			t.Fatalf("normalizeDBType(%q) must fail", input)
		}
	}
}

func TestDBTypeChoicesCoverSupportedDrivers(t *testing.T) {
	choices := dbTypeChoices()
	if len(choices) != 2 || choices[0] != "MySQL" || choices[1] != "PostgreSQL" {
		t.Fatalf("dbTypeChoices() = %v, want [MySQL PostgreSQL]", choices)
	}
}

func TestPasswordRevealToggleAndMask(t *testing.T) {
	var reveal passwordReveal
	if mask := reveal.mask(); mask != '•' {
		t.Fatalf("default mask = %q, want bullet", mask)
	}
	reveal.toggle()
	if mask := reveal.mask(); mask != 0 {
		t.Fatalf("revealed mask = %v, want 0", mask)
	}
	reveal.toggle()
	if mask := reveal.mask(); mask != '•' {
		t.Fatalf("hidden mask = %q, want bullet", mask)
	}
}

func TestButtonColorsSignalBusyAndDanger(t *testing.T) {
	bg, fg := buttonColors(false, false)
	if bg != colorSurface2 || fg != colorText {
		t.Fatalf("normal colors = %v %v", bg, fg)
	}
	bg, fg = buttonColors(false, true)
	if bg != colorDanger || fg != colorBackground {
		t.Fatalf("danger colors = %v %v", bg, fg)
	}
	bg, _ = buttonColors(true, true)
	if bg != colorSurface2 {
		t.Fatalf("busy must override danger styling: bg = %v", bg)
	}
}

func TestRequireObjectKeyAndSQLStatementRejectBlank(t *testing.T) {
	if err := requireObjectKey(""); err == nil || err.Error() != "object key is required" {
		t.Fatalf("empty object key error = %v", err)
	}
	if err := requireObjectKey("   "); err == nil {
		t.Fatal("blank object key must be rejected")
	}
	if err := requireSQLStatement(""); err == nil || err.Error() != "SQL statement is required" {
		t.Fatalf("empty statement error = %v", err)
	}
	if err := requireSQLStatement("   "); err == nil {
		t.Fatal("blank statement must be rejected")
	}
	if err := requireObjectKey("backup.txt"); err != nil {
		t.Fatalf("valid key error = %v", err)
	}
	if err := requireSQLStatement("SELECT 1"); err != nil {
		t.Fatalf("valid statement error = %v", err)
	}
}

func TestSendTerminalInputWritesLineAndClearsInput(t *testing.T) {
	ui := NewWindow(nil)
	terminal := &fakeTerminal{}
	ui.terminal = terminal
	ui.terminalInput.SetText("ls -la")

	if !ui.sendTerminalInput() {
		t.Fatal("sendTerminalInput with a live terminal must succeed")
	}
	if string(terminal.written) != "ls -la\n" {
		t.Fatalf("terminal written = %q, want %q", terminal.written, "ls -la\n")
	}
	if ui.terminalInput.Text() != "" {
		t.Fatal("sendTerminalInput must clear the input editor")
	}

	ui.sendTerminalInput()
	if len(terminal.written) != len("ls -la\n") {
		t.Fatalf("empty input must not write again: %q", terminal.written)
	}

	ui.terminal = nil
	if ui.sendTerminalInput() {
		t.Fatal("sendTerminalInput without a terminal must fail")
	}
	if ui.model.Error != "SSH terminal is not connected" {
		t.Fatalf("missing terminal error = %q", ui.model.Error)
	}
}

func TestSelectObjectFillsObjectKeyEditor(t *testing.T) {
	ui := NewWindow(nil)
	ui.objects = []storage.Object{{Key: "a.txt", Size: 1}, {Key: "dir/b.txt", Size: 2}}
	ui.selectObject(1)
	if ui.storageKey.Text() != "dir/b.txt" {
		t.Fatalf("object key after select = %q", ui.storageKey.Text())
	}
	ui.selectObject(-1)
	ui.selectObject(len(ui.objects))
	if ui.storageKey.Text() != "dir/b.txt" {
		t.Fatalf("out-of-range select must not change the key: %q", ui.storageKey.Text())
	}
}

func TestSelectRemoteObjectFillsObjectKeyEditor(t *testing.T) {
	ui := NewWindow(nil)
	ui.remoteObjects = []remote.S3Object{{Key: "x.txt"}, {Key: "y.txt"}}
	ui.selectRemoteObject(1)
	if ui.storageKey.Text() != "y.txt" {
		t.Fatalf("remote object key after select = %q", ui.storageKey.Text())
	}
	ui.selectRemoteObject(-1)
	ui.selectRemoteObject(99)
	if ui.storageKey.Text() != "y.txt" {
		t.Fatalf("out-of-range remote select must not change the key: %q", ui.storageKey.Text())
	}
}

func TestTryActionsGuardBusyAndValidation(t *testing.T) {
	ui := NewWindow(nil)

	ui.busy = true
	if ui.trySignIn() || ui.tryCreateVault() || ui.tryRotateRecovery() || ui.tryRemoteSignIn() || ui.trySSHConnect() {
		t.Fatal("try actions must be rejected while busy")
	}
	if ui.terminalCtx != nil {
		t.Fatal("busy trySSHConnect must not create a terminal context")
	}
	ui.busy = false

	if ui.trySignIn() {
		t.Fatal("empty sign-in credentials must be rejected")
	}
	if ui.model.Error == "" || ui.busy {
		t.Fatalf("sign-in validation error = %q busy %v", ui.model.Error, ui.busy)
	}

	if ui.tryCreateVault() {
		t.Fatal("empty vault credentials must be rejected")
	}
	if ui.model.Error == "" || ui.busy {
		t.Fatalf("vault validation error = %q busy %v", ui.model.Error, ui.busy)
	}

	if ui.tryRotateRecovery() {
		t.Fatal("empty recovery credentials must be rejected")
	}
	if ui.model.Error == "" || ui.busy {
		t.Fatalf("recovery validation error = %q busy %v", ui.model.Error, ui.busy)
	}

	ui.remoteURL.SetText("https://auth.example.com")
	ui.remoteUsername.SetText("alice")
	ui.remotePassword.SetText("password")
	if ui.tryRemoteSignIn() {
		t.Fatal("missing remote service must be rejected")
	}
	if ui.model.Error != "Remote authentication service is unavailable" {
		t.Fatalf("remote service error = %q", ui.model.Error)
	}

	if ui.trySSHConnect() {
		t.Fatal("invalid SSH profile must be rejected")
	}
	if ui.model.Error == "" || ui.terminalCtx != nil {
		t.Fatalf("ssh validation error = %q ctx %v", ui.model.Error, ui.terminalCtx)
	}
}

func TestObjectsHeaderCountsObjects(t *testing.T) {
	ui := NewWindow(nil)
	if got := ui.objectsHeader(0); got != "No objects found." {
		t.Fatalf("empty objects header = %q", got)
	}
	if got := ui.objectsHeader(3); got != "3 objects" {
		t.Fatalf("objects header = %q, want %q", got, "3 objects")
	}
	ui.language = "zh-TW"
	if got := ui.objectsHeader(3); !strings.Contains(got, "物件") {
		t.Fatalf("traditional chinese objects header = %q", got)
	}
}

func TestRequestConfirmBlocksWhileBusy(t *testing.T) {
	ui := NewWindow(nil)
	if ui.setupDBKind != dbTypeMySQL || ui.databaseKind != dbTypeMySQL {
		t.Fatalf("default db kinds = %q/%q, want mysql", ui.setupDBKind, ui.databaseKind)
	}

	ui.requestConfirm("Delete object", "This permanently deletes the object. This action cannot be undone.", func() {})
	if !ui.confirm.active {
		t.Fatal("requestConfirm should open the confirmation modal when idle")
	}
	ui.confirm.cancel()

	ui.busy = true
	ui.requestConfirm("Delete object", "This permanently deletes the object. This action cannot be undone.", func() {})
	if ui.confirm.active {
		t.Fatal("requestConfirm must not open the confirmation modal while busy")
	}
	ui.busy = false
}

func TestUseStackedRowSwitchesOnNarrowWidths(t *testing.T) {
	if !useStackedRow(320) {
		t.Fatal("narrow form width must stack editor rows")
	}
	if useStackedRow(editorRowStackBelowDp) {
		t.Fatal("width at threshold must stay side-by-side")
	}
	if !useStackedRow(editorRowStackBelowDp - 1) {
		t.Fatal("width below threshold must stack")
	}
	if useStackedRow(1280) {
		t.Fatal("wide form width must stay side-by-side")
	}
}

func TestApplyDatabaseKindNormalizesProfileTypes(t *testing.T) {
	if got := applyDatabaseKind(dbTypeMySQL, "PostgreSQL"); got != dbTypePostgres {
		t.Fatalf("applyDatabaseKind = %q, want %q", got, dbTypePostgres)
	}
	if got := applyDatabaseKind(dbTypePostgres, "pg"); got != dbTypePostgres {
		t.Fatalf("applyDatabaseKind = %q, want %q", got, dbTypePostgres)
	}
	if got := applyDatabaseKind(dbTypeMySQL, "MYSQL"); got != dbTypeMySQL {
		t.Fatalf("applyDatabaseKind = %q, want %q", got, dbTypeMySQL)
	}
	if got := applyDatabaseKind(dbTypePostgres, "sqlite"); got != dbTypePostgres {
		t.Fatalf("unsupported legacy type must keep current kind, got %q", got)
	}
}

func TestIsSecretHintDetectsSecretFields(t *testing.T) {
	for _, label := range []string{"Password", "Vault password", "Secret key", "Key passphrase"} {
		if !isSecretHint(label) {
			t.Fatalf("isSecretHint(%q) = false, want true", label)
		}
	}
	for _, label := range []string{"Access key", "Host", "User", "Object key"} {
		if isSecretHint(label) {
			t.Fatalf("isSecretHint(%q) = true, want false", label)
		}
	}
}

func TestUIUpgradeStringsTranslateToTraditionalChinese(t *testing.T) {
	ui := NewWindow(nil)
	ui.language = "zh-TW"
	sources := []string{
		"Cancel",
		"Confirm",
		"Delete object",
		"This permanently deletes the object. This action cannot be undone.",
		"Execute SQL statement",
		"This runs a statement that can modify data. Continue?",
		"Show",
		"Hide",
		"Copy recovery key",
		"Recovery key copied to clipboard.",
		"No profiles yet. Create one below.",
		"SQL statement is required",
		"SSH terminal is not connected",
		"%d objects",
		"Signing out...",
		"database type must be MySQL or PostgreSQL",
	}
	for _, source := range sources {
		if got := ui.text(source); got == source {
			t.Fatalf("missing Traditional Chinese translation for %q", source)
		}
	}
	if got := ui.text("object key is required"); got == "object key is required" {
		t.Fatal("lowercase validation message must also translate")
	}
}
