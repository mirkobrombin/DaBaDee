// Package store provides a content-addressed file store with whole-file and
// content-defined chunk deduplication.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const formatVersion = 2

var ErrClosed = errors.New("store: closed")

type Options struct {
	Root             string `json:"root"`
	PreserveMetadata bool   `json:"preserve_metadata"`
	MinimumChunkSize int    `json:"minimum_chunk_size,omitempty"`
	AverageChunkSize int    `json:"average_chunk_size,omitempty"`
	MaximumChunkSize int    `json:"maximum_chunk_size,omitempty"`
}

type Metadata struct {
	Mode fs.FileMode
	UID  int
	GID  int
}

type ImportOptions struct {
	Metadata *Metadata
}

type ImportResult struct {
	ContentDigest string
	ObjectKey     string
	Size          int64
	Written       int64
	Reused        int64
	WholeFile     bool
}

type TreeResult struct {
	Files   int
	Written int64
	Reused  int64
}

type GCResult struct {
	Objects int
	Bytes   int64
	Chunks  int
}

type Capabilities struct {
	Reflink bool
}

type Store struct {
	options     Options
	stateRoot   string
	objectsRoot string
	stagingRoot string
	indexPath   string
	lock        *os.File
	index       storeIndex
	reflink     bool
	dirty       bool
	closed      bool
}

type storeIndex struct {
	Version int                      `json:"version"`
	Objects map[string]objectRecord  `json:"objects"`
	Chunks  map[string][]chunkRecord `json:"chunks"`
}

type objectRecord struct {
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	ModTimeNS int64  `json:"mod_time_ns"`
}

