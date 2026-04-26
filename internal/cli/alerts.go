package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/prow-sh/prow/internal/client"
)

func newAlertsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "List normalized events from prowd (PCS events)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _, err := LoadConfig()
			if err != nil {
				return err
			}
			pname := profile
			if pname == "" {
				pname = defaultProfileName
			}
			p, ok := root.Profiles[pname]
			if !ok {
				return fmt.Errorf("profile %q not found", pname)
			}

			useJSON := jsonOut || strings.EqualFold(root.Output.Format, "json")
			c := client.New(p.URL, p.Token)
			events, err := c.Events(cmd.Context())
			if err != nil {
				return err
			}

			if useJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(events)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "OCCURRED_AT\tSEVERITY\tCATEGORY\tTITLE")
			for _, e := range events {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					e.OccurredAt.UTC().Format(time.RFC3339),
					string(e.Severity),
					e.Category,
					strings.ReplaceAll(e.Title, "\t", " "),
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print events as JSON (also implied by output.format=json)")
	return cmd
}
