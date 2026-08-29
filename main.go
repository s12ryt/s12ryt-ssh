package main

import (
	"fmt"
	"os"
	"path/filepath"

	gioapp "gioui.org/app"

	"s12ryt-ssh/internal/gui"
	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/securestore"
)

func main() {
	// Gio's app.Main blocks forever on Windows (osMain ends in select{}), so the
	// UI loop runs on a separate goroutine and exits the process explicitly.
	go func() {
		if err := run(); err != nil {
			fmt.Fprintf(os.Stderr, "s12ryt-ssh: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	gioapp.Main()
}

func run() error {
	secrets := securestore.NewDPAPIStoreAt(applicationSecurestoreDir())
	remoteService := remote.NewService(applicationRemotePreferencesPath(), secrets, nil)
	window := new(gioapp.Window)
	controller := gui.NewWindowWithPreferences(remoteService, applicationPreferencesPath())
	return controller.Run(window)
}

func applicationPreferencesPath() string {
	return filepath.Join(applicationConfigRoot(), "preferences.json")
}

func applicationRemotePreferencesPath() string {
	return filepath.Join(applicationConfigRoot(), "remote-preferences.json")
}

func applicationSecurestoreDir() string {
	return filepath.Join(applicationConfigRoot(), "securestore")
}

func applicationConfigRoot() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = "."
	}
	return filepath.Join(configDir, "s12ryt-ssh")
}
