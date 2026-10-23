// Command cents-cloud runs the server, bot and public gateway on one Machine.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/lazybark/cents/cloud"
	"github.com/spf13/cobra"
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "cents-cloud:", err)
		os.Exit(1)
	}
}

func root() *cobra.Command {
	var path string
	cmd := &cobra.Command{Use: "cents-cloud", Short: "Run the server, Telegram bot and gateway together", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true, RunE: func(*cobra.Command, []string) error {
		cfg, err := cloud.LoadConfig(path)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return cloud.Run(ctx, cfg, os.Stdout)
	}}
	cmd.PersistentFlags().StringVar(&path, "config", "/data/cloud.yml", "config file")
	config := &cobra.Command{Use: "config", Short: "Config file help"}
	config.AddCommand(&cobra.Command{Use: "example", Args: cobra.NoArgs, Run: func(cmd *cobra.Command, _ []string) { fmt.Fprint(cmd.OutOrStdout(), cloud.ExampleConfig) }})
	cmd.AddCommand(config)
	return cmd
}
