package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const environmentGuide = `Sateia CLI configuration, credentials, and safe automation.

Server selection, from highest to lowest precedence:
  1. --server
  2. SATEIA_SERVER
  3. The server saved by "sateia auth login"
  4. https://xxnian.site/sateia-server

Authentication:
  - Interactive: run "sateia auth login" with a code created in
    Sateia app > Settings > CLI Access.
  - The CLI device name must exactly match the name used to create the code.
  - Headless: set SATEIA_TOKEN. Never pass a token as a command-line flag.
  - SATEIA_TOKEN overrides a token in the system credential store.

Credential storage:
  - macOS Keychain, Linux Secret Service, or Windows Credential Manager.
  - config.json contains only server and token metadata, never the token secret.
  - "sateia auth logout" removes the local credential only. Revoke the token
    in the Sateia app when it must also become invalid on the server.

Safe agent workflow:
  1. Run "sateia record list --help" before a read. Use explicit RFC 3339
     bounds and keep every filter unchanged when continuing with --cursor.
  2. Run "sateia auth status" before a write.
  3. Run "sateia record create --help" and validate all required values.
  4. Use --json for machine-readable query and mutation output.
  5. After an ambiguous network failure, retry the exact same request with
     both the printed --record-id and --mutation-id. Never generate new IDs
     for that retry.

Exit status is zero on success and non-zero on validation, authentication,
network, or server errors.`

func (app *application) newEnvironmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Explain configuration and safe automation",
		Long:  environmentGuide,
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintln(app.out, environmentGuide)
			return nil
		},
	}
}
