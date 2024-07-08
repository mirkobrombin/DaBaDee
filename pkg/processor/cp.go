package processor

import (
	"fmt"
	"log"
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
}

// NewCpProcessor creates a new CpProcessor
func NewCpProcessor(sourceFile, destFile string, storage *storage.Storage, hashGen hash.Generator) *CpProcessor {
	return &CpProcessor{
		SourceFile: sourceFile,
		DestFile:   destFile,
		Storage:    storage,
		HashGen:    hashGen,
	}
}

// Process processes the file and creates a link at the destination
func (p *CpProcessor) Process(verbose bool) (err error) {
	if verbose {
		log.Printf("Processing file: %s", p.SourceFile)
	}

	// Compute file hash
	var finalHash string

	if p.Storage.Opts.WithMetadata {
		if verbose {
			log.Println("Computing full hash with metadata")
		}
		finalHash, err = p.HashGen.ComputeFullHash(p.SourceFile)
		if err != nil {
			return fmt.Errorf("computing full hash: %w", err)
		}
	} else {
		if verbose {
			log.Println("Computing content hash without metadata")
		}
		finalHash, err = p.HashGen.ComputeFileHash(p.SourceFile)
		if err != nil {
			return fmt.Errorf("computing content hash: %w", err)
		}
	}

	dedupPath := filepath.Join(p.Storage.Opts.Root, finalHash)

	// Check if the deduplicated file already exists in storage
	exists, err := p.Storage.FileExists(dedupPath)
	if err != nil {
		return fmt.Errorf("checking file existence in storage: %w", err)
	}

	// If the file does not exist, move it to storage
	if !exists {
		if verbose {
			log.Printf("File does not exist in storage, moving it: %s", dedupPath)
		}
		err = p.Storage.MoveFileToStorage(p.SourceFile, finalHash)
		if err != nil {
			return fmt.Errorf("moving file to storage: %w", err)
		}
	} else {
		if verbose {
			log.Printf("File already exists in storage: %s", dedupPath)
		}
	}

	// Create a link at the destination pointing to the file in storage,
	// removing the destination if it already exists
	if verbose {
		log.Printf("Creating link at destination: %s", p.DestFile)
	}
	os.Remove(p.DestFile)
	err = os.Link(dedupPath, p.DestFile)
	if err != nil {
		return fmt.Errorf("linking file: %w", err)
	}

	if verbose {
		log.Printf("Successfully linked file to destination: %s", p.DestFile)
	}

	return nil
}
