package main

import (
	"fmt"
	"os"

	"github.com/mirkobrombin/dabadee/cmd"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{Use: "dabadee"}

	rootCmd.AddCommand(cmd.NewCpCommand())
	rootCmd.AddCommand(cmd.NewDedupCommand())
	rootCmd.AddCommand(cmd.NewFindLinksCommand())
	rootCmd.AddCommand(cmd.NewRmOrphansCommand())
	rootCmd.AddCommand(cmd.NewRmCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
