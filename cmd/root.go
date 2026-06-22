package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "buildpecker-agent",
	Short: "VPS Agent for buildpecker.sh",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Oops. An error occurred! '%s'\n", err)
		os.Exit(1)
	}
}
