//go:build windows

package gui

import (
	"io"
	"strconv"
	"strings"

	"github.com/UserExistsError/conpty"
)

type conPTYTerminal struct {
	pty *conpty.ConPty
}

func localShellUsesConPTY() bool { return true }

func startLocalShell(command string, args []string) (ptyTerminal, error) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteWindowsArgument(command))
	for _, arg := range args {
		parts = append(parts, quoteWindowsArgument(arg))
	}
	pty, err := conpty.Start(strings.Join(parts, " "))
	if err != nil {
		return nil, err
	}
	return &conPTYTerminal{pty: pty}, nil
}

func quoteWindowsArgument(value string) string {
	return strconv.Quote(value)
}

func (terminal *conPTYTerminal) Read(p []byte) (int, error) {
	if terminal == nil || terminal.pty == nil {
		return 0, io.ErrClosedPipe
	}
	return terminal.pty.Read(p)
}

func (terminal *conPTYTerminal) Write(p []byte) (int, error) {
	if terminal == nil || terminal.pty == nil {
		return 0, io.ErrClosedPipe
	}
	return terminal.pty.Write(p)
}

func (terminal *conPTYTerminal) Resize(width, height int) error {
	if terminal == nil || terminal.pty == nil {
		return io.ErrClosedPipe
	}
	return terminal.pty.Resize(width, height)
}

func (terminal *conPTYTerminal) Close() error {
	if terminal == nil || terminal.pty == nil {
		return nil
	}
	return terminal.pty.Close()
}
