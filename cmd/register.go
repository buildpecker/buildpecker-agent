package cmd

import (
	"github.com/pthsarmah/buildpecker-agent/internal/api"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this node to ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := api.RegisterNode(args[0])
		return err
	},
}

var deregisterCmd = &cobra.Command{
	Use:   "deregister",
	Short: "Deregister this node from Buildpecker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := api.DeregisterNode(args[0])
		return err
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(deregisterCmd)
}
