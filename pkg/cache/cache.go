package cache

import (
	"encoding/json"
	"os"
)

type CacheEntry struct {
	ModTime int64  `json:"mod_time"`
	Size    int64  `json:"size"`
	Hash    string `json:"hash"`
}

type Cache struct {
	Path    string                `json:"-"`
	Entries map[string]CacheEntry `json:"entries"`
}

func Load(path string) (*Cache, error) {
	c := &Cache{Path: path, Entries: make(map[string]CacheEntry)}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&c.Entries); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cache) Save() error {
	if c == nil {
		return nil
	}
	f, err := os.Create(c.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(c.Entries)
}

func (c *Cache) Get(path string) (CacheEntry, bool) {
	if c == nil {
		return CacheEntry{}, false
	}
	e, ok := c.Entries[path]
	return e, ok
}

func (c *Cache) Update(path string, entry CacheEntry) {
	if c == nil {
		return
	}
	c.Entries[path] = entry
}
