package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login <server-url>",
		Short: "Configure prow to talk to a prowd server (Phase 0A lab stores token in config.yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return errors.New("--token is required")
			}
			serverURL := args[0]

			cfgPath, err := ConfigPath()
			if err != nil {
				return err
			}
			root, _, err := LoadConfig()
			if err != nil {
				return err
			}

			pname := profile
			if pname == "" {
				pname = defaultProfileName
			}

			if root.Profiles == nil {
				root.Profiles = map[string]Profile{}
			}
			root.Profiles[pname] = Profile{URL: serverURL, Token: token}
			root.DefaultProfile = pname

			// TODO(production): store tokens in the OS keychain (macOS Keychain / Windows Credential Manager / libsecret)
			// instead of ~/.prow/config.yaml. Phase 0A lab mode intentionally uses plaintext for speed of iteration.
			if err := SaveConfig(cfgPath, root); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Saved profile %q (%s)\n", pname, serverURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "bearer token for lab / non-interactive login")
	return cmd
}
