//go:build windows

package gui

import "testing"

func TestWindowsLocalShellUsesConPTYBackend(t *testing.T) {
	if !localShellUsesConPTY() {
		t.Fatal("Windows local shell must use the ConPTY backend")
	}
}
