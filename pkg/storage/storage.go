package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Storage is an interface to interact with the storage
type Storage struct {
	// Opts are the options for the storage
	Opts StorageOptions
}

// StorageOptions are the options for the storage
type StorageOptions struct {
	// Root is the base path of the storage
	Root string

	// WithMetadata indicates if the storage should store metadata
	WithMetadata bool

	// Paths holds the parent paths used during the deduplication process
	Paths []string
}

// NewStorage creates a new Storage
func NewStorage(opts StorageOptions) (storage *Storage, err error) {
	// Check if storage directory exists
	_, err = os.Stat(opts.Root)
	if err != nil {
		if os.IsNotExist(err) {
			// Storage directory does not exist, so create it
			err = os.MkdirAll(opts.Root, 0755)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	configFilePath := filepath.Join(opts.Root, ".dabadee")

	// Check if the config file exists
	_, err = os.Stat(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file found, so create it
			optsFile, err := os.Create(configFilePath)
			if err != nil {
				return nil, err
			}
			defer optsFile.Close()

			optsJSON, err := json.Marshal(opts)
			if err != nil {
				return nil, err
			}

			_, err = optsFile.Write(optsJSON)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		// Config file found, so load it
		opts, err = loadConfig(opts.Root)
		if err != nil {
			return nil, err
		}
	}

	storage = &Storage{Opts: opts}
	return storage, nil
}

func loadConfig(root string) (StorageOptions, error) {
	optsFile, err := os.Open(filepath.Join(root, ".dabadee"))
	if err != nil {
		return StorageOptions{}, err
	}
	defer optsFile.Close()

	var opts StorageOptions
	err = json.NewDecoder(optsFile).Decode(&opts)
	if err != nil {
		return StorageOptions{}, err
	}

	return opts, nil
}

// MoveFileToStorage moves the file from the source path to the storage
func (s *Storage) MoveFileToStorage(sourcePath, destHash string) error {
	destPath := filepath.Join(s.Opts.Root, destHash)
	err := os.Rename(sourcePath, destPath)
	if err != nil {
		return err
	}

	// store the parent path of the file
	parentPath := filepath.Dir(sourcePath)
	err = s.storeNewPath(parentPath)
	if err != nil {
		return err
	}

	return nil
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

// FindHardLinks finds all hard links to the specified file path, it searches
// the stored paths by default, additional paths can be specified to extend
// the search
func (s *Storage) FindLinks(filePath string, additionalPaths []string) ([]string, error) {
	var links []string

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, os.ErrInvalid
	}

	inode := stat.Ino

	searchFunc := func(path string, d os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		dStat, ok := d.Sys().(*syscall.Stat_t)
		if ok && dStat.Ino == inode {
			links = append(links, path)
		}

		return nil
	}

	paths := append(s.Opts.Paths, additionalPaths...)
	for _, path := range paths {
		err = filepath.Walk(path, searchFunc)
		if err != nil {
			return nil, err
		}
	}

	return links, nil
}

// updateOpts updates the storage options
func (s *Storage) updateOpts(opts StorageOptions) error {
	optsFile, err := os.Create(filepath.Join(opts.Root, ".dabadee"))
	if err != nil {
		return err
	}
	defer optsFile.Close()

	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return err
	}

	_, err = optsFile.Write(optsJSON)
	if err != nil {
		return err
	}

	return nil
}

// storeNewPath stores the parent path of the file
func (s *Storage) storeNewPath(path string) error {
	// Check if the path is already stored
	for _, p := range s.Opts.Paths {
		if p == path || (len(p) < len(path) && path[:len(p)] == p) {
			return nil
		}
	}

	// Store the new path
	s.Opts.Paths = append(s.Opts.Paths, path)
	return s.updateOpts(s.Opts)
}

// removeStoredPath removes the parent path of the file
func (s *Storage) removeStoredPath(path string) error {
	for i, p := range s.Opts.Paths {
		if p == path || (len(p) < len(path) && path[:len(p)] == p) {
			s.Opts.Paths = append(s.Opts.Paths[:i], s.Opts.Paths[i+1:]...)
			return s.updateOpts(s.Opts)
		}
	}

	return nil
}

// RemoveFile removes a file from the storage and all its links.
func (s *Storage) RemoveFile(path string) (err error) {
	ok := s.isStoredPath(path)
	if !ok {
		path, err = s.findStoredPath(path)
		if err != nil {
			return err
		}
	}

	links, err := s.FindLinks(path, nil)
	if err != nil {
		return err
	}

	for _, link := range links {
		err = os.Remove(link)
		if err != nil {
			return err
		}
	}

	err = os.Remove(path)
	if err != nil {
		return err
	}

	return nil
}

// isStoredPath checks if the path is stored
func (s *Storage) isStoredPath(path string) bool {
	absStorePath, err := filepath.Abs(s.Opts.Root)
	if err != nil {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return strings.HasPrefix(absPath, absStorePath)
}

// findStoredPath finds the stored path for the given path by looking
// for the same inode in the store
func (s *Storage) findStoredPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", os.ErrInvalid
	}

	inode := stat.Ino

	searchFunc := func(p string, d os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		dStat, ok := d.Sys().(*syscall.Stat_t)
		if ok && dStat.Ino == inode {
			path = p
			return filepath.SkipDir
		}

		return nil
	}

	err = filepath.Walk(s.Opts.Root, searchFunc)
	if err != nil {
		return "", err
	}

	return path, nil
}

// RemoveOrphans removes all files that are not linked to any other file
func (s *Storage) RemoveOrphans() error {
	// Get all files in the storage
	files, err := s.ListFiles()
	if err != nil {
		return err
	}

	// Check if the file is linked to any other file
	for _, file := range files {
		if file.Name() == ".dabadee" {
			continue
		}

		links, err := s.FindLinks(filepath.Join(s.Opts.Root, file.Name()), nil)
		if err != nil {
			return err
		}

		if len(links) == 0 {
			err = s.RemoveFile(filepath.Join(s.Opts.Root, file.Name()))
			if err != nil {
				return err
			}
		}
	}

	// Check if the registered paths have still at least one linked file
	for _, path := range s.Opts.Paths {
		var links []string

		_, err := os.Stat(path)
		if err == nil {
			links, err = s.FindLinks(path, nil)
			if err != nil {
				return err
			}
		}

		if len(links) == 0 {
			// Remove the path from the list
			err = s.removeStoredPath(path)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ListFiles returns the list of files in the storage
func (s *Storage) ListFiles() ([]os.DirEntry, error) {
	dir, err := os.ReadDir(s.Opts.Root)
	if err != nil {
		return nil, err
	}

	var files []os.DirEntry
	for _, file := range dir {
		if file.Name() != ".dabadee" {
			files = append(files, file)
		}
	}

	return files, nil
}
