package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/prow-sh/prow/internal/client"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local prow config and remote prowd health",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfgPath, err := LoadConfig()
			if err != nil {
				return err
			}
			pname := profile
			if pname == "" {
				pname = defaultProfileName
			}
			p, ok := root.Profiles[pname]
			if !ok {
				return fmt.Errorf("profile %q not found in %s", pname, cfgPath)
			}

			fail := false
			check := func(name string, ok bool, detail string) {
				if ok {
					fmt.Fprintf(cmd.OutOrStdout(), "[pass] %s: %s\n", name, detail)
					return
				}
				fail = true
				fmt.Fprintf(cmd.OutOrStdout(), "[fail] %s: %s\n", name, detail)
			}

			_, statErr := os.Stat(cfgPath)
			check("config file", statErr == nil, cfgPath)
			check("server url", strings.TrimSpace(p.URL) != "", strings.TrimSpace(p.URL))
			check("token present", strings.TrimSpace(p.Token) != "", "token is set")

			c := client.New(p.URL, p.Token)
			ctx := cmd.Context()
			if _, err := c.Health(ctx); err != nil {
				check("GET /health", false, err.Error())
			} else {
				check("GET /health", true, "ok")
			}
			if v, err := c.Version(ctx); err != nil {
				check("GET /version", false, err.Error())
			} else {
				check("GET /version", true, fmt.Sprintf("%s (%s)", v.Version, v.Commit))
			}

			if fail {
				return fmt.Errorf("doctor: one or more checks failed")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "All checks passed.")
			return nil
		},
	}
}
