package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <source> <storage-path>",
		Short: "Remove a file from the storage",
		Args:  cobra.ExactArgs(2),
		Run:   rmCommand,
	}

	cmd.Flags().BoolP("yes", "y", false, "Assume yes; do not prompt")

	return cmd
}

func rmCommand(cmd *cobra.Command, args []string) {
	source, storagePath := args[0], args[1]
	assumeYes, _ := cmd.Flags().GetBool("yes")

	// Create storage
	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	// Prompt
	if !assumeYes {
		fmt.Printf("This will remove %s and its link from the storage. Continue? [y/N] ", source)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			log.Print("Aborting")
			return
		}
	}

	// Remove file
	err = s.RemoveFile(source)
	if err != nil {
		log.Fatalf("Error removing file: %v", err)
	}

	log.Print("Done")
}
