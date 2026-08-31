package ssh

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"time"

	pkgsftp "github.com/pkg/sftp"
)

// SFTPEntry is the transport-neutral metadata exposed to the GUI.
type SFTPEntry struct {
	Name      string
	Path      string
	Size      int64
	Mode      os.FileMode
	Modified  time.Time
	Directory bool
	Symlink   bool
}

// SFTPClient keeps the GUI independent from the concrete SFTP package.
type SFTPClient interface {
	ReadDir(context.Context, string) ([]SFTPEntry, error)
	Lstat(string) (SFTPEntry, error)
	Mkdir(string) error
	Rename(string, string) error
	Remove(string) error
	RemoveDirectory(string) error
	Symlink(string, string) error
	ReadLink(string) (string, error)
	OpenReader(string, int64) (io.ReadCloser, error)
	OpenWriter(string, int64, bool) (io.WriteCloser, error)
	Close() error
}

type sftpClient struct {
	client *pkgsftp.Client
}

// OpenSFTP opens an independent SFTP session over the existing SSH transport.
func (c *Client) OpenSFTP() (SFTPClient, error) {
	if c.conn == nil {
		return nil, errNotConnected
	}
	client, err := pkgsftp.NewClient(c.conn)
	if err != nil {
		return nil, err
	}
	return &sftpClient{client: client}, nil
}

func (c *sftpClient) ReadDir(ctx context.Context, remotePath string) ([]SFTPEntry, error) {
	if ctx == nil {
		return nil, errors.New("ssh: nil context")
	}
	files, err := c.client.ReadDirContext(ctx, remotePath)
	if err != nil {
		return nil, err
	}
	entries := make([]SFTPEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, sftpEntryFromFileInfo(path.Join(remotePath, file.Name()), file))
	}
	return entries, nil
}

func (c *sftpClient) Lstat(remotePath string) (SFTPEntry, error) {
	info, err := c.client.Lstat(remotePath)
	if err != nil {
		return SFTPEntry{}, err
	}
	return sftpEntryFromFileInfo(remotePath, info), nil
}

func (c *sftpClient) Mkdir(remotePath string) error {
	return c.client.Mkdir(remotePath)
}

func (c *sftpClient) Rename(oldPath, newPath string) error {
	return c.client.Rename(oldPath, newPath)
}

func (c *sftpClient) Remove(remotePath string) error {
	return c.client.Remove(remotePath)
}

func (c *sftpClient) RemoveDirectory(remotePath string) error {
	return c.client.RemoveDirectory(remotePath)
}

func (c *sftpClient) Symlink(targetPath, linkPath string) error {
	return c.client.Symlink(targetPath, linkPath)
}

func (c *sftpClient) ReadLink(remotePath string) (string, error) {
	return c.client.ReadLink(remotePath)
}

func (c *sftpClient) OpenReader(remotePath string, offset int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, errors.New("ssh: invalid SFTP read offset")
	}
	file, err := c.client.Open(remotePath)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (c *sftpClient) OpenWriter(remotePath string, offset int64, truncate bool) (io.WriteCloser, error) {
	if offset < 0 {
		return nil, errors.New("ssh: invalid SFTP write offset")
	}
	flags := os.O_CREATE | os.O_WRONLY
	if truncate {
		flags |= os.O_TRUNC
	}
	file, err := c.client.OpenFile(remotePath, flags)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (c *sftpClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func sftpEntryFromFileInfo(remotePath string, info os.FileInfo) SFTPEntry {
	return SFTPEntry{
		Name:      info.Name(),
		Path:      remotePath,
		Size:      info.Size(),
		Mode:      info.Mode(),
		Modified:  info.ModTime(),
		Directory: info.IsDir(),
		Symlink:   info.Mode()&os.ModeSymlink != 0,
	}
}
