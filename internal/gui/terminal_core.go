package gui

import (
	"fmt"
	"image"
	"strings"
	"sync"

	"gioui.org/io/key"
	terminalte "github.com/rcarmo/go-te/pkg/te"
)

type terminalColor string

const (
	terminalColorDefault terminalColor = "default"
	terminalColorRed     terminalColor = "red"
)

type terminalCell struct {
	Text       string
	Foreground terminalColor
	Background terminalColor
	Bold       bool
	Italics    bool
	Underline  bool
	Reverse    bool
}

type terminalCursor struct {
	Row    int
	Column int
	Hidden bool
}

type terminalFrame struct {
	Width  int
	Height int
	Rows   []string
	Cells  [][]terminalCell
	Cursor terminalCursor
}

type terminalEmulator interface {
	Feed([]byte) error
	Frame() terminalFrame
	Resize(width, height int) error
}

func terminalGridSize(viewport, cell image.Point) image.Point {
	if cell.X < 1 {
		cell.X = 1
	}
	if cell.Y < 1 {
		cell.Y = 1
	}
	width, height := viewport.X/cell.X, viewport.Y/cell.Y
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return image.Point{X: width, Y: height}
}

func prepareTerminalPaste(text string) ([]byte, bool) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	multiline := strings.Contains(normalized, "\n")
	return []byte(strings.ReplaceAll(normalized, "\n", "\r")), multiline
}

func terminalCellAt(position, cell, grid image.Point) image.Point {
	if cell.X < 1 {
		cell.X = 1
	}
	if cell.Y < 1 {
		cell.Y = 1
	}
	if grid.X < 1 || grid.Y < 1 {
		return image.Point{}
	}
	column, row := position.X/cell.X, position.Y/cell.Y
	if column < 0 {
		column = 0
	} else if column >= grid.X {
		column = grid.X - 1
	}
	if row < 0 {
		row = 0
	} else if row >= grid.Y {
		row = grid.Y - 1
	}
	return image.Point{X: column, Y: row}
}

