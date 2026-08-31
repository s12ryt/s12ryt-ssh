package gui

import (
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

const sftpDropMaxBytes = 1 << 20

var (
	errInvalidSFTPDrop  = errors.New("invalid SFTP file drop")
	errSFTPDropTooLarge = errors.New("SFTP file drop exceeds the size limit")
)

func readSFTPDropData(reader io.ReadCloser) ([]string, error) {
	if reader == nil {
		return nil, errInvalidSFTPDrop
	}
	paths, readErr := readSFTPDropPayload(reader)
	return paths, errors.Join(readErr, reader.Close())
}

func (ui *Window) handleSFTPDropData(tabID string, reader io.ReadCloser) bool {
	paths, err := readSFTPDropData(reader)
	if err != nil {
		if ui != nil && ui.model != nil {
			if errors.Is(err, errSFTPDropTooLarge) {
				ui.model.Error = ui.text("Dropped file data is too large.")
			} else {
				ui.model.Error = ui.text("Dropped files are invalid.")
			}
		}
		return true
	}
	if len(paths) == 0 || ui == nil || ui.transferSFTPTab(tabID) == nil {
		return true
	}
	ui.prepareSFTPUploads(tabID, paths)
	return true
}

func readSFTPDropPayload(reader io.Reader) ([]string, error) {
	if reader == nil {
		return nil, errInvalidSFTPDrop
	}
	payload, err := io.ReadAll(io.LimitReader(reader, sftpDropMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > sftpDropMaxBytes {
		return nil, errSFTPDropTooLarge
	}
	return parseDroppedFilePaths(string(payload))
}

func parseDroppedFilePaths(payload string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(payload, "\r\n", "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		filePath, err := droppedFilePath(line)
		if err != nil {
			return nil, err
		}
		paths = append(paths, filePath)
	}
	return paths, nil
}

func droppedFilePath(value string) (string, error) {
	if strings.ContainsRune(value, '\x00') {
		return "", errInvalidSFTPDrop
	}
	if strings.Contains(value, "://") || strings.HasPrefix(strings.ToLower(value), "file:") {
		parsed, err := url.Parse(value)
		if err != nil || !strings.EqualFold(parsed.Scheme, "file") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", errInvalidSFTPDrop
		}
		if parsed.Host != "" {
			return `\\` + parsed.Host + filepath.FromSlash(parsed.Path), nil
		}
		filePath := filepath.FromSlash(parsed.Path)
		if len(filePath) >= 3 && filePath[0] == '\\' && filePath[2] == ':' {
			filePath = filePath[1:]
		}
		if !filepath.IsAbs(filePath) {
			return "", errInvalidSFTPDrop
		}
		return filePath, nil
	}

	if !filepath.IsAbs(value) {
		return "", errInvalidSFTPDrop
	}
	return filepath.Clean(value), nil
}
