package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"
)

// SHA256Generator is a generator that computes SHA256 hashes
type SHA256Generator struct{}

// NewSHA256Generator creates a new SHA256Generator
func NewSHA256Generator() *SHA256Generator {
	return &SHA256Generator{}
}

// ComputeFileHash computes the SHA256 hash of the file at the given path
func (gen *SHA256Generator) ComputeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("calculating hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// ComputeMetadataHash computes the SHA256 hash of the file metadata
func (gen *SHA256Generator) ComputeMetadataHash(info os.FileInfo) string {
	sys := info.Sys().(*syscall.Stat_t)
	metadata := fmt.Sprintf("%d-%d-%d", sys.Uid, sys.Gid, info.Mode())
	hash := sha256.New()
	hash.Write([]byte(metadata))
	return hex.EncodeToString(hash.Sum(nil))
}

// ComputeFullHash computes the full SHA256 hash of the file at the given path
// by combining the content hash and the metadata hash
func (gen *SHA256Generator) ComputeFullHash(path string) (string, error) {
	contentHash, err := gen.ComputeFileHash(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("getting file info: %w", err)
	}

	metadataHash := gen.ComputeMetadataHash(info)
	return contentHash + "-" + metadataHash, nil
}
