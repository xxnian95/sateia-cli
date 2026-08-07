package credential

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxTokenFileSize = 64 << 10

type PreparedTokenFile struct {
	path      string
	file      *os.File
	committed bool
}

func PrepareTokenFile(rawPath string) (*PreparedTokenFile, error) {
	path, err := normalizeTokenFilePath(rawPath)
	if err != nil {
		return nil, err
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("inspect token file directory: %w", err)
	}
	if !parent.IsDir() {
		return nil, errors.New("token file parent must be a directory")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("token file already exists: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("create token file: %w", err)
	}
	return &PreparedTokenFile{path: path, file: file}, nil
}

func (prepared *PreparedTokenFile) Path() string {
	return prepared.path
}

func (prepared *PreparedTokenFile) Commit(token string) error {
	if prepared == nil || prepared.file == nil {
		return errors.New("token file is not prepared")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("token is empty")
	}
	if _, err := io.WriteString(prepared.file, token); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	if err := prepared.file.Sync(); err != nil {
		return fmt.Errorf("sync token file: %w", err)
	}
	if err := prepared.file.Close(); err != nil {
		return fmt.Errorf("close token file: %w", err)
	}
	prepared.file = nil
	prepared.committed = true
	return nil
}

func (prepared *PreparedTokenFile) Abort() {
	if prepared == nil || prepared.committed {
		return
	}
	if prepared.file != nil {
		_ = prepared.file.Close()
		prepared.file = nil
	}
	_ = os.Remove(prepared.path)
}

func ReadTokenFile(rawPath string) (string, error) {
	path, err := normalizeTokenFilePath(rawPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("token file must be a regular file")
	}
	if info.Size() < 1 || info.Size() > maxTokenFileSize {
		return "", fmt.Errorf("token file must contain between 1 and %d bytes", maxTokenFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errors.New("token file is empty")
	}
	return token, nil
}

func normalizeTokenFilePath(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", errors.New("token file path is required")
	}
	if strings.ContainsAny(path, "\r\n") {
		return "", errors.New("token file path must not contain line breaks")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve token file path: %w", err)
	}
	return absolute, nil
}
