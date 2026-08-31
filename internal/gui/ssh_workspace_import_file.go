package gui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxSSHWorkspaceImportBytes = 16 * 1024 * 1024

func readSSHWorkspaceImportFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("workspace import file is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > maxSSHWorkspaceImportBytes {
		return "", fmt.Errorf("workspace import file exceeds %d bytes", maxSSHWorkspaceImportBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSSHWorkspaceImportBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSSHWorkspaceImportBytes {
		return "", fmt.Errorf("workspace import file exceeds %d bytes", maxSSHWorkspaceImportBytes)
	}
	return string(data), nil
}

func writeSSHWorkspaceExportFile(path, encoded string) (err error) {
	if path == "" {
		return errors.New("workspace export file is required")
	}
	if len(encoded) > maxSSHWorkspaceImportBytes {
		return fmt.Errorf("workspace export package exceeds %d bytes", maxSSHWorkspaceImportBytes)
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".s12ryt-ssh-export-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if cleanupErr := os.Remove(temporaryPath); err == nil && cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = cleanupErr
		}
	}()
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = io.WriteString(temporary, encoded); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(temporaryPath, path)
}