type chunkRecord struct {
	Object string `json:"object"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
}

type importedChunk struct {
	digest string
	offset int64
	size   int64
}

type formatConfig struct {
	Version      int  `json:"version"`
	LegacyFormat bool `json:"legacy_format"`
}

func Open(options Options) (*Store, error) {
	if options.Root == "" {
		return nil, fmt.Errorf("store: root is required")
	}
	applyChunkDefaults(&options)
	if _, err := newChunker(strings.NewReader(""), options.MinimumChunkSize, options.AverageChunkSize, options.MaximumChunkSize); err != nil {
		return nil, err
	}

	stateRoot := filepath.Join(options.Root, ".dabadee-v2")
	store := &Store{
		options:     options,
		stateRoot:   stateRoot,
		objectsRoot: filepath.Join(stateRoot, "objects", "sha256"),
		stagingRoot: filepath.Join(stateRoot, "staging"),
		indexPath:   filepath.Join(stateRoot, "index.json"),
		index: storeIndex{
			Version: formatVersion,
			Objects: make(map[string]objectRecord),
			Chunks:  make(map[string][]chunkRecord),
		},
	}
	for _, path := range []string{options.Root, store.objectsRoot, store.stagingRoot} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("store: create %s: %w", path, err)
		}
	}
	lock, err := acquireLock(filepath.Join(stateRoot, "lock"))
	if err != nil {
		return nil, err
	}
	store.lock = lock
	if err = store.loadIndex(); err != nil {
		store.Close()
		return nil, err
	}
	if err = store.writeFormatConfig(); err != nil {
		store.Close()
		return nil, err
	}
	if err = store.cleanStaging(); err != nil {
		store.Close()
		return nil, err
	}
	store.reflink = store.probeReflink()
	return store, nil
}

func (s *Store) Capabilities() Capabilities {
	return Capabilities{Reflink: s.reflink}
}

func (s *Store) Import(ctx context.Context, destination string, reader io.Reader, options ImportOptions) (ImportResult, error) {
	if s.closed {
		return ImportResult{}, ErrClosed
	}
	if destination == "" {
		return ImportResult{}, fmt.Errorf("store: destination is required")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return ImportResult{}, fmt.Errorf("store: create destination directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.stagingRoot, "import-")
	if err != nil {
		return ImportResult{}, fmt.Errorf("store: create temporary object: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()

	chunkReader, err := newChunker(reader, s.options.MinimumChunkSize, s.options.AverageChunkSize, s.options.MaximumChunkSize)
	if err != nil {
		return ImportResult{}, err
	}
	contentHash := sha256.New()
	chunks := make([]importedChunk, 0, 16)
	result := ImportResult{}
	for {
		if err = ctx.Err(); err != nil {
			return ImportResult{}, err
		}
		data, readErr := chunkReader.Next()
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return ImportResult{}, fmt.Errorf("store: read content: %w", readErr)
		}
		if _, err = contentHash.Write(data); err != nil {
			return ImportResult{}, fmt.Errorf("store: hash content: %w", err)
		}
		digest := sha256.Sum256(data)
		chunkDigest := hex.EncodeToString(digest[:])
		reused := s.cloneChunk(temporary, chunkDigest, result.Size, int64(len(data)))
		if !reused {
			if _, err = temporary.WriteAt(data, result.Size); err != nil {
				return ImportResult{}, fmt.Errorf("store: write chunk: %w", err)
			}
			result.Written += int64(len(data))
		} else {
			result.Reused += int64(len(data))
		}
		chunks = append(chunks, importedChunk{digest: chunkDigest, offset: result.Size, size: int64(len(data))})
		result.Size += int64(len(data))
	}
	if err = temporary.Truncate(result.Size); err != nil {
		return ImportResult{}, fmt.Errorf("store: truncate object: %w", err)
	}
	if err = applyMetadata(temporaryPath, options.Metadata); err != nil {
		return ImportResult{}, err
	}
	if err = temporary.Sync(); err != nil {
		return ImportResult{}, fmt.Errorf("store: sync object: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return ImportResult{}, fmt.Errorf("store: close object: %w", err)
	}

	result.ContentDigest = hex.EncodeToString(contentHash.Sum(nil))
	result.ObjectKey = objectKey(result.ContentDigest, options.Metadata, s.options.PreserveMetadata)
	objectPath := filepath.Join(s.objectsRoot, result.ObjectKey)
	if adopted, adoptErr := s.adoptLegacyObject(objectPath, result.ObjectKey, result.ContentDigest, result.Size); adoptErr != nil {
		return ImportResult{}, adoptErr
	} else if adopted {
		result.WholeFile = true
	}
	matches, matchErr := s.objectMatches(objectPath, result.ContentDigest, result.Size)
	if matchErr != nil {
		return ImportResult{}, matchErr
	}
	if matches {
		if err = replaceWithLink(objectPath, destination); err != nil {
			return ImportResult{}, err
		}
		if err = s.recordObject(result.ObjectKey, result.ContentDigest, result.Size, chunks); err != nil {
			return ImportResult{}, err
		}
		result.WholeFile = true
		result.Reused = result.Size
		result.Written = 0
		return result, nil
	}
	if err = os.Rename(temporaryPath, objectPath); err != nil {
		return ImportResult{}, fmt.Errorf("store: publish object: %w", err)
	}
	if err = replaceWithLink(objectPath, destination); err != nil {
		return ImportResult{}, err
	}
	if err = s.recordObject(result.ObjectKey, result.ContentDigest, result.Size, chunks); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

func (s *Store) ImportFile(ctx context.Context, source, destination string, options ImportOptions) (ImportResult, error) {
	file, err := os.Open(source)
	if err != nil {
		return ImportResult{}, fmt.Errorf("store: open source: %w", err)
	}
	defer file.Close()
	return s.Import(ctx, destination, file, options)
}

// Contains reports whether an object can be reused without reading its content.
func (s *Store) Contains(ctx context.Context, contentDigest string, size int64, options ImportOptions) (bool, error) {
	if s.closed {
		return false, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	key, err := s.reusableObjectKey(contentDigest, size, options)
	if err != nil {
		return false, err
	}
	objectPath := filepath.Join(s.objectsRoot, key)
	if _, err = s.adoptLegacyObject(objectPath, key, contentDigest, size); err != nil {
		return false, err
	}
	return s.objectMatches(objectPath, contentDigest, size)
}

func (s *Store) Reuse(ctx context.Context, destination, contentDigest string, size int64, options ImportOptions) (bool, error) {
	if destination == "" {
		return false, fmt.Errorf("store: invalid reusable object")
	}
	known, err := s.Contains(ctx, contentDigest, size, options)
	if err != nil || !known {
		return false, err
	}
	key := objectKey(contentDigest, options.Metadata, s.options.PreserveMetadata)
	objectPath := filepath.Join(s.objectsRoot, key)
	if err = replaceWithLink(objectPath, destination); err != nil {
		return false, err
	}
	if err = s.recordObject(key, contentDigest, size, nil); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) reusableObjectKey(contentDigest string, size int64, options ImportOptions) (string, error) {
	if size < 0 || len(contentDigest) != sha256.Size*2 {
		return "", fmt.Errorf("store: invalid reusable object")
	}
	if _, err := hex.DecodeString(contentDigest); err != nil {
		return "", fmt.Errorf("store: invalid reusable object")
	}
	return objectKey(contentDigest, options.Metadata, s.options.PreserveMetadata), nil
}

func (s *Store) DeduplicateTree(ctx context.Context, root string) (TreeResult, error) {
	result := TreeResult{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if samePath(path, s.stateRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		metadata := MetadataFromInfo(info)
		imported, err := s.ImportFile(ctx, path, path, ImportOptions{Metadata: &metadata})
		if err != nil {
			return err
		}
		result.Files++
		result.Written += imported.Written
		result.Reused += imported.Reused
		return nil
	})
	return result, err
}

func (s *Store) GC(ctx context.Context) (GCResult, error) {
	return s.collectGarbage(ctx, true, nil)
}

func (s *Store) CollectGarbage(ctx context.Context, apply bool) (GCResult, error) {
	return s.collectGarbage(ctx, apply, nil)
}

func (s *Store) PlanGC(ctx context.Context, releasedPaths []string) (GCResult, error) {
	released, err := releasedLinks(ctx, releasedPaths)
	if err != nil {
		return GCResult{}, err
	}
	return s.collectGarbage(ctx, false, released)
}

func (s *Store) collectGarbage(ctx context.Context, apply bool, released map[fileIdentity]uint64) (GCResult, error) {
	if s.closed {
		return GCResult{}, ErrClosed
	}
	result := GCResult{}
	entries, err := os.ReadDir(s.objectsRoot)
	if err != nil {
		return result, fmt.Errorf("store: list objects: %w", err)
	}
	removed := make(map[string]bool)
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if !entry.Type().IsRegular() || !validObjectKey(entry.Name()) {
			continue
		}
		path := filepath.Join(s.objectsRoot, entry.Name())
		info, statErr := os.Stat(path)
		if statErr != nil {
			return result, fmt.Errorf("store: stat object: %w", statErr)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(stat.Nlink) > 1+released[fileIdentity{device: stat.Dev, inode: stat.Ino}] {
			continue
		}
		removed[entry.Name()] = true
		result.Objects++
		result.Bytes += info.Size()
		if !apply {
			continue
		}
		if err = os.Remove(path); err != nil {
			return result, fmt.Errorf("store: remove object: %w", err)
		}
		delete(s.index.Objects, entry.Name())
	}
	for digest, records := range s.index.Chunks {
		kept := records[:0]
		for _, record := range records {
			if removed[record.Object] || !s.validChunkRecord(record) {
				result.Chunks++
				if !apply {
					kept = append(kept, record)
				}
				continue
			}
			kept = append(kept, record)
		}
		if !apply {
			continue
		}
		if len(kept) == 0 {
			delete(s.index.Chunks, digest)
			continue
		}
		s.index.Chunks[digest] = kept
	}
	if apply && (result.Objects > 0 || result.Chunks > 0) {
		s.dirty = true
	}
	return result, nil
}

type fileIdentity struct {
	device uint64
	inode  uint64
}

func releasedLinks(ctx context.Context, roots []string) (map[fileIdentity]uint64, error) {
	result := make(map[fileIdentity]uint64)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if ok {
				result[fileIdentity{device: stat.Dev, inode: stat.Ino}]++
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("store: inspect released path: %w", err)
		}
	}
	return result, nil
}

func (s *Store) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	var result error
	if s.dirty {
		result = s.saveIndex()
	}
	if err := releaseLock(s.lock); result == nil {
		result = err
	}
	return result
}

func MetadataFromInfo(info fs.FileInfo) Metadata {
	metadata := Metadata{Mode: info.Mode(), UID: -1, GID: -1}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		metadata.UID = int(stat.Uid)
		metadata.GID = int(stat.Gid)
	}
	return metadata
}

func applyChunkDefaults(options *Options) {
	if options.MinimumChunkSize == 0 {
		options.MinimumChunkSize = 64 * 1024
	}
	if options.AverageChunkSize == 0 {
		options.AverageChunkSize = 256 * 1024
	}
	if options.MaximumChunkSize == 0 {
		options.MaximumChunkSize = 1024 * 1024
	}
}

func (s *Store) cloneChunk(destination *os.File, digest string, offset, size int64) bool {
	if !s.reflink {
		return false
	}
	for _, record := range s.index.Chunks[digest] {
		if record.Size != size || !s.validChunkRecord(record) {
			continue
		}
		source, err := os.Open(filepath.Join(s.objectsRoot, record.Object))
		if err != nil {
			continue
		}
		err = reflinkRange(source, destination, record.Offset, offset, size)
		source.Close()
		if err == nil {
			return true
		}
	}
	return false
}

func (s *Store) validChunkRecord(record chunkRecord) bool {
	object, found := s.index.Objects[record.Object]
	if !found || record.Offset < 0 || record.Size <= 0 || record.Offset+record.Size > object.Size {
		return false
	}
	info, err := os.Stat(filepath.Join(s.objectsRoot, record.Object))
	return err == nil && info.Mode().IsRegular() && info.Size() == object.Size && info.ModTime().UnixNano() == object.ModTimeNS
}

func (s *Store) addChunkRecord(digest string, record chunkRecord) {
	records := s.index.Chunks[digest]
	for _, current := range records {
		if current == record {
			return
		}
	}
	if len(records) >= 4 {
		return
	}
	s.index.Chunks[digest] = append(records, record)
}

func (s *Store) recordObject(key, content string, size int64, chunks []importedChunk) error {
	info, err := os.Stat(filepath.Join(s.objectsRoot, key))
	if err != nil {
		return fmt.Errorf("store: stat object: %w", err)
	}
	s.index.Objects[key] = objectRecord{Content: content, Size: size, ModTimeNS: info.ModTime().UnixNano()}
	if s.reflink {
		for _, chunk := range chunks {
			s.addChunkRecord(chunk.digest, chunkRecord{Object: key, Offset: chunk.offset, Size: chunk.size})
		}
	}
	s.dirty = true
	return nil
}

func (s *Store) objectMatches(path, content string, size int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: stat object: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return false, nil
	}
	key := filepath.Base(path)
	if record, found := s.index.Objects[key]; found && record.Content == content && record.Size == size && record.ModTimeNS == info.ModTime().UnixNano() {
		return true, nil
	}
	digest, err := digestFile(path)
	if err != nil {
		return false, err
	}
	return digest == content, nil
}

func (s *Store) adoptLegacyObject(objectPath, key, content string, size int64) (bool, error) {
	legacyPath := filepath.Join(s.options.Root, key)
	info, err := os.Stat(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: stat legacy object: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return false, nil
	}
	digest, err := digestFile(legacyPath)
	if err != nil {
		return false, err
	}
	if digest != content {
		return false, nil
	}
	if err = os.Link(legacyPath, objectPath); err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: adopt legacy object: %w", err)
	}
	objectInfo, err := os.Stat(objectPath)
	if err != nil {
		return false, fmt.Errorf("store: stat adopted object: %w", err)
	}
	s.index.Objects[key] = objectRecord{Content: content, Size: size, ModTimeNS: objectInfo.ModTime().UnixNano()}
	s.dirty = true
	return true, nil
}

func (s *Store) probeReflink() bool {
	source, err := os.CreateTemp(s.stagingRoot, "probe-source-")
	if err != nil {
		return false
	}
	sourcePath := source.Name()
	defer os.Remove(sourcePath)
	defer source.Close()
	destination, err := os.CreateTemp(s.stagingRoot, "probe-destination-")
	if err != nil {
		return false
	}
	destinationPath := destination.Name()
	defer os.Remove(destinationPath)
	defer destination.Close()
	if err = source.Truncate(chunkAlignment); err != nil {
		return false
	}
	return reflinkRange(source, destination, 0, 0, chunkAlignment) == nil
}

func (s *Store) loadIndex() error {
	file, err := os.Open(s.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("store: open index: %w", err)
	}
	defer file.Close()
	var index storeIndex
	if err = json.NewDecoder(file).Decode(&index); err != nil {
		return fmt.Errorf("store: decode index: %w", err)
	}
	if index.Version != formatVersion {
		return fmt.Errorf("store: unsupported index version %d", index.Version)
	}
	if index.Objects == nil {
		index.Objects = make(map[string]objectRecord)
	}
	if index.Chunks == nil {
		index.Chunks = make(map[string][]chunkRecord)
	}
	s.index = index
	return nil
}

func (s *Store) saveIndex() error {
	return writeJSONAtomic(s.indexPath, s.index)
}

func (s *Store) writeFormatConfig() error {
	_, err := os.Stat(filepath.Join(s.options.Root, ".dabadee"))
	config := formatConfig{Version: formatVersion, LegacyFormat: err == nil}
	return writeJSONAtomic(filepath.Join(s.stateRoot, "config.json"), config)
}

func (s *Store) cleanStaging() error {
	entries, err := os.ReadDir(s.stagingRoot)
	if err != nil {
		return fmt.Errorf("store: list staging: %w", err)
	}
	for _, entry := range entries {
		if err = os.RemoveAll(filepath.Join(s.stagingRoot, entry.Name())); err != nil {
			return fmt.Errorf("store: clean staging: %w", err)
		}
	}
	return nil
}

func objectKey(content string, metadata *Metadata, preserve bool) string {
	if !preserve || metadata == nil {
		return content
	}
	value := strconv.Itoa(metadata.UID) + "-" + strconv.Itoa(metadata.GID) + "-" + strconv.FormatUint(uint64(metadata.Mode), 10)
	digest := sha256.Sum256([]byte(value))
	return content + "-" + hex.EncodeToString(digest[:])
}

func applyMetadata(path string, metadata *Metadata) error {
	if metadata == nil {
		return nil
	}
	if metadata.UID >= 0 || metadata.GID >= 0 {
		if err := os.Chown(path, metadata.UID, metadata.GID); err != nil {
			return fmt.Errorf("store: set object owner: %w", err)
		}
	}
	if err := os.Chmod(path, metadata.Mode); err != nil {
		return fmt.Errorf("store: set object mode: %w", err)
	}
	return nil
}

func replaceWithLink(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("store: create destination directory: %w", err)
	}
	temporary := filepath.Join(filepath.Dir(destination), "."+filepath.Base(destination)+".dabadee-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	if err := os.Link(source, temporary); err != nil {
		return fmt.Errorf("store: link destination: %w", err)
	}
	defer os.Remove(temporary)
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("store: replace destination: %w", err)
	}
	return nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("store: open object: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("store: hash object: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func acquireLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("store: open lock: %w", err)
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("store: acquire lock: %w", err)
	}
	return file, nil
}

func releaseLock(file *os.File) error {
	if file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return fmt.Errorf("store: release lock: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("store: close lock: %w", closeErr)
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(value); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func validObjectKey(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if len(part) != sha256.Size*2 {
			return false
		}
		if _, err := hex.DecodeString(part); err != nil {
			return false
		}
	}
	return true
}

func samePath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftPath == rightPath
}
