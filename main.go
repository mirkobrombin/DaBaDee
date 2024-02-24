package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

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

// handleFile processes the file at the given path
func handleFile(job Job, storagePath string) error {
	hashSum, err := computeFileHash(job.Path)
	if err != nil {
		return fmt.Errorf("computing file hash: %w", err)
	}

	dedupPath := filepath.Join(storagePath, hashSum)

	// If the file is not already in the storage, move it there, otherwise
	// remove the duplicate
	if _, err := os.Stat(dedupPath); os.IsNotExist(err) {
		if err := os.Rename(job.Path, dedupPath); err != nil {
			return fmt.Errorf("moving file to storage: %w", err)
		}
	} else if err := os.Remove(job.Path); err != nil {
		return fmt.Errorf("removing duplicate file: %w", err)
	}

	// Create a link to the deduplicated file in the source directory to
	// make it accessible from the original path
	if _, err := os.Lstat(job.Path); os.IsNotExist(err) {
		if err := os.Link(dedupPath, job.Path); err != nil {
			return fmt.Errorf("creating link: %w", err)
		}
	}

	return nil
}

// Dedup walks the source directory and deduplicates files in the storage
// directory using the given number of workers
func Dedup(source, storagePath string, workers int) error {
	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	jobs := make(chan Job, workers)
	var wg sync.WaitGroup

	// Start the workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := handleFile(job, storagePath); err != nil {
					log.Println(err)
				}
			}
		}()
	}

	// Walk the source directory and send the files to the workers
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
func Cp(source, dest, storagePath string) error {
	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	job := Job{Path: source}
	if err := handleFile(job, storagePath); err != nil {
		return fmt.Errorf("handling file: %w", err)
	}

	hashSum, err := computeFileHash(source)
	if err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	dedupPath := filepath.Join(storagePath, hashSum)
	if err := os.Link(dedupPath, dest); err != nil {
		return fmt.Errorf("creating link: %w", err)
	}

	return nil
}

func main() {
	usage := `Usage:
dabadee dedup <source> <storage> <workers>
dabadee cp <source> <dest> <storage>
dabadee --help`

	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "cp":
		if len(os.Args) != 5 {
			fmt.Println("Usage: dabadee cp <source> <dest> <storage>")
			os.Exit(1)
		}
		if err := Cp(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			log.Fatalf("Error during copy and link: %v", err)
		}
	case "dedup":
		if len(os.Args) != 5 {
			fmt.Println("Usage: dabadee dedup <source> <storage> <workers>")
			os.Exit(1)
		}
		workers, err := strconv.Atoi(os.Args[4])
		if err != nil {
			log.Fatalf("Error converting workers to integer: %v", err)
		}
		if err := Dedup(os.Args[2], os.Args[3], workers); err != nil {
			log.Fatalf("Error during deduplication: %v", err)
		}
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}
