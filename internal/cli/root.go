package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var profile string

// Execute runs the prow CLI.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "prow",
		Short:         "Prow analyst CLI (thin client for prowd)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&profile, "profile", defaultProfileName, "config profile name")

	root.AddCommand(newLoginCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newAlertsCmd())
	return root
}
