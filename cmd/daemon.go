package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pthsarmah/forge-agent/internal/deploy"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run forge-agent as a long-running service",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		fmt.Println("forge-agent daemon started")
		if err := deploy.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "daemon exited: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("forge-agent daemon stopped")
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
