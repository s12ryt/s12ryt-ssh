package gui

import (
	"context"
	"image/color"
	"strings"

	"gioui.org/widget"
)

// ptyTerminal decouples interactive terminal handling from the SSH client type
// so input dispatch stays testable without a live connection.
type ptyTerminal interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// terminalMaxRunes bounds the interactive terminal scrollback buffer.
const terminalMaxRunes = 65536

// outputMaxLines bounds operation output panels to their most recent lines.
const outputMaxLines = 1000

// stripANSI removes CSI and OSC escape sequences from terminal output so raw
// control bytes never reach the visible transcript.
func stripANSI(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		c := text[i]
		if c != 0x1b {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(text) {
			break
		}
		switch kind := text[i+1]; kind {
		case '[':
			i += 2
			for i < len(text) && (text[i] < 0x40 || text[i] > 0x7e) {
				i++
			}
			i++
		case ']':
			i += 2
			for i < len(text) {
				if text[i] == 0x07 {
					i++
					break
				}
				if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			i += 2
		}
	}
	return b.String()
}

// appendTerminalFilter appends filtered terminal output and keeps at most
// maxRunes of the most recent content.
func appendTerminalFilter(current, incoming string, maxRunes int) string {
	next := current + stripANSI(incoming)
	runes := []rune(next)
	if len(runes) <= maxRunes {
		return next
	}
	return string(runes[len(runes)-maxRunes:])
}

// tailLines keeps the last maxLines lines, preserving a missing trailing newline.
func tailLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	trailingNewline := strings.HasSuffix(text, "\n")
	if trailingNewline {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= maxLines {
		return text
	}
	kept := strings.Join(lines[len(lines)-maxLines:], "\n")
	if trailingNewline {
		kept += "\n"
	}
	return kept
}

// consumeSubmit reports whether an editor event batch contains a submit event.
func consumeSubmit(events []widget.EditorEvent) bool {
	for _, event := range events {
		if _, ok := event.(widget.SubmitEvent); ok {
			return true
		}
	}
	return false
}

// confirmation is the modal state for destructive-action confirmation.
type confirmation struct {
	active  bool
	title   string
	message string
	action  func()
}

func (c *confirmation) request(title, message string, action func()) {
	c.active = true
	c.title = title
	c.message = message
	c.action = action
}

func (c *confirmation) cancel() {
	c.active = false
	c.action = nil
}

func (c *confirmation) accept() {
	if !c.active || c.action == nil {
		return
	}
	action := c.action
	c.cancel()
	action()
}

// editorRowStackBelowDp is the form width under which paired editor fields
// stack vertically instead of squeezing each other out of view.
const editorRowStackBelowDp = 640

// useStackedRow reports whether a form of the given width in dp should stack
// its paired fields vertically.
func useStackedRow(widthDp int) bool {
	return widthDp < editorRowStackBelowDp
}

// isSecretHint reports whether an editor label describes a secret input that
// should be masked by default.
func isSecretHint(label string) bool {
	lower := strings.ToLower(label)
	return strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "passphrase")
}

// passwordReveal tracks the show/hide state of a secret editor.
type passwordReveal struct {
	shown bool
}

func (p *passwordReveal) toggle() { p.shown = !p.shown }

func (p *passwordReveal) mask() rune {
	if p.shown {
		return 0
	}
	return '•'
}

// buttonColors picks action-button colors: busy overrides danger so blocked
// actions never look clickable.
func buttonColors(busy, danger bool) (background, foreground color.NRGBA) {
	switch {
	case busy:
		return colorSurface2, colorMuted
	case danger:
		return colorDanger, colorBackground
	default:
		return colorSurface2, colorText
	}
}

// sendTerminalInput writes the current input line to the live terminal.
func (ui *Window) sendTerminalInput() bool {
	if ui.terminal == nil {
		ui.model.Error = "SSH terminal is not connected"
		return false
	}
	text := ui.terminalInput.Text()
	if text == "" {
		return true
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if _, err := ui.terminal.Write([]byte(text)); err != nil {
		ui.model.Error = err.Error()
		return false
	}
	ui.terminalInput.SetText("")
	return true
}

// editorReveal pairs a secret editor's mask state with its show/hide toggle.
type editorReveal struct {
	state  passwordReveal
	button widget.Clickable
}

// revealLabel picks the accessible label for a secret editor's toggle.
func revealLabel(shown bool) string {
	if shown {
		return "Hide"
	}
	return "Show"
}

// monoTypeface is the monospace face used for terminal and SQL output.
const monoTypeface = "Go Mono"

// tryRemoteSignIn validates and starts remote authentication sign-in.
func (ui *Window) tryRemoteSignIn() bool {
	if ui.busy {
		return false
	}
	rawURL := strings.TrimSpace(ui.remoteURL.Text())
	username := strings.TrimSpace(ui.remoteUsername.Text())
	password := ui.remotePassword.Text()
	if err := validateRemoteCredentials(rawURL, username, password); err != nil {
		ui.model.Error = err.Error()
		return false
	}
	if ui.model.RemoteService == nil {
		ui.model.Error = "Remote authentication service is unavailable"
		return false
	}
	service := ui.model.RemoteService
	ui.async("Signing in to authentication service...", func(ctx context.Context) (func(), error) {
		session, err := service.Login(ctx, rawURL, username, password)
		if err != nil {
			return nil, err
		}
		overview, err := session.ResourcesOverview(ctx)
		if err != nil {
			_ = session.Logout(ctx)
			return nil, err
		}
		return func() {
			ui.remotePassword.SetText("")
			ui.activateRemoteSession(session, overview.SSHEnabled)
		}, nil
	})
	return true
}
