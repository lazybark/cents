// Package cli wires the cents entrypoint: it parses flags and runs the
// desktop app.
package cli

import (
	"github.com/lazybark/cents/config"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	var (
		configPath string
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:           "cents",
		Short:         "Personal finance tracker",
		Long:          "cents tracks accounts, cashflow, debts, goals, taxes and more.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if configPath == "" {
				defaultPath, err := config.DefaultPath()
				if err != nil {
					return err
				}

				configPath = defaultPath
			}

			return runDesktop(launcher{configPath: configPath, dbOverride: dbPath})
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "config file path (default is cents/config.yml in the OS user config directory)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database file to use for this run only, created if missing; the config file is not read or changed")

	return cmd
}
