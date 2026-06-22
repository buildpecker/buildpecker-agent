package cmd

import (
	"github.com/pthsarmah/buildpecker-agent/internal/api"
	"github.com/spf13/cobra"
)

func RegisterNode(token string) error {
	err := api.RegisterNode(token)
	return err
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this VPS to user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RegisterNode(args[0])
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
}
