package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveReplacesCacheAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cache.json")
	cache := &Cache{Path: path, Entries: map[string]CacheEntry{"file": {Size: 42, Hash: "hash"}}}
	if err := cache.Save(); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	if loaded.Entries["file"].Hash != "hash" {
		t.Fatalf("unexpected cached entry: %+v", loaded.Entries)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "cache.json" {
			t.Fatalf("unexpected temporary cache file: %s", entry.Name())
		}
	}
}
