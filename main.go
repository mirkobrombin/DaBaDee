package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/mirkobrombin/dabadee/pkg/dabadee"
	"github.com/mirkobrombin/dabadee/pkg/hash"
	"github.com/mirkobrombin/dabadee/pkg/processor"
	"github.com/mirkobrombin/dabadee/pkg/storage"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{Use: "dabadee"}

	var withMetadata bool

	var cpCmd = &cobra.Command{
		Use:   "cp [source] [dest] [storage]",
		Short: "Copy a file and deduplicate it in storage",
		Args:  cobra.ExactArgs(3),
		Run:   cpCommand,
	}
	cpCmd.Flags().BoolVarP(&withMetadata, "with-metadata", "m", false, "Include file metadata in hash calculation")

	var dedupCmd = &cobra.Command{
		Use:   "dedup [source] [storage] [workers]",
		Short: "Deduplicate files in a directory",
		Args:  cobra.ExactArgs(3),
		Run:   dedupCommand,
	}
	dedupCmd.Flags().BoolVarP(&withMetadata, "with-metadata", "m", false, "Include file metadata in hash calculation")

	rootCmd.AddCommand(cpCmd, dedupCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cpCommand(cmd *cobra.Command, args []string) {
	source, dest, storagePath := args[0], args[1], args[2]
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")

	s := storage.NewStorage(storagePath)
	h := hash.NewSHA256Generator()
	processor := processor.NewCpProcessor(source, dest, s, h, withMetadata)
	d := dabadee.NewDaBaDee(processor)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during copy and link: %v", err)
	}
}

func dedupCommand(cmd *cobra.Command, args []string) {
	source, storagePath, workersStr := args[0], args[1], args[2]
	workers, err := strconv.Atoi(workersStr)
	if err != nil {
		log.Fatalf("Invalid number of workers: %v", err)
	}

	withMetadata, _ := cmd.Flags().GetBool("with-metadata")

	s := storage.NewStorage(storagePath)
	h := hash.NewSHA256Generator()
	processor := processor.NewDedupProcessor(source, s, h, workers, withMetadata)
	d := dabadee.NewDaBaDee(processor)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during deduplication: %v", err)
	}
}
