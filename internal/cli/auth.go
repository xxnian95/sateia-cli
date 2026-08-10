package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/config"
	"github.com/xxnian95/sateia-cli/internal/credential"
)

var pairingCodePattern = regexp.MustCompile(`^[A-HJ-KM-NP-Z2-9]{4}-[A-HJ-KM-NP-Z2-9]{4}$`)

func (app *application) newAuthCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long: `Manage the CLI credential used for Sateia API requests.

Interactive login exchanges an app-issued, five-minute code for an independent
CLI token. One-off commands may provide --token; headless agents should prefer
SATEIA_TOKEN or SATEIA_TOKEN_FILE. Run "sateia environment" for device naming,
credential precedence, storage ownership, and recovery instructions.`,
	}
	setCommandRisk(command, riskNone)
	command.AddCommand(app.newLoginCommand(), app.newStatusCommand(), app.newLogoutCommand())
	return command
}

func (app *application) newLoginCommand() *cobra.Command {
	var deviceCode string
	var deviceName string
	var tokenFile string
	command := &cobra.Command{
		Use:   "login",
		Short: "Exchange an app-issued device code for a CLI token",
		Long: `Exchange a five-minute, single-use code for a CLI token.

Create the code in Sateia app > Settings > CLI Access. The CLI supplies the
device name during exchange and stores it as token metadata; it does not need
to match a legacy name shown while creating the code. Without flags, the
command prompts for both values. The token is never printed.

Before exchanging a code, AI agents must identify the current machine: run
hostname, choose a stable, recognizable name such as "agent-host-01 (Sateia
CLI)", and pass it to --device-name with the code. Do not reuse a generic name
across multiple devices.

By default the token is stored in the operating system credential store. On a
headless Linux machine without Secret Service, use --token-file with a new path;
the CLI creates it with private permissions before consuming the device code.

Field guidance:
  --device-code is the current eight-character app code in ABCD-EFGH form.
  --device-name is a stable, recognizable name for this machine, up to 100
  characters. Run hostname before choosing it.
  --token-file is a new, non-existing path whose parent directory already
  exists. Omit it to use the operating system credential store.

AI agents: immediately before every login attempt, run
"sateia auth login --help" again. Installed CLI updates may change these
instructions.`,
		Example: `  # Interactive login
  sateia auth login

  # Non-interactive code exchange; the code is short-lived, not the token
  sateia auth login --device-code ABCD-EFGH --device-name "Pengnian Mac"

  # Headless Linux after choosing this machine's stable device name
  sateia auth login --device-code ABCD-EFGH \
    --device-name "agent-host-01 (Sateia CLI)" \
    --token-file /secure/path/sateia-token`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if app.manualTokenSet {
				return errors.New("--token cannot be used with auth login because login creates and stores a new token\nNext: omit --token, then retry auth login with an app-issued device code")
			}
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN")) != "" {
				return errors.New("SATEIA_TOKEN already selects an environment-managed token\nNext: unset SATEIA_TOKEN before storing a different token with auth login")
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")) != "" {
				return errors.New("SATEIA_TOKEN_FILE already selects a file-managed token\nNext: unset SATEIA_TOKEN_FILE before storing a different token with auth login")
			}

			var prepared *credential.PreparedTokenFile
			if tokenFile != "" {
				prepared, err = credential.PrepareTokenFile(tokenFile)
				if err != nil {
					return fmt.Errorf("prepare token file: %w\nNext: choose a new path whose parent directory exists; auth login never overwrites a token file", err)
				}
				defer prepared.Abort()
			} else if existing, getErr := app.store.Get(baseURL); getErr == nil && existing != "" {
				return errors.New("a keyring token is already stored for this server\nNext: run \"sateia auth status\" to verify it, or \"sateia auth logout\" before storing a replacement")
			} else if errors.Is(getErr, credential.ErrUnavailable) {
				return credentialStoreUnavailableError("access system credential store", getErr)
			} else if getErr != nil && !errors.Is(getErr, credential.ErrNotFound) {
				return fmt.Errorf("access system credential store: %w\nNext: resolve the keyring error, or retry with --token-file <new-path> on a headless system", getErr)
			}

			reader := bufio.NewReader(app.in)
			if deviceName == "" {
				defaultName, hostnameErr := os.Hostname()
				if hostnameErr != nil || strings.TrimSpace(defaultName) == "" {
					defaultName = "Sateia CLI"
				}
				fmt.Fprintf(app.errOut, "Stable, recognizable device name for this machine [%s]: ", defaultName)
				line, readErr := reader.ReadString('\n')
				if readErr != nil && len(line) == 0 {
					return fmt.Errorf("read device name: %w\nNext: retry with --device-name on a non-interactive terminal", readErr)
				}
				deviceName = strings.TrimSpace(line)
				if deviceName == "" {
					deviceName = defaultName
				}
			}
			deviceName = strings.TrimSpace(deviceName)
			if deviceName == "" {
				return errors.New("device name must not be empty\nNext: pass --device-name with a stable name that identifies this machine, or omit the flag to use the interactive prompt")
			}
			if len([]rune(deviceName)) > 100 {
				return errors.New("device name must contain at most 100 characters\nNext: shorten --device-name while keeping it specific to this machine")
			}
			if deviceCode == "" {
				fmt.Fprint(app.errOut, "Device code from the Sateia app: ")
				line, readErr := reader.ReadString('\n')
				if readErr != nil && len(line) == 0 {
					return fmt.Errorf("read device code: %w\nNext: retry with both --device-code and --device-name on a non-interactive terminal", readErr)
				}
				deviceCode = line
			}
			deviceCode = strings.ToUpper(strings.TrimSpace(deviceCode))
			if !pairingCodePattern.MatchString(deviceCode) {
				return errors.New("device code must match ABCD-EFGH using the characters shown in the app\nNext: copy the current code from Sateia app > Settings > CLI Access without adding spaces")
			}

			client, err := api.NewClient(baseURL, "", app.version, nil)
			if err != nil {
				return err
			}
			issued, metadata, err := client.ExchangePairingCode(command.Context(), deviceCode, deviceName)
			if err != nil {
				return loginErrorWithGuidance(err)
			}
			credentialSource := "system keyring"
			if prepared != nil {
				if err := prepared.Commit(issued.Token); err != nil {
					return fmt.Errorf("store token in token file: %w\nThe device code was consumed, and the uncommitted token file will be removed.\nNext: resolve the file error, create a new app code, and retry with a new path", err)
				}
				credentialSource = "token_file"
			} else if err := app.store.Set(baseURL, issued.Token); err != nil {
				if errors.Is(err, credential.ErrUnavailable) {
					return fmt.Errorf("store token in system credential store: %w\nThe device code was consumed, but no credential was stored.\nNext: on headless Linux, create a new app code and retry with --token-file <new-path>", err)
				}
				return fmt.Errorf("store token in system credential store: %w\nThe device code was consumed, but no credential was stored.\nNext: resolve the keyring error, create a new app code, and retry", err)
			}
			stored.BaseURL = baseURL
			stored.TokenID = issued.TokenID
			stored.ExpiresAt = &issued.ExpiresAt
			stored.DeviceName = deviceName
			if err := config.Save(stored); err != nil {
				if prepared != nil {
					return fmt.Errorf("token stored in %s but save non-secret metadata: %w\nNext: set SATEIA_TOKEN_FILE=%q and run \"sateia --server %s auth status\"; do not exchange the code again", prepared.Path(), err, prepared.Path(), baseURL)
				}
				return fmt.Errorf("keyring token stored but save non-secret metadata: %w\nNext: run \"sateia --server %s auth status\"; repair the configuration location before changing credentials", err, baseURL)
			}
			if app.jsonOutput {
				output := struct {
					Server           string `json:"server"`
					DeviceName       string `json:"device_name"`
					TokenID          string `json:"token_id"`
					TokenExpiresAt   string `json:"token_expires_at"`
					CredentialSource string `json:"credential_source"`
					TokenFile        string `json:"token_file,omitempty"`
					RequestID        string `json:"request_id"`
				}{
					Server: baseURL, DeviceName: deviceName, TokenID: issued.TokenID,
					TokenExpiresAt: issued.ExpiresAt.Format(time.RFC3339), CredentialSource: credentialSource,
					RequestID: metadata.RequestID,
				}
				if prepared != nil {
					output.TokenFile = prepared.Path()
				}
				return app.writeJSON(command.Context(), output)
			}
			fmt.Fprintf(app.out, `Authentication succeeded.
server: %s
device_name: %s
token_id: %s
token_expires_at: %s
credential_source: %s
`, baseURL, deviceName, issued.TokenID, issued.ExpiresAt.Format(time.RFC3339), credentialSource)
			if prepared != nil {
				fmt.Fprintf(app.out, "token_file: %s\n", prepared.Path())
			}
			if metadata.RequestID != "" {
				fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
			}
			if prepared != nil {
				fmt.Fprintf(app.out, "Next: set SATEIA_TOKEN_FILE=%q and run \"sateia auth status\".\n", prepared.Path())
			} else {
				fmt.Fprintln(app.out, `Next: run "sateia auth status" to verify the stored credential.`)
			}
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskLocalWrite)
	command.Flags().StringVar(&deviceCode, "device-code", "", "current five-minute app code in ABCD-EFGH form; omit for an interactive prompt")
	command.Flags().StringVar(&deviceName, "device-name", "", "stable recognizable name for this machine, up to 100 characters; omit for a hostname-based prompt")
	command.Flags().StringVar(&tokenFile, "token-file", "", "new non-existing token path with an existing parent; omit to use the keyring")
	return command
}

