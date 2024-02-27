package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestDedupCommand(t *testing.T) {
	// Create temporary directories
	testPath := filepath.Join(t.TempDir(), "testdata")
	storagePath := filepath.Join(t.TempDir(), "storage")

	err := os.MkdirAll(testPath, 0755)
	assert.Nil(t, err)

	err = os.MkdirAll(storagePath, 0755)
	assert.Nil(t, err)

	// Create test data
	const diffTestFiles = 100
	for i := 0; i < diffTestFiles; i++ {
		filePath := filepath.Join(testPath, fmt.Sprintf("file-%d", i))
		err = os.WriteFile(filePath, []byte(fmt.Sprintf("test-%d", i)), 0644)
		assert.Nil(t, err)
	}

	const sameTestFiles = 50
	for i := 0; i < sameTestFiles; i++ {
		filePath := filepath.Join(testPath, fmt.Sprintf("same-file-%d", i))
		err = os.WriteFile(filePath, []byte("test"), 0644)
		assert.Nil(t, err)
	}

	// Create instances of the required components
	storageOpts := storage.StorageOptions{
		Root:         storagePath,
		WithMetadata: true,
	}
	s, err := storage.NewStorage(storageOpts)
	if err != nil {
		t.Fatalf("Error creating storage: %v", err)
	}

	h := hash.NewSHA256Generator()

	processor := processor.NewDedupProcessor(testPath, s, h, 1)

	d := dabadee.NewDaBaDee(processor, true)

	// Run the deduplication
	err = d.Run()
	if err != nil {
		t.Fatalf("Error during deduplication: %v", err)
	}

	// Check the results
	files, err := s.ListFiles()
	assert.Nil(t, err)
	assert.Equal(t, diffTestFiles+1, len(files))

	t.Logf("There are %d files in the storage (%d different files + 50 duplicated files treated as one)", len(files), diffTestFiles)
}
