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
	var additionalPaths []string

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

	var findLinksCmd = &cobra.Command{
		Use:   "find-links [source] [storage]",
		Short: "Find all hard links to the specified file",
		Args:  cobra.ExactArgs(2),
		Run:   findLinksCommand,
	}
	findLinksCmd.Flags().StringSliceVarP(&additionalPaths, "additional-paths", "p", []string{}, "Additional paths to search for links")

	var removeOrphansCmd = &cobra.Command{
		Use:   "remove-orphans [storage]",
		Short: "Remove all orphaned files from the storage",
		Args:  cobra.ExactArgs(1),
		Run:   removeOrphansCommand,
	}

	var rmCmd = &cobra.Command{
		Use:   "rm [source] [storage]",
		Short: "Remove a file and its link from storage",
		Args:  cobra.ExactArgs(2),
		Run:   removeCommand,
	}

	rootCmd.AddCommand(cpCmd, dedupCmd, findLinksCmd, removeOrphansCmd, rmCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cpCommand(cmd *cobra.Command, args []string) {
	source, dest, storagePath := args[0], args[1], args[2]
	withMetadata, _ := cmd.Flags().GetBool("with-metadata")

	storageOpts := storage.StorageOptions{
		Root:         storagePath,
		WithMetadata: withMetadata,
	}

	s, err := storage.NewStorage(storageOpts)
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

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

	storageOpts := storage.StorageOptions{
		Root:         storagePath,
		WithMetadata: withMetadata,
	}

	s, err := storage.NewStorage(storageOpts)
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	h := hash.NewSHA256Generator()
	processor := processor.NewDedupProcessor(source, s, h, workers, withMetadata)
	d := dabadee.NewDaBaDee(processor)
	if err := d.Run(); err != nil {
		log.Fatalf("Error during deduplication: %v", err)
	}
}

func findLinksCommand(cmd *cobra.Command, args []string) {
	path, storagePath := args[0], args[1]

	additionalPaths, _ := cmd.Flags().GetStringSlice("additional-paths")

	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	links, err := s.FindLinks(path, additionalPaths)
	if err != nil {
		log.Fatalf("Error finding links: %v", err)
	}

	for _, link := range links {
		fmt.Println(link)
	}
}

func removeOrphansCommand(cmd *cobra.Command, args []string) {
	storagePath := args[0]

	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	err = s.RemoveOrphans()
	if err != nil {
		log.Fatalf("Error removing orphans: %v", err)
	}
}

func removeCommand(cmd *cobra.Command, args []string) {
	source, storagePath := args[0], args[1]

	s, err := storage.NewStorage(storage.StorageOptions{Root: storagePath})
	if err != nil {
		log.Fatalf("Error creating storage: %v", err)
	}

	err = s.RemoveFile(source)
	if err != nil {
		log.Fatalf("Error removing file: %v", err)
	}
}
