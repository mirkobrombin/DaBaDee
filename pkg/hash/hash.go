package hash

import (
	"os"
)

// Generator defines the operations for generating hashes of files and metadata
type Generator interface {
	// ComputeFileHash computes the hash of the file at the given path
	ComputeFileHash(path string) (string, error)

	// ComputeMetadataHash computes the hash of the metadata of the given
	// file info
	ComputeMetadataHash(info os.FileInfo) string

	// ComputeFullHash computes the hash of the file and its metadata using
	// the pattern "<file hash>-<metadata hash>"
	ComputeFullHash(path string) (string, error)
}
