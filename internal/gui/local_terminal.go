//go:build !windows

package gui

import (
	"io"
	"os/exec"
	"sync"
)

type localShellTerminal struct {
	command  *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	waitDone chan struct{}
	close    sync.Once
}

func startLocalShell(command string, args []string) (ptyTerminal, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	terminal := &localShellTerminal{
		command:  cmd,
		stdin:    stdin,
		stdout:   stdout,
		waitDone: make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(terminal.waitDone)
	}()
	return terminal, nil
}

func (terminal *localShellTerminal) Read(p []byte) (int, error) {
	return terminal.stdout.Read(p)
}

func (terminal *localShellTerminal) Write(p []byte) (int, error) {
	return terminal.stdin.Write(p)
}

func (terminal *localShellTerminal) Close() error {
	if terminal == nil {
		return nil
	}
	terminal.close.Do(func() {
		_ = terminal.stdin.Close()
		if terminal.command.Process != nil {
			_ = terminal.command.Process.Kill()
		}
		_ = terminal.stdout.Close()
		<-terminal.waitDone
	})
	return nil
}
