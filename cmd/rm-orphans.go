package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewRmOrphansCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm-orphans <storage>",
		Short: "Remove all orphaned files from the storage",
		Args:  cobra.ExactArgs(1),
		Run:   rmOrphansCommand,
	}

	cmd.Flags().BoolP("yes", "y", false, "Assume yes; do not prompt")

	return cmd
}

func rmOrphansCommand(cmd *cobra.Command, args []string) {
	storagePath := args[0]
	assumeYes, _ := cmd.Flags().GetBool("yes")

	// Create storage
	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	// Prompt
	if !assumeYes {
		fmt.Print("This will remove all orphaned files from the storage. Continue? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			log.Print("Aborting")
			return
		}
	}

	// Remove orphans
	log.Print("Removing orphans..")
	err = s.RemoveOrphans()
	if err != nil {
		log.Fatalf("Error removing orphans: %v", err)
	}

	log.Print("Done")
}
