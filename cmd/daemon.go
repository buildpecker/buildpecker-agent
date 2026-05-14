package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pthsarmah/forge-agent/internal/api"
	"github.com/pthsarmah/forge-agent/internal/deploy"
	"github.com/pthsarmah/forge-agent/internal/metrics"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run forge-agent as a long-running service",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		fmt.Println("forge-agent daemon started")

		go metrics.StartCollector(ctx, 5*time.Second)
		go func() {
			if err := metrics.Serve(ctx, ":2112"); err != nil {
				fmt.Fprintf(os.Stderr, "metrics server: %v\n", err)
			}
		}()

		go func() {
			if err := api.SendHeartbeat(ctx); err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "could not send heartbeat: %v\n", err)
			}
		}()

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
