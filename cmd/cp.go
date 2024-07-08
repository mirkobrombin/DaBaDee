package cmd

import (
	"log"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewCpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cp [source] [dest]",
		Short: "Copy a file and deduplicate it in storage",
		Args:  cobra.ExactArgs(2),
		Run:   cpCommand,
	}

	cmd.Flags().BoolP("with-metadata", "m", false, "Include file metadata in hash calculation")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")
	cmd.Flags().BoolP("append", "a", false, "Append directory contents to destination")
	cmd.Flags().String("storage", "", "Storage directory for deduplicated files")
	cmd.Flags().Int("workers", 1, "Number of workers to use")

	return cmd
}

func cpCommand(cmd *cobra.Command, args []string) {
	source, dest := args[0], args[1]
	storagePath, _ := cmd.Flags().GetString("storage")
	if storagePath == "" {
		storagePath = GetDefaultStoragePath()
	}
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")
	verbose, _ := cmd.Flags().GetBool("verbose")
	appendFlag, _ := cmd.Flags().GetBool("append")
	workers, _ := cmd.Flags().GetInt("workers")

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

	// Create processor based on the append flag
	var proc processor.Processor
	if appendFlag {
		proc = processor.NewDedupProcessor(source, dest, s, h, workers)
	} else {
		proc = processor.NewCpProcessor(source, dest, s, h)
	}

	// Run the processor
	log.Printf("Copying %s to %s..", source, dest)
	d := dabadee.NewDaBaDee(proc, verbose)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during copy and link: %v", err)
	}

	log.Print("Done")
}