func (app *application) newStatusCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "status",
		Short: "Verify the current token",
		Long: `Verify the effective credential with a read-only API request.

Credential precedence is --token, SATEIA_TOKEN, SATEIA_TOKEN_FILE, then the
system credential store. This command does not reveal the token secret and
does not write nutrition data.

Use --server only for the intended HTTPS API base URL. Use --token only with a
non-empty one-off bearer token; prefer managed environment or file credentials
for automation.

AI agents: immediately before every status check, run
"sateia auth status --help" again. Installed CLI updates may change these
instructions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			token, source, err := app.token(baseURL)
			if err != nil {
				return authenticationTokenError(err, source)
			}
			client, err := api.NewClient(baseURL, token, app.version, nil)
			if err != nil {
				return err
			}
			metadata, err := client.CheckAuthentication(command.Context())
			if err != nil {
				return authenticationCheckError(err, source)
			}
			if app.jsonOutput {
				output := struct {
					Server           string `json:"server"`
					CredentialSource string `json:"credential_source"`
					TokenExpiresAt   string `json:"token_expires_at,omitempty"`
					RequestID        string `json:"request_id"`
				}{Server: baseURL, CredentialSource: source, RequestID: metadata.RequestID}
				if source == "keyring" && stored.ExpiresAt != nil {
					output.TokenExpiresAt = stored.ExpiresAt.Format(time.RFC3339)
				}
				return app.writeJSON(command.Context(), output)
			}
			fmt.Fprintf(app.out, "Authentication verified.\nserver: %s\ncredential_source: %s\n", baseURL, source)
			if source == "keyring" && stored.ExpiresAt != nil {
				fmt.Fprintf(app.out, "token_expires_at: %s\n", stored.ExpiresAt.Format(time.RFC3339))
			}
			if metadata.RequestID != "" {
				fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
			}
			fmt.Fprintln(app.out, `Next: run "sateia record list --help" for a read, or inspect the selected create, update, or delete subcommand before a write.`)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskReadOnly)
	return command
}

func (app *application) newLogoutCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored CLI token",
		Long: `Remove the token from the local operating system credential store.

