package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pthsarmah/buildpecker-agent/internal/api"
	"github.com/pthsarmah/buildpecker-agent/internal/deploy"
	"github.com/pthsarmah/buildpecker-agent/internal/metrics"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run buildpecker-agent as a long-running service",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		fmt.Println("buildpecker-agent daemon started")

		shutdownMetrics, err := metrics.Start(ctx, "", time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "metrics push init: %v\n", err)
		} else {
			defer func() {
				sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = shutdownMetrics(sctx)
			}()
		}

		go func() {
			if err := api.SendHeartbeat(ctx); err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "could not send heartbeat: %v\n", err)
			}
		}()

		if err := deploy.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "daemon exited: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("buildpecker-agent daemon stopped")
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
