package cmd

import (
	"encoding/json"
	"log"
	"os"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func NewDedupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dedup [source]",
		Short: "Deduplicate files in a directory",
		Args:  cobra.ExactArgs(1),
		Run:   dedupCommand,
	}

	cmd.Flags().BoolP("with-metadata", "m", false, "Include file metadata in hash calculation")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")
	cmd.Flags().String("manifest-output", "", "Output manifest file to the given path")
	cmd.Flags().String("dest", "", "Destination directory for copying deduplicated files")
	cmd.Flags().String("storage", "", "Storage directory for deduplicated files")
	cmd.Flags().Int("workers", 1, "Number of workers to use")

	return cmd
}

func dedupCommand(cmd *cobra.Command, args []string) {
	source := args[0]
	storagePath, _ := cmd.Flags().GetString("storage")
	if storagePath == "" {
		storagePath = GetDefaultStoragePath()
	}
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")
	verbose, _ := cmd.Flags().GetBool("verbose")
	outputManifest, _ := cmd.Flags().GetString("manifest-output")
	destDir, _ := cmd.Flags().GetString("dest")
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

	// Create processor
	processor := processor.NewDedupProcessor(source, destDir, s, h, workers)

	// Run the processor
	log.Printf("Deduplicating %s..", source)
	d := dabadee.NewDaBaDee(processor, verbose)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during deduplication: %v", err)
	}

	// Output manifest
	if outputManifest != "" {
		log.Printf("Writing manifest to %s..", outputManifest)

		manifest, err := json.Marshal(processor.FileMap)
		if err != nil {
			log.Fatalf("Error marshalling manifest: %v\n\nPrinting to stdout instead:\n\n%v", err, processor.FileMap)
		}

		if err := os.WriteFile(outputManifest, manifest, 0644); err != nil {
			log.Fatalf("Error writing manifest: %v\n\nPrinting to stdout instead:\n\n%v", err, processor.FileMap)
		}
	}

	log.Print("Done")
}