This does not revoke the token on the server. Revoke the CLI token in the
Sateia app when the credential must become invalid everywhere. Environment and
token-file credentials are managed by their owner and are not deleted. Do not
pass --token: logout acts only on the keyring credential for the selected
server.

AI agents: immediately before every logout, run
"sateia auth logout --help" again. Installed CLI updates may change these
instructions.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if app.manualTokenSet {
				return errors.New("--token is used only for the current invocation and is not stored by the CLI\nNext: omit --token to remove a keyring credential, or revoke the token in Sateia app > Settings > CLI Access")
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN")) != "" {
				return errors.New("SATEIA_TOKEN is managed by the environment and cannot be removed by auth logout\nNext: unset or replace SATEIA_TOKEN at its source; revoke it in the Sateia app if it must stop working")
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")) != "" {
				return errors.New("SATEIA_TOKEN_FILE points to an owner-managed token file that auth logout will not delete\nNext: remove or replace that secret at its source, or unset SATEIA_TOKEN_FILE before removing a keyring credential")
			}
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			if err := app.store.Delete(baseURL); errors.Is(err, credential.ErrUnavailable) {
				return credentialStoreUnavailableError("delete token from system credential store", err)
			} else if err != nil && !errors.Is(err, credential.ErrNotFound) {
				return fmt.Errorf("delete token from system credential store: %w", err)
			}
			stored.TokenID = ""
			stored.ExpiresAt = nil
			stored.DeviceName = ""
			if err := config.Save(stored); err != nil {
				return fmt.Errorf("keyring credential removed but clear non-secret metadata: %w\nNext: repair the configuration location; the local keyring token is already removed, but the server token is not revoked", err)
			}
			if app.jsonOutput {
				return app.writeJSON(command.Context(), struct {
					Server           string `json:"server"`
					CredentialSource string `json:"credential_source"`
					LocalRemoved     bool   `json:"local_removed"`
					ServerRevoked    bool   `json:"server_revoked"`
				}{Server: baseURL, CredentialSource: "keyring", LocalRemoved: true, ServerRevoked: false})
			}
			fmt.Fprintf(app.out, `Local authentication removed.
server: %s
To invalidate the token on the server, revoke it in Sateia app > Settings > CLI Access.
`, baseURL)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskLocalDelete)
	return command
}

