package storage

import (
	"os"
	"path/filepath"
)

// Storage is an interface to interact with the storage
type Storage struct {
	// Path is the base path of the storage
	Path string
}

// NewStorage creates a new Storage
func NewStorage(path string) *Storage {
	storage := &Storage{Path: path}
	_ = os.MkdirAll(path, 0755)
	return storage
}

// MoveFileToStorage moves the file from the source path to the storage
func (s *Storage) MoveFileToStorage(sourcePath, destHash string) error {
	destPath := filepath.Join(s.Path, destHash)
	return os.Rename(sourcePath, destPath)
}

// RemoveFile removes the file at the given path
func (s *Storage) RemoveFile(filePath string) error {
	return os.Remove(filePath)
}

// CreateLink creates a link at the given path pointing to the target path
func (s *Storage) CreateLink(targetPath, linkPath string) error {
	return os.Link(targetPath, linkPath)
}

// FileExists checks if the file at the given path exists
func (s *Storage) FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
