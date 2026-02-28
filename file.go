package gogram

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func buildFileLink(client *Client, filePath string) string {
	client.locker.RLock()
	host := client.options.host
	token := client.token
	test := client.options.test
	client.locker.RUnlock()

	link := "https://" + host + "/file/bot" + token + "/"
	if test {
		link += "test/"
	}

	return link + strings.TrimPrefix(filePath, "/")
}

// ReceiveFileReader returns a reader for the file content from Telegram servers.
// The caller is responsible for closing the reader.
func ReceiveFileReader(client *Client, file *File) (io.ReadCloser, error) {
	link := buildFileLink(client, file.FilePath)

	req, err := http.NewRequest(http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("gogram: failed to create request: %w", err)
	}

	resp, err := client.do(req)
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
func ReceiveFileReaderByFileID(client *Client, fileID string) (io.ReadCloser, error) {
	file, err := client.GetFile(&GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}

	return ReceiveFileReader(client, file)
}

// DownloadFile downloads a file from Telegram servers to the specified local path.
func DownloadFile(client *Client, file *File, path string) error {
	const perm = 0o640

	f, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("gogram: failed to open file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	rc, err := ReceiveFileReader(client, file)
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
func DownloadByFileID(client *Client, fileID, filePath string) error {
	file, err := client.GetFile(&GetFileParams{FileID: fileID})
	if err != nil {
		return err
	}

	return DownloadFile(client, file, filePath)
}