func credentialStoreUnavailableError(action string, err error) error {
	return fmt.Errorf("%s: %w\nNext: supply --token for a one-off command; on headless Linux, set SATEIA_TOKEN or SATEIA_TOKEN_FILE, or create a new code and run auth login with --token-file <new-path>; otherwise start a Secret Service provider and user D-Bus session", action, err)
}

func authenticationTokenError(err error, source string) error {
	if errors.Is(err, credential.ErrNotFound) {
		return errors.New("no authentication credential was found\nNext: run \"sateia auth login\", supply --token for this invocation, or set SATEIA_TOKEN or SATEIA_TOKEN_FILE")
	}
	if errors.Is(err, credential.ErrUnavailable) {
		return credentialStoreUnavailableError("read token from system credential store", err)
	}
	if source == "token_file" {
		return fmt.Errorf("authentication token file is unusable: %w\nNext: fix the file selected by SATEIA_TOKEN_FILE, or unset SATEIA_TOKEN_FILE and choose another credential source", err)
	}
	return fmt.Errorf("read authentication token: %w\nNext: resolve the credential-store error, then run \"sateia auth status\" again", err)
}

func loginErrorWithGuidance(err error) error {
	wrapped := fmt.Errorf("authentication exchange failed: %w", err)
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "INVALID_PAIRING_CODE":
			return fmt.Errorf("%w\nNext: create a new code in Sateia app > Settings > CLI Access and retry with the same device name", wrapped)
		case "RATE_LIMIT_EXCEEDED":
			return fmt.Errorf("%w\nNext: wait for the rate limit to clear; if the five-minute code expires, create a new code and retry with the same device name", wrapped)
		}
	}
	return fmt.Errorf("%w\nNo credential was stored. If the exchange outcome is unknown, create a new app code and retry with the same device name; do not assume authentication succeeded", wrapped)
}

func authenticationCheckError(err error, source string) error {
	wrapped := fmt.Errorf("authentication check failed: %w", err)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "UNAUTHENTICATED" {
		return fmt.Errorf("%w\nNext: check the server URL and connectivity, then retry \"sateia auth status\"", wrapped)
	}
	switch source {
	case "argument":
		return fmt.Errorf("%w\nNext: supply a valid --token value, then run \"sateia auth status\" again", wrapped)
	case "environment":
		return fmt.Errorf("%w\nNext: replace or unset SATEIA_TOKEN, then run \"sateia auth status\" again", wrapped)
	case "token_file":
		return fmt.Errorf("%w\nNext: replace the secret in SATEIA_TOKEN_FILE or unset it and choose another credential source, then run \"sateia auth status\" again", wrapped)
	}
	return fmt.Errorf("%w\nNext: run \"sateia auth logout\", create a new app code, and run \"sateia auth login\"", wrapped)
}
