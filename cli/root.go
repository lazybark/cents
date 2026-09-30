// Package cli wires the cents entrypoint: it parses flags and hands off to
// one of the run sequences (desktop window or terminal UI).
package cli

import (
	"fmt"
	"strings"

	"github.com/lazybark/cents/config"
	"github.com/spf13/cobra"
)

const (
	modeDesktop = "desktop"
	modeTUI     = "tui"
)

// modeAliases maps every accepted --mode value to its canonical mode.
var modeAliases = map[string]string{
	"desktop": modeDesktop,
	"d":       modeDesktop,
	"tui":     modeTUI,
	"t":       modeTUI,
}

func NewRootCommand() *cobra.Command {
	var (
		mode       string
		configPath string
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:           "cents",
		Short:         "Personal finance tracker",
		Long:          "cents tracks accounts, cashflow, debts, goals, taxes and more.\nRuns as a desktop app by default; use --mode=t for the terminal UI.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolved, ok := modeAliases[strings.ToLower(strings.TrimSpace(mode))]
			if !ok {
				return fmt.Errorf("unknown mode %q: use desktop (d) or tui (t)", mode)
			}

			if configPath == "" {
				defaultPath, err := config.DefaultPath()
				if err != nil {
					return err
				}

				configPath = defaultPath
			}

			l := launcher{configPath: configPath, dbOverride: dbPath}

			switch resolved {
			case modeTUI:
				return runTUI(l)
			default:
				return runDesktop(l)
			}
		},
	}

	cmd.Flags().StringVarP(&mode, "mode", "m", modeDesktop, "interface to run: desktop (d) or tui (t)")
	cmd.Flags().StringVar(&configPath, "config", "", "config file path (default is cents/config.yml in the OS user config directory)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database file to use for this run only, created if missing; the config file is not read or changed")

	return cmd
}
