package gui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"
)

func TestTerminalEmulatorRendersVisibleTextAndCursor(t *testing.T) {
	emulator := newTerminalEmulator(12, 3)
	if err := emulator.Feed([]byte("hello")); err != nil {
		t.Fatalf("feed terminal text: %v", err)
	}

	frame := emulator.Frame()
	if got := strings.TrimRight(frame.Rows[0], " "); got != "hello" {
		t.Fatalf("first row = %q, want %q", got, "hello")
	}
	if frame.Width != 12 || frame.Height != 3 {
		t.Fatalf("frame size = %dx%d, want 12x3", frame.Width, frame.Height)
	}
	if frame.Cursor.Row != 0 || frame.Cursor.Column != 5 {
		t.Fatalf("cursor = %+v, want row 0 column 5", frame.Cursor)
	}
}

func TestTerminalEmulatorPreservesANSIStyles(t *testing.T) {
	emulator := newTerminalEmulator(12, 3)
	if err := emulator.Feed([]byte("\x1b[31mred\x1b[0m plain")); err != nil {
		t.Fatalf("feed styled terminal text: %v", err)
	}

	frame := emulator.Frame()
	if got := strings.TrimRight(frame.Rows[0], " "); got != "red plain" {
		t.Fatalf("styled row = %q, want %q", got, "red plain")
	}
	if frame.Cells[0][0].Foreground != terminalColorRed {
		t.Fatalf("red cell foreground = %q, want %q", frame.Cells[0][0].Foreground, terminalColorRed)
	}
	if frame.Cells[0][4].Foreground != terminalColorDefault {
		t.Fatalf("reset cell foreground = %q, want %q", frame.Cells[0][4].Foreground, terminalColorDefault)
	}
}

func TestTerminalEmulatorRestoresAlternateScreen(t *testing.T) {
	emulator := newTerminalEmulator(12, 3)
	if err := emulator.Feed([]byte("main\x1b[?1049h\x1b[2Jalternate")); err != nil {
		t.Fatalf("feed alternate screen: %v", err)
	}
	if got := strings.TrimRight(emulator.Frame().Rows[0], " "); got != "alternate" {
		t.Fatalf("alternate row = %q, want %q", got, "alternate")
	}

	if err := emulator.Feed([]byte("\x1b[?1049l")); err != nil {
		t.Fatalf("leave alternate screen: %v", err)
	}
	if got := strings.TrimRight(emulator.Frame().Rows[0], " "); got != "main" {
		t.Fatalf("restored row = %q, want %q", got, "main")
	}
}

func TestTerminalEmulatorResizeUpdatesFrameDimensions(t *testing.T) {
	emulator := newTerminalEmulator(12, 3)
	if err := emulator.Resize(20, 5); err != nil {
		t.Fatalf("resize terminal: %v", err)
	}

	frame := emulator.Frame()
	if frame.Width != 20 || frame.Height != 5 {
		t.Fatalf("resized frame = %dx%d, want 20x5", frame.Width, frame.Height)
	}
	if len(frame.Rows) != 5 || len(frame.Cells) != 5 {
		t.Fatalf("resized rows/cells = %d/%d, want 5/5", len(frame.Rows), len(frame.Cells))
	}
}

func TestSSHTabsKeepIndependentTerminalFrames(t *testing.T) {
	store := sshTabStore{}
	first := store.open(testSSHHost("first", "first"))
	second := store.open(testSSHHost("second", "second"))

	if err := first.emulator.Feed([]byte("first")); err != nil {
		t.Fatalf("feed first tab: %v", err)
	}
	if err := second.emulator.Feed([]byte("second")); err != nil {
		t.Fatalf("feed second tab: %v", err)
	}

	if got := strings.TrimRight(first.emulator.Frame().Rows[0], " "); got != "first" {
		t.Fatalf("first tab frame = %q, want %q", got, "first")
	}
	if got := strings.TrimRight(second.emulator.Frame().Rows[0], " "); got != "second" {
		t.Fatalf("second tab frame = %q, want %q", got, "second")
	}
}

