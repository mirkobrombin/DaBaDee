package cmd

import (
	"fmt"
	"log"

	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewFindLinksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find-links <source> <storage>",
		Short: "Find all hard links to the specified file",
		Args:  cobra.ExactArgs(2),
		Run:   findLinksCommand,
	}

	cmd.Flags().StringSliceP("additional-paths", "p", []string{}, "Additional paths to search for links")

	return cmd
}

func findLinksCommand(cmd *cobra.Command, args []string) {
	path, storagePath := args[0], args[1]
	additionalPaths, _ := cmd.Flags().GetStringSlice("additional-paths")

	// Create storage
	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	// Find links
	log.Printf("Finding links to %s..", path)
	links, err := s.FindLinks(path, additionalPaths)
	if err != nil {
		log.Fatalf("Error finding links: %v", err)
	}

	// Print links
	for _, link := range links {
		fmt.Printf("- %s\n", link)
	}

	log.Print("Done")
}
