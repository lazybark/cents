// Command cents-server serves a cents database over a JSON REST API (see
// package server).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lazybark/cents/config"
	"github.com/lazybark/cents/server"
	"github.com/spf13/cobra"
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "cents-server:", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	dir, err := config.Dir()
	if err != nil {
		return "server.yml"
	}

	return filepath.Join(dir, "server.yml")
}

func root() *cobra.Command {
	var configPath string

	serve := func(*cobra.Command, []string) error {
		cfg, err := server.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("%w (start from: cents-server config example)", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return server.Run(ctx, cfg, os.Stderr)
	}

	cmd := &cobra.Command{
		Use:           "cents-server",
		Short:         "Serve a cents database over a JSON REST API",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          serve,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath(), "config file")

	cmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the server (the default)",
		Args:  cobra.NoArgs,
		RunE:  serve,
	})

	var (
		name     string
		readOnly bool
		admin    bool
	)
	token := &cobra.Command{Use: "token", Short: "Manage client tokens"}
	newToken := &cobra.Command{
		Use:   "new",
		Short: "Make a token for a client",
		Long:  "Makes a token for a client. The token is shown once: give it to the client. Add the lines printed for the config to its tokens.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("give the client a name: --name desktop, say")
			}

			value, hash, err := server.NewToken()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Token for %s (shown once; give it to the client):\n\n  %s\n\n", name, value)
			fmt.Fprintf(out, "Add to tokens in %s:\n\n  - name: %s\n    sha256: %s\n", configPath, name, hash)
			if readOnly {
				fmt.Fprintln(out, "    read_only: true")
			}
			if admin {
				fmt.Fprintln(out, "    admin: true")
			}
			return nil
		},
	}
	newToken.Flags().StringVar(&name, "name", "", "the client's name, shown in logs")
	newToken.Flags().BoolVar(&readOnly, "read-only", false, "the client can only read")
	newToken.Flags().BoolVar(&admin, "admin", false, "allow administration access, including diagnostic logs")
	token.AddCommand(newToken)
	cmd.AddCommand(token)

	example := &cobra.Command{Use: "config", Short: "Config file help"}
	example.AddCommand(&cobra.Command{
		Use:   "example",
		Short: "Print an example config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprint(cmd.OutOrStdout(), server.ExampleConfig)
		},
	})
	cmd.AddCommand(example)

	return cmd
}
