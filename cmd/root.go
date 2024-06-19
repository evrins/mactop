package cmd

import (
	"github.com/context-labs/mactop/v2/app"
	"github.com/spf13/cobra"
)

var showVersion bool
var colorName string
var updateInterval int

func init() {
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "show version of mactop")
	rootCmd.PersistentFlags().StringVarP(&colorName, "color", "c", "white", "set the UI color. Default is white. Options are 'green', 'red', 'blue', 'cyan', 'magenta', 'yellow', and 'white'.")
	rootCmd.PersistentFlags().IntVarP(&updateInterval, "interval", "i", 1000, "set the powermetrics update interval in milliseconds")
}

var rootCmd = &cobra.Command{
	Use: "mactop",
	Long: `You must use sudo to run mactop, as powermetrics requires root privileges.
For more information, see https://github.com/context-labs/mactop
`,
	Run: func(c *cobra.Command, args []string) {
		app.Start(updateInterval, colorName)
	},
}

func Execute() error {
	return rootCmd.Execute()
}
