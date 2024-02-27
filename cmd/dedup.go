package cmd

import (
	"log"
	"strconv"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewDedupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dedup [source] [storage] [workers]",
		Short: "Deduplicate files in a directory",
		Args:  cobra.ExactArgs(3),
		Run:   dedupCommand,
	}

	cmd.Flags().BoolP("with-metadata", "m", false, "Include file metadata in hash calculation")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")

	return cmd
}

func dedupCommand(cmd *cobra.Command, args []string) {
	source, storagePath, workersStr := args[0], args[1], args[2]
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")
	verbose, _ := cmd.Flags().GetBool("verbose")
	workers, err := strconv.Atoi(workersStr)
	if err != nil {
		log.Fatalf("Invalid number of workers: %v", err)
	}

	// Create storage
	storageOpts := storage.StorageOptions{
		Root:         storagePath,
		WithMetadata: withMetadata,
	}
	s, err := storage.NewStorage(storageOpts)
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	// Create hash generator
	h := hash.NewSHA256Generator()

	// Create processor
	processor := processor.NewDedupProcessor(source, s, h, workers)

	// Run the processor
	log.Printf("Deduplicating %s..", source)
	d := dabadee.NewDaBaDee(processor, verbose)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during deduplication: %v", err)
	}

	log.Print("Done")
}
