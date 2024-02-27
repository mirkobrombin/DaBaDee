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
		Use:   "cp [source] [dest] [storage]",
		Short: "Copy a file and deduplicate it in storage",
		Args:  cobra.ExactArgs(3),
		Run:   cpCommand,
	}

	cmd.Flags().BoolP("with-metadata", "m", false, "Include file metadata in hash calculation")

	return cmd
}

func cpCommand(cmd *cobra.Command, args []string) {
	source, dest, storagePath := args[0], args[1], args[2]
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")

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
	processor := processor.NewCpProcessor(source, dest, s, h)

	// Run the processor
	log.Printf("Copying %s to %s..", source, dest)
	d := dabadee.NewDaBaDee(processor)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during copy and link: %v", err)
	}

	log.Print("Done")
}