func terminalSelectionText(frame terminalFrame, start, end image.Point) string {
	if frame.Width < 1 || frame.Height < 1 || len(frame.Cells) == 0 {
		return ""
	}
	start = terminalCellAt(start, image.Pt(1, 1), image.Pt(frame.Width, frame.Height))
	end = terminalCellAt(end, image.Pt(1, 1), image.Pt(frame.Width, frame.Height))
	if end.Y < start.Y || end.Y == start.Y && end.X < start.X {
		start, end = end, start
	}

	lines := make([]string, 0, end.Y-start.Y+1)
	for row := start.Y; row <= end.Y && row < len(frame.Cells); row++ {
		from, to := 0, frame.Width
		if row == start.Y {
			from = start.X
		}
		if row == end.Y {
			to = end.X
		}
		if start.Y == end.Y && to < from {
			from, to = to, from
		}
		if from < 0 {
			from = 0
		}
		if to > len(frame.Cells[row]) {
			to = len(frame.Cells[row])
		}
		if to < from {
			to = from
		}
		var line strings.Builder
		for column := from; column < to; column++ {
			line.WriteString(frame.Cells[row][column].Text)
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.Join(lines, "\n")
}

func terminalCellSelected(selection terminalSelection, cell image.Point) bool {
	if !selection.active || selection.start == selection.end {
		return false
	}
	start, end := selection.start, selection.end
	if end.Y < start.Y || end.Y == start.Y && end.X < start.X {
		start, end = end, start
	}
	if cell.Y < start.Y || cell.Y > end.Y {
		return false
	}
	if start.Y == end.Y {
		return cell.Y == start.Y && cell.X >= start.X && cell.X < end.X
	}
	if cell.Y == start.Y {
		return cell.X >= start.X
	}
	if cell.Y == end.Y {
		return cell.X < end.X
	}
	return true
}

func terminalDragSelection(anchor, focus image.Point) terminalSelection {
	if anchor == focus {
		return terminalSelection{}
	}
	if focus.Y > anchor.Y || focus.Y == anchor.Y && focus.X > anchor.X {
		focus.X++
		return terminalSelection{active: true, start: anchor, end: focus}
	}
	anchor.X++
	return terminalSelection{active: true, start: anchor, end: focus}
}

type goTerminalEmulator struct {
	mu     sync.Mutex
	screen *terminalte.HistoryScreen
	stream *terminalte.ByteStream
}

func newTerminalEmulator(width, height int) terminalEmulator {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	screen := terminalte.NewHistoryScreen(width, height, outputMaxLines)
	return &goTerminalEmulator{
		screen: screen,
		stream: terminalte.NewByteStream(screen, false),
	}
}

func (e *goTerminalEmulator) Feed(data []byte) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stream.Feed(data)
}

func (e *goTerminalEmulator) Resize(width, height int) error {
	if e == nil {
		return nil
	}
	if width < 1 || height < 1 {
		return fmt.Errorf("terminal size must be positive: %dx%d", width, height)
	}
	e.mu.Lock()
	e.screen.Resize(height, width)
	e.mu.Unlock()
	return nil
}

func (e *goTerminalEmulator) Frame() terminalFrame {
	if e == nil {
		return terminalFrame{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	frame := terminalFrame{
		Width:  e.screen.Columns,
		Height: e.screen.Lines,
		Rows:   make([]string, e.screen.Lines),
		Cells:  make([][]terminalCell, e.screen.Lines),
		Cursor: terminalCursor{
			Row:    e.screen.Cursor.Row,
			Column: e.screen.Cursor.Col,
			Hidden: e.screen.Cursor.Hidden,
		},
	}
	for row := 0; row < e.screen.Lines; row++ {
		cells := make([]terminalCell, e.screen.Columns)
		var text strings.Builder
		for column := 0; column < e.screen.Columns; column++ {
			cell := terminalte.Cell{Data: " "}
			if row < len(e.screen.Buffer) && column < len(e.screen.Buffer[row]) {
				cell = e.screen.Buffer[row][column]
			}
			if cell.Data == "" {
				cell.Data = " "
			}
			text.WriteString(cell.Data)
			cells[column] = terminalCell{
				Text:       cell.Data,
				Foreground: terminalColorValue(cell.Attr.Fg),
				Background: terminalColorValue(cell.Attr.Bg),
				Bold:       cell.Attr.Bold,
				Italics:    cell.Attr.Italics,
				Underline:  cell.Attr.Underline,
				Reverse:    cell.Attr.Reverse,
			}
		}
		frame.Rows[row] = text.String()
		frame.Cells[row] = cells
	}
	return frame
}

func terminalColorValue(color terminalte.Color) terminalColor {
	if color.Name != "" {
		return terminalColor(color.Name)
	}
	switch color.Mode {
	case terminalte.ColorANSI16:
		return terminalColor(fmt.Sprintf("ansi16:%d", color.Index))
	case terminalte.ColorANSI256:
		return terminalColor(fmt.Sprintf("ansi256:%d", color.Index))
	case terminalte.ColorTrueColor:
		return terminalColor(fmt.Sprintf("truecolor:%s", color.Name))
	default:
		return terminalColorDefault
	}
}

// encodeTerminalKey translates non-text Gio key events into the byte sequences
// expected by an interactive xterm-compatible PTY. Printable text continues
// through widget.Editor EditEvent handling instead of this path.
func encodeTerminalKey(event key.Event) []byte {
	if event.State != key.Press {
		return nil
	}
	if event.Modifiers == key.ModCtrl && len(event.Name) == 1 {
		name := event.Name[0]
		if name >= 'A' && name <= 'Z' {
			return []byte{name & 0x1f}
		}
	}

	sequences := map[key.Name]string{
		key.NameLeftArrow:      "\x1b[D",
		key.NameRightArrow:     "\x1b[C",
		key.NameUpArrow:        "\x1b[A",
		key.NameDownArrow:      "\x1b[B",
		key.NameHome:           "\x1b[H",
		key.NameEnd:            "\x1b[F",
		key.NameReturn:         "\r",
		key.NameEnter:          "\r",
		key.NameTab:            "\t",
		key.NameEscape:         "\x1b",
		key.NameDeleteBackward: "\x7f",
		key.NameDeleteForward:  "\x1b[3~",
		key.NamePageUp:         "\x1b[5~",
		key.NamePageDown:       "\x1b[6~",
		key.NameF1:             "\x1b[11~",
		key.NameF2:             "\x1b[12~",
		key.NameF3:             "\x1b[13~",
		key.NameF4:             "\x1b[14~",
		key.NameF5:             "\x1b[15~",
		key.NameF6:             "\x1b[17~",
		key.NameF7:             "\x1b[18~",
		key.NameF8:             "\x1b[19~",
		key.NameF9:             "\x1b[20~",
		key.NameF10:            "\x1b[21~",
		key.NameF11:            "\x1b[23~",
		key.NameF12:            "\x1b[24~",
	}
	sequence, ok := sequences[event.Name]
	if !ok {
		return nil
	}
	return []byte(sequence)
}
