package processor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/storage"
)

// CpProcessor is a processor that links a stored file to a destination. If the
// file is not already in storage, it is moved there and link the source to it
// before linking it to the destination
type CpProcessor struct {
	// SourceFile is the path of the file to be linked
	SourceFile string

	// DestFile is the path of the link to be created
	DestFile string

	// Storage is the storage interface to use
	Storage *storage.Storage

	// HashGen is the hash generator to use
	HashGen hash.Generator

	// WithMetadata is a flag to enable metadata hashing
	WithMetadata bool
}

// NewCpProcessor creates a new CpProcessor
func NewCpProcessor(sourceFile, destFile string, storage *storage.Storage, hashGen hash.Generator, withMetadata bool) *CpProcessor {
	return &CpProcessor{
		SourceFile:   sourceFile,
		DestFile:     destFile,
		Storage:      storage,
		HashGen:      hashGen,
		WithMetadata: withMetadata,
	}
}

// Process processes the file and creates a link at the destination
func (p *CpProcessor) Process() (err error) {
	// Compute file hash
	var finalHash string

	if p.WithMetadata {
		finalHash, err = p.HashGen.ComputeFullHash(p.SourceFile)
		if err != nil {
			return fmt.Errorf("computing full hash: %w", err)
		}
	} else {
		finalHash, err = p.HashGen.ComputeFileHash(p.SourceFile)
		if err != nil {
			return fmt.Errorf("computing content hash: %w", err)
		}
	}

	dedupPath := filepath.Join(p.Storage.Path, finalHash)

	// Check if the deduplicated file already exists in storage
	exists, err := p.Storage.FileExists(dedupPath)
	if err != nil {
		return fmt.Errorf("checking file existence in storage: %w", err)
	}

	// If the file does not exist, move it to storage
	if !exists {
		err = p.Storage.MoveFileToStorage(p.SourceFile, finalHash)
		if err != nil {
			return fmt.Errorf("moving file to storage: %w", err)
		}
	}

	// Create a link at the destination pointing to the file in storage,
	// removing the destination if it already exists
	os.Remove(p.DestFile)
	err = os.Link(dedupPath, p.DestFile)
	if err != nil {
		return fmt.Errorf("linking file: %w", err)
	}

	return nil
}
