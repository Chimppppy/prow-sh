package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/prow-sh/prow/internal/api"
	"github.com/prow-sh/prow/internal/auth"
	"github.com/prow-sh/prow/internal/store"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "prowd",
		Short:         "Prow server (API + storage for Phase 0A lab mode)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newServeCmd())
	return root
}

func newInitCmd() *cobra.Command {
	var lab bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize prowd configuration and local state",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !lab {
				return errors.New("Phase 0A only supports: prowd init --lab")
			}
			paths, err := store.InitLab()
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Welcome to Prow (lab mode).")
			fmt.Fprintln(cmd.OutOrStdout(), "This runs locally with SQLite and no external dependencies.")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Created config at %s\n", paths.ConfigPath)
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Initialized SQLite database at %s\n", paths.DBPath)
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated local admin token (saved to %s)\n", paths.TokenPath)
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
			fmt.Fprintf(cmd.OutOrStdout(), "  1) Start the server:\n     prowd serve --config %s\n", paths.ConfigPath)
			fmt.Fprintln(cmd.OutOrStdout(), "  2) Connect the CLI (pick one):")
			if runtime.GOOS == "windows" {
				fmt.Fprintln(cmd.OutOrStdout(), `     prow login http://localhost:7777 --token (Get-Content $env:USERPROFILE\.prow\prowd.token)`)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "     prow login http://localhost:7777 --token $(cat ~/.prow/prowd.token)")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  3) Sanity checks:")
			fmt.Fprintln(cmd.OutOrStdout(), "     prow doctor")
			fmt.Fprintln(cmd.OutOrStdout(), "     prow alerts")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lab, "lab", false, "initialize local lab mode (SQLite + local token)")
	return cmd
}

func newServeCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the prowd HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				p, err := defaultProwdConfigPath()
				if err != nil {
					return err
				}
				configPath = p
			}

			v := viper.New()
			v.SetConfigFile(configPath)
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("read config %q: %w", configPath, err)
			}

			bind := v.GetString("server.bind")
			if bind == "" {
				bind = "127.0.0.1:7777"
			}
			dsn := v.GetString("storage.sqlite_dsn")
			if dsn == "" {
				return errors.New("config missing storage.sqlite_dsn")
			}
			tokenPath := v.GetString("auth.token_file")
			if tokenPath == "" {
				return errors.New("config missing auth.token_file")
			}
			token, err := auth.ReadLabTokenFile(tokenPath)
			if err != nil {
				return fmt.Errorf("read lab token: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(cmd.OutOrStdout(), "prowd listening on http://%s (config=%s)\n", bind, configPath)
			return api.Run(ctx, api.ServerConfig{
				BindAddr:   bind,
				LabToken:   token,
				SQLiteDSN: dsn,
			})
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "path to prowd YAML config (default: ~/.prow/prowd-config.yaml)")
	return cmd
}

func defaultProwdConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".prow", "prowd-config.yaml"), nil
}
