package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestImportHardlinksEqualFiles(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store"), PreserveMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	metadata := &Metadata{Mode: 0644, UID: -1, GID: -1}
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	if _, err = storage.Import(context.Background(), first, bytes.NewReader([]byte("same")), ImportOptions{Metadata: metadata}); err != nil {
		t.Fatal(err)
	}
	result, err := storage.Import(context.Background(), second, bytes.NewReader([]byte("same")), ImportOptions{Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WholeFile || result.Reused != 4 || result.Written != 0 {
		t.Fatalf("second import: %+v", result)
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstInfo.Sys().(*syscall.Stat_t).Ino != secondInfo.Sys().(*syscall.Stat_t).Ino {
		t.Fatal("equal files do not share an inode")
	}
}

func TestImportKeepsDifferentMetadataSeparate(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store"), PreserveMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	if _, err = storage.Import(context.Background(), first, bytes.NewReader([]byte("same")), ImportOptions{Metadata: &Metadata{Mode: 0644, UID: -1, GID: -1}}); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.Import(context.Background(), second, bytes.NewReader([]byte("same")), ImportOptions{Metadata: &Metadata{Mode: 0755, UID: -1, GID: -1}}); err != nil {
		t.Fatal(err)
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstInfo.Sys().(*syscall.Stat_t).Ino == secondInfo.Sys().(*syscall.Stat_t).Ino {
		t.Fatal("files with different metadata share an inode")
	}
	if firstInfo.Mode().Perm() != 0644 || secondInfo.Mode().Perm() != 0755 {
		t.Fatalf("modes: %o and %o", firstInfo.Mode().Perm(), secondInfo.Mode().Perm())
	}
}

func TestReuseLinksKnownObjectWithoutReadingContent(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store"), PreserveMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	content := []byte("known")
	metadata := &Metadata{Mode: 0644, UID: -1, GID: -1}
	first := filepath.Join(root, "first")
	result, err := storage.Import(context.Background(), first, bytes.NewReader(content), ImportOptions{Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, "second")
	reused, err := storage.Reuse(context.Background(), second, result.ContentDigest, result.Size, ImportOptions{Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	if !reused {
		t.Fatal("known object was not reused")
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstInfo.Sys().(*syscall.Stat_t).Ino != secondInfo.Sys().(*syscall.Stat_t).Ino {
		t.Fatal("reused object does not share an inode")
	}
}

func TestReuseRejectsUnknownObject(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store")})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	digest := sha256.Sum256([]byte("missing"))
	reused, err := storage.Reuse(context.Background(), filepath.Join(root, "output"), hex.EncodeToString(digest[:]), 7, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if reused {
		t.Fatal("unknown object was reused")
	}
}

func TestContainsChecksKnownObjectWithoutCreatingDestination(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store"), PreserveMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	content := []byte("known")
	metadata := &Metadata{Mode: 0644, UID: -1, GID: -1}
	result, err := storage.Import(context.Background(), filepath.Join(root, "first"), bytes.NewReader(content), ImportOptions{Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	known, err := storage.Contains(context.Background(), result.ContentDigest, result.Size, ImportOptions{Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("known object was not found")
	}
	if _, err = os.Stat(filepath.Join(root, "second")); !os.IsNotExist(err) {
		t.Fatalf("contains created a destination: %v", err)
	}
}

func TestImportPreservesSpecialModeBitsAfterOwnership(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store"), PreserveMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	destination := filepath.Join(root, "destination")
	metadata := &Metadata{Mode: 0755 | os.ModeSetuid, UID: os.Getuid(), GID: os.Getgid()}
	if _, err = storage.Import(context.Background(), destination, strings.NewReader("binary"), ImportOptions{Metadata: metadata}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid == 0 {
		t.Fatalf("setuid bit was cleared: %v", info.Mode())
	}
}

func TestOpenAdoptsV1ObjectLazily(t *testing.T) {
	root := t.TempDir()
	content := []byte("legacy")
	digest := sha256.Sum256(content)
	key := hex.EncodeToString(digest[:])
	if err := os.WriteFile(filepath.Join(root, ".dabadee"), []byte(`{"Root":"legacy"}`), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, key)
	if err := os.WriteFile(legacy, content, 0644); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "output")
	result, err := storage.Import(context.Background(), destination, bytes.NewReader(content), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WholeFile || result.Written != 0 {
		t.Fatalf("legacy import: %+v", result)
	}
	legacyInfo, err := os.Stat(legacy)
	if err != nil {
		t.Fatal(err)
	}
	destinationInfo, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if legacyInfo.Sys().(*syscall.Stat_t).Ino != destinationInfo.Sys().(*syscall.Stat_t).Ino {
		t.Fatal("legacy object was copied instead of adopted")
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCleansInterruptedImport(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".dabadee-v2", "staging")
	if err := os.MkdirAll(staging, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "import-interrupted"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	entries, err := os.ReadDir(staging)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("interrupted imports were retained: %v", entries)
	}
}

func TestImportCancellationLeavesNoPartialObject(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = storage.Import(ctx, filepath.Join(root, "output"), bytes.NewReader([]byte("partial")), ImportOptions{}); err == nil {
		t.Fatal("cancelled import succeeded")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".dabadee-v2", "staging"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cancelled import was retained: %v", entries)
	}
	if _, err = os.Stat(filepath.Join(root, "output")); !os.IsNotExist(err) {
		t.Fatalf("cancelled destination exists: %v", err)
	}
}

func TestImportDoesNotAdoptOverExistingV2Object(t *testing.T) {
	root := t.TempDir()
	content := []byte("legacy")
	digest := sha256.Sum256(content)
	key := hex.EncodeToString(digest[:])
	if err := os.WriteFile(filepath.Join(root, key), content, 0644); err != nil {
		t.Fatal(err)
	}
	objectRoot := filepath.Join(root, ".dabadee-v2", "objects", "sha256")
	if err := os.MkdirAll(objectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, key), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "output")
	result, err := storage.Import(context.Background(), destination, bytes.NewReader(content), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.WholeFile {
		t.Fatalf("corrupt v2 object was reported as reused: %+v", result)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content: %q", got)
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGCRemovesObjectsWithoutExternalLinks(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store")})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "output")
	if _, err = storage.Import(context.Background(), destination, bytes.NewReader([]byte("unused")), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	result, err := storage.GC(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Objects != 1 || result.Bytes != 6 {
		t.Fatalf("gc result: %+v", result)
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectGarbageDoesNotApplyDryRun(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store")})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "output")
	if _, err = storage.Import(context.Background(), destination, bytes.NewReader([]byte("unused")), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	result, err := storage.CollectGarbage(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Objects != 1 || result.Bytes != 6 {
		t.Fatalf("dry-run result: %+v", result)
	}
	result, err = storage.CollectGarbage(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Objects != 1 {
		t.Fatalf("dry run changed the store: %+v", result)
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanGCIncludesLinksReleasedByCaller(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: filepath.Join(root, "store")})
	if err != nil {
		t.Fatal(err)
	}
	destinationRoot := filepath.Join(root, "orphan")
	if _, err = storage.Import(context.Background(), filepath.Join(destinationRoot, "value"), bytes.NewReader([]byte("unused")), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	current, err := storage.CollectGarbage(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if current.Objects != 0 {
		t.Fatalf("linked object reported as garbage: %+v", current)
	}
	planned, err := storage.PlanGC(context.Background(), []string{destinationRoot})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Objects != 1 || planned.Bytes != 6 {
		t.Fatalf("planned gc: %+v", planned)
	}
	if _, err = os.Stat(filepath.Join(destinationRoot, "value")); err != nil {
		t.Fatalf("plan removed destination: %v", err)
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseWritesAnAtomicIndex(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storage.Import(context.Background(), filepath.Join(root, "output"), bytes.NewReader([]byte("indexed")), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = storage.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".dabadee-v2"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name()[0] == '.' {
			t.Fatalf("temporary file remained after close: %s", entry.Name())
		}
	}
}

func TestChunkerProducesAlignedBoundaries(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefgh"), 512*1024)
	chunker, err := newChunker(bytes.NewReader(data), 16*1024, 32*1024, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for {
		chunk, err := chunker.Next()
		if err != nil {
			break
		}
		total += len(chunk)
		if total != len(data) && len(chunk)%chunkAlignment != 0 {
			t.Fatalf("unaligned chunk size: %d", len(chunk))
		}
	}
	if total != len(data) {
		t.Fatalf("chunked bytes: got %d, want %d", total, len(data))
	}
}

func TestImportReusesChunksWhenReflinkIsAvailable(t *testing.T) {
	root := t.TempDir()
	storage, err := Open(Options{
		Root:             filepath.Join(root, "store"),
		MinimumChunkSize: 64 * 1024,
		AverageChunkSize: 128 * 1024,
		MaximumChunkSize: 256 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if !storage.Capabilities().Reflink {
		t.Skip("filesystem does not support reflink")
	}
	first := bytes.Repeat([]byte("a"), 1024*1024)
	second := append([]byte(nil), first...)
	copy(second[400*1024:404*1024], bytes.Repeat([]byte("b"), 4*1024))
	if _, err = storage.Import(context.Background(), filepath.Join(root, "first"), bytes.NewReader(first), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := storage.Import(context.Background(), filepath.Join(root, "second"), bytes.NewReader(second), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reused == 0 || result.Written >= int64(len(second)) {
		t.Fatalf("chunk reuse: %+v", result)
	}
	got, err := os.ReadFile(filepath.Join(root, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, second) {
		t.Fatal("reflinked content differs from the input")
	}
}

func BenchmarkImportSimilarFiles(b *testing.B) {
	data := bytes.Repeat([]byte("abcdefgh"), 512*1024)
	for index := 0; index < b.N; index++ {
		root := b.TempDir()
		storage, err := Open(Options{Root: filepath.Join(root, "store")})
		if err != nil {
			b.Fatal(err)
		}
		if _, err = storage.Import(context.Background(), filepath.Join(root, "first"), bytes.NewReader(data), ImportOptions{}); err != nil {
			b.Fatal(err)
		}
		changed := append([]byte(nil), data...)
		copy(changed[len(changed)/2:len(changed)/2+4096], bytes.Repeat([]byte("z"), 4096))
		result, err := storage.Import(context.Background(), filepath.Join(root, "second"), bytes.NewReader(changed), ImportOptions{})
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(result.Reused), "reused-B/op")
		if err = storage.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
