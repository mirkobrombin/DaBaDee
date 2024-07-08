package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/storage"
)

var (
	globalLock     sync.Mutex
	processing     = make(map[string]bool)
	doneProcessing = make(map[string]chan struct{})
)

// DedupProcessor is a processor that deduplicates files by moving them to a
// storage and creating a link to the original location
type DedupProcessor struct {
	// Source is the path of the directory to deduplicate
	Source string

	// DestDir is the path of the directory to copy deduplicated files to
	DestDir string

	// Storage is the storage interface to use
	Storage *storage.Storage

	// HashGen is the hash generator to use
	HashGen hash.Generator

	// Workers is the number of workers to use
	Workers int

	// FileMap is a map of original file paths to their hash in storage
	FileMap map[string]string

	// mapMutex is a mutex to protect the FileMap from concurrent access
	mapMutex sync.Mutex
}

// NewDedupProcessor creates a new DedupProcessor
func NewDedupProcessor(source, destDir string, storage *storage.Storage, hashGen hash.Generator, workers int) *DedupProcessor {
	return &DedupProcessor{
		Source:  source,
		DestDir: destDir,
		Storage: storage,
		HashGen: hashGen,
		Workers: workers,
		FileMap: make(map[string]string),
	}
}

// dedupStartProcessing marks the given hash as processing and returns a channel to
// wait on if the hash is already being processed
func dedupStartProcessing(hash string) (alreadyProcessing bool, waitChan chan struct{}) {
	globalLock.Lock()
	defer globalLock.Unlock()

	if processing[hash] {
		// If the hash is already being processed, return the channel to wait on
		if ch, exists := doneProcessing[hash]; exists {
			return true, ch
		}

		doneProcessing[hash] = make(chan struct{})
		return true, doneProcessing[hash]
	}

	// Mark the hash as processing and proceed
	processing[hash] = true
	doneProcessing[hash] = make(chan struct{})
	return false, nil
}

// dedupFinishProcessing marks the given hash as no longer processing and closes the
// channel to signal that the processing has finished
func dedupFinishProcessing(hash string) {
	globalLock.Lock()
	defer globalLock.Unlock()

	processing[hash] = false
	if ch, exists := doneProcessing[hash]; exists {
		close(ch)
		delete(doneProcessing, hash)
	}
}

// Process processes the files in the source directory
func (p *DedupProcessor) Process(verbose bool) error {
	if p.DestDir != "" {
		if verbose {
			log.Printf("Creating destination directory: %s", p.DestDir)
		}
		err := os.MkdirAll(p.DestDir, 0755)
		if err != nil {
			return fmt.Errorf("creating destination directory: %w", err)
		}
	}

	jobs := make(chan string, p.Workers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < p.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				err := p.processFile(path, verbose)
				if err != nil {
					if verbose {
						log.Printf("Error processing file %s: %v", path, err)
					}
				}
			}
		}()
	}

	// Walk the source directory to enqueue jobs
	err := filepath.Walk(p.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if verbose {
				log.Printf("Error accessing path %s: %v", path, err)
			}
			return filepath.SkipDir
		}

		if !info.IsDir() && path != p.Storage.Opts.Root {
			// Check if we have permission to read the file
			file, err := os.Open(path)
			if err != nil {
				if verbose {
					log.Printf("Skipping file %s due to permissions: %v", path, err)
				}
				return nil
			}
			file.Close()

			if verbose {
				log.Printf("Adding file to job queue: %s", path)
			}
			jobs <- path
		}
		return nil
	})
	if err != nil {
		return err
	}
	close(jobs)
	wg.Wait()

	return nil
}

func (p *DedupProcessor) processFile(path string, verbose bool) (err error) {
	if verbose {
		log.Printf("Processing file: %s", path)
	}

	// Compute file hash
	var finalHash string

	if p.Storage.Opts.WithMetadata {
		if verbose {
			log.Println("Computing full hash with metadata")
		}
		finalHash, err = p.HashGen.ComputeFullHash(path)
		if err != nil {
			return fmt.Errorf("computing full hash: %w", err)
		}
	} else {
		if verbose {
			log.Println("Computing content hash without metadata")
		}
		finalHash, err = p.HashGen.ComputeFileHash(path)
		if err != nil {
			return fmt.Errorf("computing content hash: %w", err)
		}
	}

	// Check if the file is already being processed
	alreadyProcessing, waitChan := dedupStartProcessing(finalHash)
	if alreadyProcessing {
		if verbose {
			log.Printf("File %s is already being processed, waiting...", path)
		}
		<-waitChan // Wait for the processing to finish
	}

	// Check if a file with the same hash already exists in storage
	dedupPath := filepath.Join(p.Storage.Opts.Root, finalHash)
	exists, err := p.Storage.FileExists(dedupPath)
	if err != nil {
		dedupFinishProcessing(finalHash)
		return fmt.Errorf("checking file existence in storage: %w", err)
	}

	if !exists {
		if verbose {
			log.Printf("File does not exist in storage, moving it: %s", dedupPath)
		}
		// If the file does not exist in storage, move it there
		err = p.Storage.MoveFileToStorage(path, finalHash)
		if err != nil {
			dedupFinishProcessing(finalHash)
			return fmt.Errorf("moving file to storage: %w", err)
		}
	} else {
		if verbose {
			log.Printf("File already exists in storage: %s", dedupPath)
		}
		// If the file already exists in storage, remove the source file
		err = os.Remove(path)
		if err != nil {
			dedupFinishProcessing(finalHash)
			return fmt.Errorf("removing source file: %w", err)
		}
	}

	// Store the original path of the file
	p.mapMutex.Lock()
	p.FileMap[path] = finalHash
	p.mapMutex.Unlock()

	// Create a link at the original location
	if verbose {
		log.Printf("Creating link at original location: %s", path)
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		err = os.Link(dedupPath, path)
		if err != nil {
			dedupFinishProcessing(finalHash)
			return fmt.Errorf("creating link to deduplicated file: %w", err)
		}
	}

	// Create a link at the destination if DestDir is set
	if p.DestDir != "" {
		relativePath, err := filepath.Rel(p.Source, path)
		if err != nil {
			dedupFinishProcessing(finalHash)
			return fmt.Errorf("getting relative path: %w", err)
		}

		destPath := filepath.Join(p.DestDir, relativePath)
		if verbose {
			log.Printf("Creating link at destination: %s", destPath)
		}
		if _, err := os.Lstat(destPath); os.IsNotExist(err) {
			err = os.Link(dedupPath, destPath)
			if err != nil {
				dedupFinishProcessing(finalHash)
				return fmt.Errorf("creating link to deduplicated file in destination: %w", err)
			}
		}
	}

	dedupFinishProcessing(finalHash)

	if verbose {
		log.Printf("Finished processing file: %s", path)
	}
	return nil
}
