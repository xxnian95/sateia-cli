package cli

import (
	"errors"
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
  - One-off commands may use --token. Prefer SATEIA_TOKEN or SATEIA_TOKEN_FILE
    for automation because command arguments may be exposed through shell
    history or process inspection.
  - Precedence: --token, SATEIA_TOKEN, SATEIA_TOKEN_FILE, then the system
    keyring.
  - Run "sateia auth status" to see credential_source without revealing the
    token. An invalid credential produces source-specific replacement steps.

Credential storage:
  - macOS Keychain, Linux Secret Service, or Windows Credential Manager.
  - A Linux server without Secret Service should use a managed environment
    secret, a mounted token file, or "auth login --token-file <new-path>".
  - --token-file reserves a new file with mode 0600 before consuming the code.
    Set SATEIA_TOKEN_FILE to that path for later commands.
  - --token is never stored. SATEIA_TOKEN and SATEIA_TOKEN_FILE remain owned by
    the environment or secret manager that supplied them.
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
  4. Mutate a record only after the user requests that exact write. Preserve
     supplied values, and never infer a record ID or expected version.
  5. Run the selected create, update, or delete subcommand with --help and
     validate every required value and legal flag combination.
  6. Use --json for machine-readable query and mutation output.
  7. After an ambiguous network failure, retry the exact same request with all
     printed identifiers. Update and delete retries must also preserve
     --expected-version. Never change a payload while reusing mutation_id.

Response notices and updates:
  - JSON responses include a top-level _notice list. Inspect each code and
    finish the requested operation before acting on informational notices.
  - UPDATE_AVAILABLE includes the exact go install command for the latest
    stable GitHub tag. NEXT_PAGE explains when pagination should continue.
  - The public GitHub tag check runs after a successful command and is cached
    for 24 hours. Failures never change the command result.
  - Set SATEIA_NO_UPDATE_NOTIFIER=1 to disable the tag check in hermetic runs.

Request correlation:
  - Successful server-backed commands expose the server's X-Request-ID as
    top-level request_id in JSON and as request_id in human-readable output.
  - API errors include request_id when the server supplied one. Report it when
    diagnosing a failure so operators can correlate structured logs and audit
    events. It identifies the request, not a nutrition record.

Exit status is zero on success and non-zero on validation, authentication,
network, or server errors.`

func (app *application) newEnvironmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Explain configuration and safe automation",
		Long:  environmentGuide,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if app.manualTokenSet {
				return errors.New("--token is not used by the environment command\nNext: omit --token to print guidance, or use it with auth status or a record operation")
			}
			fmt.Fprintln(app.out, environmentGuide)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
}
