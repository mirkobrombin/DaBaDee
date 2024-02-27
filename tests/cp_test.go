package tests

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestCpCommand(t *testing.T) {
	// Create temporary directories
	testPath := filepath.Join(t.TempDir(), "testdata")
	storagePath := filepath.Join(t.TempDir(), "storage")

	err := os.MkdirAll(testPath, 0755)
	assert.Nil(t, err)

	err = os.MkdirAll(storagePath, 0755)
	assert.Nil(t, err)

	// Create test data
	filePath := filepath.Join(testPath, "file-0")
	err = os.WriteFile(filePath, []byte("test"), 0644)
	assert.Nil(t, err)

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

	// Create a processor to test
	cpProcessor := processor.NewCpProcessor(
		filepath.Join(testPath, "file-0"),
		filepath.Join(testPath, "file-0-link"),
		s,
		h,
	)

	d := dabadee.NewDaBaDee(cpProcessor, true)

	// Run the command
	err = d.Run()
	if err != nil {
		t.Fatalf("Error running command: %v", err)
	}

	// Check the link has been created
	_, err = os.Lstat(filepath.Join(testPath, "file-0-link"))
	assert.Nil(t, err)

	// Check if they have same inode
	fileInfo1, err := os.Lstat(filepath.Join(testPath, "file-0"))
	assert.Nil(t, err)
	fileInfo2, err := os.Lstat(filepath.Join(testPath, "file-0-link"))
	assert.Nil(t, err)
	assert.Equal(t, fileInfo1.Sys().(*syscall.Stat_t).Ino, fileInfo2.Sys().(*syscall.Stat_t).Ino)
}
