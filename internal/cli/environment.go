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
  - Before exchanging a code, run hostname and choose a stable, recognizable
    name for the current machine, such as "agent-host-01 (Sateia CLI)".
  - The CLI supplies this name during exchange as token metadata. It does not
    need to match a legacy app label. Never reuse one generic name across
    devices.
  - Headless: set SATEIA_TOKEN or SATEIA_TOKEN_FILE. Never pass a token as a
    command-line argument.
  - Precedence: SATEIA_TOKEN, SATEIA_TOKEN_FILE, then the system keyring.

Credential storage:
  - macOS Keychain, Linux Secret Service, or Windows Credential Manager.
  - A Linux server without Secret Service should use a managed environment
    secret, a mounted token file, or "auth login --token-file <new-path>".
  - --token-file reserves a new file with mode 0600 before consuming the code.
    Set SATEIA_TOKEN_FILE to that path for later commands.
  - config.json contains only server and token metadata, never the token secret.
  - "sateia auth logout" removes only a keyring credential. Environment and
    token-file credentials remain owned by their secret manager.
  - Revoke the token in the Sateia app when it must become invalid on the server.

Safe agent workflow:
  1. Run hostname and use a stable name that identifies the current machine
     whenever requesting device-code authentication.
  2. Run "sateia record list --help" before a read. Use explicit RFC 3339
     bounds and keep every filter unchanged when continuing with --cursor.
  3. Run "sateia auth status" before a write.
  4. Run "sateia record create --help" and validate all required values.
  5. Use --json for machine-readable query and mutation output.
  6. After an ambiguous network failure, retry the exact same request with
     both the printed --record-id and --mutation-id. Never generate new IDs
     for that retry.

Response notices and updates:
  - JSON responses include a top-level _notice list. Inspect each code and
    finish the requested operation before acting on informational notices.
  - UPDATE_AVAILABLE includes the exact go install command for the latest
    stable GitHub tag. NEXT_PAGE explains when pagination should continue.
  - The public GitHub tag check runs after a successful command and is cached
    for 24 hours. Failures never change the command result.
  - Set SATEIA_NO_UPDATE_NOTIFIER=1 to disable the tag check in hermetic runs.

Exit status is zero on success and non-zero on validation, authentication,
network, or server errors.`

func (app *application) newEnvironmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Explain configuration and safe automation",
		Long:  environmentGuide,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			fmt.Fprintln(app.out, environmentGuide)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
}
