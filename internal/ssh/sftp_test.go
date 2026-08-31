package ssh

import (
	"errors"
	"os"
	"testing"
	"time"

	"s12ryt-ssh/internal/config"
)

func TestClientOpenSFTPRequiresConnection(t *testing.T) {
	client := NewClient(config.SSHProfile{Name: "test", Host: "host", Port: 22, User: "user"})

	session, err := client.OpenSFTP()
	if session != nil {
		t.Fatal("an unconnected client must not return an SFTP session")
	}
	if !errors.Is(err, errNotConnected) {
		t.Fatalf("OpenSFTP error = %v, want %v", err, errNotConnected)
	}
}

func TestSFTPEntryFromFileInfoPreservesRemoteMetadata(t *testing.T) {
	modified := time.Date(2026, time.August, 30, 10, 20, 30, 0, time.UTC)
	tests := []struct {
		name      string
		info      os.FileInfo
		path      string
		directory bool
		symlink   bool
	}{
		{
			name:      "directory",
			info:      testSFTPFileInfo{name: "logs", mode: os.ModeDir | 0o750, modified: modified},
			path:      "/srv/logs",
			directory: true,
		},
		{
			name:    "symbolic link",
			info:    testSFTPFileInfo{name: "current", size: 12, mode: os.ModeSymlink | 0o777, modified: modified},
			path:    "/srv/current",
			symlink: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := sftpEntryFromFileInfo(tt.path, tt.info)
			if entry.Name != tt.info.Name() || entry.Path != tt.path {
				t.Fatalf("entry identity = %#v", entry)
			}
			if entry.Size != tt.info.Size() || entry.Mode != tt.info.Mode() || !entry.Modified.Equal(modified) {
				t.Fatalf("entry metadata = %#v", entry)
			}
			if entry.Directory != tt.directory || entry.Symlink != tt.symlink {
				t.Fatalf("entry type = directory %v, symlink %v", entry.Directory, entry.Symlink)
			}
		})
	}
}

type testSFTPFileInfo struct {
	name     string
	size     int64
	mode     os.FileMode
	modified time.Time
}

func (f testSFTPFileInfo) Name() string       { return f.name }
func (f testSFTPFileInfo) Size() int64        { return f.size }
func (f testSFTPFileInfo) Mode() os.FileMode  { return f.mode }
func (f testSFTPFileInfo) ModTime() time.Time { return f.modified }
func (f testSFTPFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f testSFTPFileInfo) Sys() any           { return nil }
