package main

import (
	"fmt"
	"os"
	"path/filepath"

	gioapp "gioui.org/app"

	coreapp "s12ryt-ssh/internal/app"
	"s12ryt-ssh/internal/gui"
	"s12ryt-ssh/internal/remote"
	"s12ryt-ssh/internal/securestore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "s12ryt-ssh: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	metadataPath, securestoreDir := applicationPaths()
	secrets := securestore.NewDPAPIStoreAt(securestoreDir)
	service := coreapp.NewService(
		metadataPath,
		secrets,
		coreapp.DefaultBackendFactory,
	)
	remoteService := remote.NewService(applicationRemotePreferencesPath(), secrets, nil)
	window := new(gioapp.Window)
	controller := gui.NewWindowWithServices(service, remoteService, applicationPreferencesPath())
	result := make(chan error, 1)
	go func() {
		result <- controller.Run(window)
	}()
	gioapp.Main()
	return <-result
}

func applicationPreferencesPath() string {
	metadataPath, _ := applicationPaths()
	return filepath.Join(filepath.Dir(metadataPath), "preferences.json")
}

func applicationRemotePreferencesPath() string {
	metadataPath, _ := applicationPaths()
	return filepath.Join(filepath.Dir(metadataPath), "remote-preferences.json")
}

func applicationPaths() (metadataPath, securestoreDir string) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = "."
	}
	root := filepath.Join(configDir, "s12ryt-ssh")
	return filepath.Join(root, "metadata.json"), filepath.Join(root, "securestore")
}
