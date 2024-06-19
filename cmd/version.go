package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var version = "v0.1.8"

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(c *cobra.Command, args []string) {
		fmt.Printf("version: %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
