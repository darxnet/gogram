package gogram

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func ReceiveFileReader(client *Client, file *File) (io.ReadCloser, error) {
	link := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", client.Token(), file.FilePath)

	req, err := http.NewRequest(http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("gogram: failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gogram: failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("gogram: failed to download file: %s", resp.Status)
	}

	return resp.Body, nil
}

func ReceiveFileReaderByFileID(client *Client, fileID string) (io.ReadCloser, error) {
	file, err := client.GetFile(&GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}

	return ReceiveFileReader(client, file)
}

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

func DownloadByFileID(client *Client, fileID, filePath string) error {
	file, err := client.GetFile(&GetFileParams{FileID: fileID})
	if err != nil {
		return err
	}

	return DownloadFile(client, file, filePath)
}
