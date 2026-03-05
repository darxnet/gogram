package gogram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (c *Client) fileLink(filePath string) string {
	c.locker.RLock()
	host := c.options.host
	token := c.token
	test := c.options.test
	c.locker.RUnlock()

	link := "https://" + host + "/file/bot" + token + "/"
	if test {
		link += "test/"
	}

	return link + strings.TrimPrefix(filePath, "/")
}

// ReceiveFileReader returns a reader for the file content from Telegram servers.
// The caller is responsible for closing the reader.
func (c *Client) ReceiveFileReader(ctx context.Context, file *File) (io.ReadCloser, error) {
	if file == nil || file.FilePath == "" {
		return c.ReceiveFileReaderByFileID(ctx, file.FileID)
	}

	link := c.fileLink(file.FilePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("gogram: failed to create request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("gogram: failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		err := NewError(resp.StatusCode, resp.Status)
		return nil, fmt.Errorf("gogram: failed to download file: %w", err)
	}

	return resp.Body, nil
}

// ReceiveFileReaderByFileID resolves the file path by fileID and returns a reader for the content.
// The caller is responsible for closing the reader.
func (c *Client) ReceiveFileReaderByFileID(ctx context.Context, fileID string) (io.ReadCloser, error) {
	file, err := c.GetFile(ctx, &GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}

	return c.ReceiveFileReader(ctx, file)
}

// DownloadFile downloads a file from Telegram servers to the specified local path.
func (c *Client) DownloadFile(ctx context.Context, file *File, path string) error {
	const perm = 0o640

	f, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("gogram: failed to open file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	rc, err := c.ReceiveFileReader(ctx, file)
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck

	_, err = io.Copy(f, rc)
	if err != nil {
		return fmt.Errorf("gogram: failed to copy file: %w", err)
	}

	return nil
}

// DownloadByFileID resolves the file path by fileID and downloads the file to the specified local path.
func (c *Client) DownloadByFileID(ctx context.Context, fileID, filePath string) error {
	file, err := c.GetFile(ctx, &GetFileParams{FileID: fileID})
	if err != nil {
		return err
	}

	return c.DownloadFile(ctx, file, filePath)
}

// ReceiveFileReader returns a reader for the file content from Telegram servers.
// The caller is responsible for closing the reader.
//
// Deprecated: use (*Client).ReceiveFileReader.
func ReceiveFileReader(client *Client, file *File) (io.ReadCloser, error) {
	return client.ReceiveFileReader(context.Background(), file)
}

// ReceiveFileReaderByFileID resolves the file path by fileID and returns a reader for the content.
// The caller is responsible for closing the reader.
//
// Deprecated: use (*Client).ReceiveFileReaderByFileID.
func ReceiveFileReaderByFileID(client *Client, fileID string) (io.ReadCloser, error) {
	return client.ReceiveFileReaderByFileID(context.Background(), fileID)
}

// DownloadFile downloads a file from Telegram servers to the specified local path.
//
// Deprecated: use (*Client).DownloadFile.
func DownloadFile(client *Client, file *File, path string) error {
	return client.DownloadFile(context.Background(), file, path)
}

// DownloadByFileID resolves the file path by fileID and downloads the file to the specified local path.
//
// Deprecated: use (*Client).DownloadByFileID.
func DownloadByFileID(client *Client, fileID, filePath string) error {
	return client.DownloadByFileID(context.Background(), fileID, filePath)
}