func TestEncodeTerminalKeySendsControlKeysToRemotePTY(t *testing.T) {
	tests := []struct {
		name  string
		event key.Event
		want  string
	}{
		{name: "ctrl c", event: key.Event{Name: "C", Modifiers: key.ModCtrl, State: key.Press}, want: "\x03"},
		{name: "ctrl d", event: key.Event{Name: "D", Modifiers: key.ModCtrl, State: key.Press}, want: "\x04"},
		{name: "left", event: key.Event{Name: key.NameLeftArrow, State: key.Press}, want: "\x1b[D"},
		{name: "right", event: key.Event{Name: key.NameRightArrow, State: key.Press}, want: "\x1b[C"},
		{name: "up", event: key.Event{Name: key.NameUpArrow, State: key.Press}, want: "\x1b[A"},
		{name: "down", event: key.Event{Name: key.NameDownArrow, State: key.Press}, want: "\x1b[B"},
		{name: "home", event: key.Event{Name: key.NameHome, State: key.Press}, want: "\x1b[H"},
		{name: "end", event: key.Event{Name: key.NameEnd, State: key.Press}, want: "\x1b[F"},
		{name: "return", event: key.Event{Name: key.NameReturn, State: key.Press}, want: "\r"},
		{name: "enter", event: key.Event{Name: key.NameEnter, State: key.Press}, want: "\r"},
		{name: "tab", event: key.Event{Name: key.NameTab, State: key.Press}, want: "\t"},
		{name: "escape", event: key.Event{Name: key.NameEscape, State: key.Press}, want: "\x1b"},
		{name: "backspace", event: key.Event{Name: key.NameDeleteBackward, State: key.Press}, want: "\x7f"},
		{name: "delete", event: key.Event{Name: key.NameDeleteForward, State: key.Press}, want: "\x1b[3~"},
		{name: "page up", event: key.Event{Name: key.NamePageUp, State: key.Press}, want: "\x1b[5~"},
		{name: "page down", event: key.Event{Name: key.NamePageDown, State: key.Press}, want: "\x1b[6~"},
		{name: "f1", event: key.Event{Name: key.NameF1, State: key.Press}, want: "\x1b[11~"},
		{name: "f12", event: key.Event{Name: key.NameF12, State: key.Press}, want: "\x1b[24~"},
		{name: "release is ignored", event: key.Event{Name: key.NameLeftArrow, State: key.Release}, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := string(encodeTerminalKey(test.event)); got != test.want {
				t.Fatalf("encoded key = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEncodeTerminalKeyDoesNotCaptureUnmodifiedPrintableKeys(t *testing.T) {
	if got := encodeTerminalKey(key.Event{Name: "A", State: key.Press}); got != nil {
		t.Fatalf("unmodified printable key = %q, want nil", got)
	}
	if got := encodeTerminalKey(key.Event{Name: "C", Modifiers: key.ModCtrl | key.ModShift, State: key.Press}); got != nil {
		t.Fatalf("clipboard shortcut candidate = %q, want nil", got)
	}
}

func TestSendSSHTabKeyWritesOnlyEncodedKeysToRequestedPTY(t *testing.T) {
	ui := NewWindow(nil)
	first := ui.sshTabs.open(testSSHHost("host-1", "web"))
	second := ui.sshTabs.open(testSSHHost("host-1", "web"))
	firstPTY := &testSSHWrites{}
	secondPTY := &testSSHWrites{}
	first.session = &sshTabSession{pty: firstPTY}
	second.session = &sshTabSession{pty: secondPTY}
	first.State = sshTabConnected
	second.State = sshTabConnected

	if !ui.sendSSHTabKey(first.ID, key.Event{Name: key.NameLeftArrow, State: key.Press}) {
		t.Fatal("supported terminal key should be handled")
	}
	if firstPTY.writes[0] != "\x1b[D" {
		t.Fatalf("first PTY writes = %q, want %q", firstPTY.writes, "\x1b[D")
	}
	if len(secondPTY.writes) != 0 {
		t.Fatalf("second PTY writes = %q, want no writes", secondPTY.writes)
	}
	if ui.sendSSHTabKey(first.ID, key.Event{Name: "A", State: key.Press}) {
		t.Fatal("printable text key should remain with the text editor")
	}
}

type testSSHResizable struct {
	testSSHWrites
	sizes []image.Point
}

func (r *testSSHResizable) Resize(width, height int) error {
	r.sizes = append(r.sizes, image.Point{X: width, Y: height})
	return nil
}

func TestResizeSSHTabUpdatesPTYAndIndependentFrame(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.sshTabs.open(testSSHHost("host-1", "web"))
	pty := &testSSHResizable{}
	tab.session = &sshTabSession{pty: pty}
	tab.State = sshTabConnected

	if !ui.resizeSSHTab(tab.ID, image.Point{X: 80, Y: 24}) {
		t.Fatal("connected tab should resize")
	}
	frame := tab.emulator.Frame()
	if frame.Width != 80 || frame.Height != 24 {
		t.Fatalf("tab frame size = %dx%d, want 80x24", frame.Width, frame.Height)
	}
	if len(pty.sizes) != 1 || pty.sizes[0] != (image.Point{X: 80, Y: 24}) {
		t.Fatalf("PTY resize calls = %+v, want one 80x24 call", pty.sizes)
	}
	if !ui.resizeSSHTab(tab.ID, image.Point{X: 80, Y: 24}) {
		t.Fatal("repeating the current tab size should remain successful")
	}
	if len(pty.sizes) != 1 {
		t.Fatalf("repeated resize sent extra PTY calls = %+v", pty.sizes)
	}
}

func TestTerminalGridSizeUsesAvailableViewportAndMinimumCell(t *testing.T) {
	if got := terminalGridSize(image.Point{X: 800, Y: 400}, image.Point{X: 8, Y: 20}); got != (image.Point{X: 100, Y: 20}) {
		t.Fatalf("terminal grid = %+v, want 100x20", got)
	}
	if got := terminalGridSize(image.Point{X: 3, Y: 4}, image.Point{X: 8, Y: 20}); got != (image.Point{X: 1, Y: 1}) {
		t.Fatalf("small terminal grid = %+v, want 1x1", got)
	}
}

func TestPrepareTerminalPasteNormalizesNewlinesAndFlagsMultipleCommands(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		want          string
		wantMultiline bool
	}{
		{name: "single command", input: "pwd", want: "pwd"},
		{name: "windows lines", input: "pwd\r\nwhoami", want: "pwd\rwhoami", wantMultiline: true},
		{name: "unix lines", input: "pwd\nwhoami\n", want: "pwd\rwhoami\r", wantMultiline: true},
		{name: "legacy carriage returns", input: "pwd\rwhoami", want: "pwd\rwhoami", wantMultiline: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, multiline := prepareTerminalPaste(test.input)
			if string(got) != test.want || multiline != test.wantMultiline {
				t.Fatalf("prepared paste = %q multiline=%v, want %q multiline=%v", got, multiline, test.want, test.wantMultiline)
			}
		})
	}
}

func TestTerminalCellAtClampsPointerToVisibleGrid(t *testing.T) {
	cell := image.Point{X: 8, Y: 20}
	grid := image.Point{X: 80, Y: 24}
	if got := terminalCellAt(image.Pt(17, 41), cell, grid); got != (image.Point{X: 2, Y: 2}) {
		t.Fatalf("cell at pointer = %+v, want 2,2", got)
	}
	if got := terminalCellAt(image.Pt(-20, 9999), cell, grid); got != (image.Point{X: 0, Y: 23}) {
		t.Fatalf("clamped cell = %+v, want 0,23", got)
	}
}

func TestTerminalSelectionTextSupportsForwardAndReverseCrossRowRanges(t *testing.T) {
	emulator := newTerminalEmulator(8, 3)
	if err := emulator.Feed([]byte("hello\r\nworld")); err != nil {
		t.Fatalf("feed selection frame: %v", err)
	}
	frame := emulator.Frame()
	start := image.Point{X: 1, Y: 0}
	end := image.Point{X: 3, Y: 1}
	want := "ello\nwor"
	if got := terminalSelectionText(frame, start, end); got != want {
		t.Fatalf("forward selection = %q, want %q", got, want)
	}
	if got := terminalSelectionText(frame, end, start); got != want {
		t.Fatalf("reverse selection = %q, want %q", got, want)
	}
}

func TestTerminalCellSelectedMatchesForwardAndReverseExclusiveRange(t *testing.T) {
	selection := terminalSelection{active: true, start: image.Pt(1, 0), end: image.Pt(3, 1)}
	cases := []struct {
		cell image.Point
		want bool
	}{
		{cell: image.Pt(0, 0), want: false},
		{cell: image.Pt(1, 0), want: true},
		{cell: image.Pt(4, 0), want: true},
		{cell: image.Pt(0, 1), want: true},
		{cell: image.Pt(2, 1), want: true},
		{cell: image.Pt(3, 1), want: false},
	}
	for _, tc := range cases {
		if got := terminalCellSelected(selection, tc.cell); got != tc.want {
			t.Errorf("terminalCellSelected(%v) = %t, want %t", tc.cell, got, tc.want)
		}
	}
	reverse := terminalSelection{active: true, start: selection.end, end: selection.start}
	for _, tc := range cases {
		if got := terminalCellSelected(reverse, tc.cell); got != tc.want {
			t.Errorf("reverse terminalCellSelected(%v) = %t, want %t", tc.cell, got, tc.want)
		}
	}
}

func TestTerminalDragSelectionIncludesBothPointerCells(t *testing.T) {
	forward := terminalDragSelection(image.Pt(1, 0), image.Pt(3, 0))
	if !terminalCellSelected(forward, image.Pt(1, 0)) || !terminalCellSelected(forward, image.Pt(3, 0)) {
		t.Fatal("forward drag must include both endpoint cells")
	}
	if terminalCellSelected(forward, image.Pt(4, 0)) {
		t.Fatal("forward drag must stop after the pointer cell")
	}

	reverse := terminalDragSelection(image.Pt(3, 0), image.Pt(1, 0))
	if !terminalCellSelected(reverse, image.Pt(1, 0)) || !terminalCellSelected(reverse, image.Pt(3, 0)) {
		t.Fatal("reverse drag must include both endpoint cells")
	}
	if terminalCellSelected(reverse, image.Pt(0, 0)) {
		t.Fatal("reverse drag must stop before the pointer cell")
	}

	if selection := terminalDragSelection(image.Pt(2, 0), image.Pt(2, 0)); selection.active {
		t.Fatal("a click without dragging must not select terminal text")
	}
}
