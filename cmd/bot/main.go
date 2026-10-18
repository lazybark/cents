// Command cents-bot is cents' Telegram bot (see package bot).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lazybark/cents/bot"
	"github.com/lazybark/cents/config"
	"github.com/spf13/cobra"
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "cents-bot:", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	dir, err := config.Dir()
	if err != nil {
		return "bot.yml"
	}

	return filepath.Join(dir, "bot.yml")
}

func root() *cobra.Command {
	var configPath string

	load := func() (bot.Config, error) {
		cfg, err := bot.LoadConfig(configPath)
		if err != nil {
			return bot.Config{}, fmt.Errorf("%w (start from: cents-bot config example)", err)
		}

		if cfg.Reminders.StateFile == "" {
			cfg.Reminders.StateFile = bot.DefaultStateFile(configPath)
		}

		return cfg, nil
	}

	run := func(*cobra.Command, []string) error {
		cfg, err := load()
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return bot.Run(ctx, cfg, os.Stderr)
	}

	cmd := &cobra.Command{
		Use:           "cents-bot",
		Short:         "cents' Telegram bot: add incomes and expenses, see reports",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          run,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath(), "config file")

	cmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the bot (the default)",
		Args:  cobra.NoArgs,
		RunE:  run,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check the config: the bot's token and the data it uses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}

			report, err := bot.CheckSetup(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), report)
			return nil
		},
	})

	example := &cobra.Command{Use: "config", Short: "Config file help"}
	example.AddCommand(&cobra.Command{
		Use:   "example",
		Short: "Print an example config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprint(cmd.OutOrStdout(), bot.ExampleConfig)
		},
	})
	cmd.AddCommand(example)

	return cmd
}
