package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

var globalLock sync.Mutex
var processing = make(map[string]bool)
var doneProcessing = make(map[string]chan struct{})

// startProcessing marks the given hash as processing and returns a channel to
// wait on if the hash is already being processed
func startProcessing(hash string) (alreadyProcessing bool, waitChan chan struct{}) {
	globalLock.Lock()
	defer globalLock.Unlock()

	if processing[hash] {
		// If the hash is already being processed, return true and the channel
		// to wait on
		if ch, exists := doneProcessing[hash]; exists {
			return true, ch
		}

		doneProcessing[hash] = make(chan struct{})
		return true, doneProcessing[hash]
	}

	// Mark the hash as processing and proceed.
	processing[hash] = true
	doneProcessing[hash] = make(chan struct{})
	return false, nil
}

// finishProcessing marks the given hash as no longer processing and closes the
// channel to signal that the processing has finished
func finishProcessing(hash string) {
	globalLock.Lock()
	defer globalLock.Unlock()

	// Mark the hash as no longer processing
	processing[hash] = false
	if ch, exists := doneProcessing[hash]; exists {
		close(ch)
		delete(doneProcessing, hash)
	}
}

// Job represents a file to be processed
type Job struct {
	// Path to the file
	Path string

	// FileInfo of the file
	Info os.FileInfo
}

// computeFileHash returns the SHA256 hash of the file at the given path
func computeFileHash(path string) (string, error) {
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

// computeMetadataHash returns the SHA256 hash of the file metadata (only
// owner, group and mode are considered)
func computeMetadataHash(info os.FileInfo) string {
	sys := info.Sys().(*syscall.Stat_t)
	metadata := fmt.Sprintf("%d-%d-%d", sys.Uid, sys.Gid, info.Mode())
	hash := sha256.New()
	hash.Write([]byte(metadata))
	return hex.EncodeToString(hash.Sum(nil))
}

// handleFile processes the file at the given path
func handleFile(job Job, storagePath string, withMetadata bool) error {
	contentHash, err := computeFileHash(job.Path)
	if err != nil {
		return fmt.Errorf("computing file hash: %w", err)
	}

	var finalHash string
	if withMetadata {
		metadataHash := computeMetadataHash(job.Info)
		finalHash = contentHash + "-" + metadataHash
	} else {
		finalHash = contentHash
	}

	// Check if the file is already being processed and wait for it to finish
	alreadyProcessing, waitChan := startProcessing(finalHash)
	if alreadyProcessing {
		<-waitChan
	}

	dedupPath := filepath.Join(storagePath, finalHash)

	if _, err := os.Stat(dedupPath); os.IsNotExist(err) {
		if err := os.Rename(job.Path, dedupPath); err != nil {
			finishProcessing(finalHash)
			return fmt.Errorf("moving file to storage: %w", err)
		}
	} else {
		if err := os.Remove(job.Path); err != nil {
			finishProcessing(finalHash)
			return fmt.Errorf("removing duplicate file: %w", err)
		}
	}

	if _, err := os.Lstat(job.Path); os.IsNotExist(err) {
		if err := os.Link(dedupPath, job.Path); err != nil {
			finishProcessing(finalHash)
			return fmt.Errorf("creating link: %w", err)
		}
	}

	finishProcessing(finalHash)
	return nil
}

// Dedup walks the source directory and deduplicates files in the storage
// directory using the given number of workers
func Dedup(source, storagePath string, workers int, withMetadata bool) error {
	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	if _, err := os.Stat(filepath.Join(storagePath, ".metadata_mode")); os.IsNotExist(err) {
		modeFlag := []byte("without-metadata")
		if withMetadata {
			modeFlag = []byte("with-metadata")
		}
		if err := os.WriteFile(filepath.Join(storagePath, ".metadata_mode"), modeFlag, 0644); err != nil {
			return fmt.Errorf("writing metadata mode flag: %w", err)
		}
	}

	jobs := make(chan Job, workers)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := handleFile(job, storagePath, withMetadata); err != nil {
					log.Println(err)
				}
			}
		}()
	}

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && path != storagePath {
			jobs <- Job{Path: path, Info: info}
		}
		return nil
	})
	close(jobs)

	wg.Wait()
	return err
}

// Cp copies the file from source to dest and deduplicates it in the storage
// if not already present
func Cp(source, dest, storagePath string, withMetadata bool) error {
	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("getting source file info: %w", err)
	}

	job := Job{Path: source, Info: info}
	if err := handleFile(job, storagePath, withMetadata); err != nil {
		return fmt.Errorf("handling file: %w", err)
	}

	contentHash, _ := computeFileHash(source)
	metadataHash := ""
	if withMetadata {
		metadataHash = computeMetadataHash(info)
	}
	dedupPath := filepath.Join(storagePath, contentHash+"-"+metadataHash)

	if err := os.Link(dedupPath, dest); err != nil {
		return fmt.Errorf("creating link: %w", err)
	}

	return nil
}

func main() {
	usage := `Usage:
dabadee dedup <source> <storage> <workers> [--with-metadata]
dabadee cp <source> <dest> <storage> [--with-metadata]
dabadee --help`

	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	withMetadata := false
	if len(os.Args) > 4 && os.Args[len(os.Args)-1] == "--with-metadata" {
		withMetadata = true
	}

	switch os.Args[1] {
	case "cp":
		if len(os.Args) < 5 || (withMetadata && len(os.Args) != 6) {
			fmt.Println("Usage: dabadee cp <source> <dest> <storage> [--with-metadata]")
			os.Exit(1)
		}
		if err := Cp(os.Args[2], os.Args[3], os.Args[4], withMetadata); err != nil {
			log.Fatalf("Error during copy and link: %v", err)
		}
	case "dedup":
		if len(os.Args) < 4 || (withMetadata && len(os.Args) != 6) {
			fmt.Println("Usage: dabadee dedup <source> <storage> <workers> [--with-metadata]")
			os.Exit(1)
		}
		workers, err := strconv.Atoi(os.Args[4])
		if err != nil {
			log.Fatalf("Error converting workers to integer: %v", err)
		}
		if err := Dedup(os.Args[2], os.Args[3], workers, withMetadata); err != nil {
			log.Fatalf("Error during deduplication: %v", err)
		}
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}
