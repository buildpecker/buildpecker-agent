package cmd

import (
	"github.com/pthsarmah/forge/internal/api"
	"github.com/spf13/cobra"
)

func RegisterNode(authToken string) {
	api.RegisterNode(authToken)
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this VPS to user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RegisterNode(args[0])
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
}
