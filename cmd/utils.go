package cmd

import (
	"os/user"
	"path/filepath"
)

// GetDefaultStoragePath determines the default storage path based on user
// and execution context
func GetDefaultStoragePath() string {
	currentUser, err := user.Current()
	if err != nil {
		return "/opt/.dabadee/Storage"
	}

	if currentUser.Uid == "0" {
		// Running as root
		return "/opt/dabadee/Storage"
	}

	// Running as non-root user
	return filepath.Join(currentUser.HomeDir, ".dabadee/Storage")
}
