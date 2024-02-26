package hash

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/minio/highwayhash"
)

// HighwayHashGenerator is a generator that computes HighwayHash hashes
type HighwayHashGenerator struct {
	key []byte
}

// NewHighwayHashGenerator creates a new HighwayHashGenerator
func NewHighwayHashGenerator() *HighwayHashGenerator {
	key := make([]byte, 32)
	return &HighwayHashGenerator{key: key}
}

// ComputeFileHash computes the HighwayHash of the file at the given path
func (gen *HighwayHashGenerator) ComputeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	hasher, err := highwayhash.New(gen.key)
	if err != nil {
		return "", fmt.Errorf("creating hasher: %w", err)
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("calculating hash: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeMetadataHash computes the HighwayHash of the file metadata
func (gen *HighwayHashGenerator) ComputeMetadataHash(info os.FileInfo) string {
	sys := info.Sys().(*syscall.Stat_t)
	metadata := fmt.Sprintf("%d-%d-%d", sys.Uid, sys.Gid, info.Mode())
	hasher, err := highwayhash.New(gen.key)
	if err != nil {
		panic(fmt.Sprintf("creating hasher: %v", err))
	}
	hasher.Write([]byte(metadata))
	return hex.EncodeToString(hasher.Sum(nil))
}

// ComputeFullHash computes the full HighwayHash of the file at the given path
func (gen *HighwayHashGenerator) ComputeFullHash(path string) (string, error) {
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
