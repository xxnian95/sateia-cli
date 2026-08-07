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
CLI token. Headless agents may provide SATEIA_TOKEN or SATEIA_TOKEN_FILE. Run
"sateia environment" for device naming, storage, and precedence details.`,
	}
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
the CLI creates it with private permissions before consuming the device code.`,
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
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN")) != "" {
				return errors.New("SATEIA_TOKEN is set; unset it before storing a different token")
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")) != "" {
				return errors.New("SATEIA_TOKEN_FILE is set; unset it before storing a different token")
			}

			var prepared *credential.PreparedTokenFile
			if tokenFile != "" {
				prepared, err = credential.PrepareTokenFile(tokenFile)
				if err != nil {
					return err
				}
				defer prepared.Abort()
			} else if existing, getErr := app.store.Get(baseURL); getErr == nil && existing != "" {
				return errors.New("a token is already stored for this server; run 'sateia auth logout' first")
			} else if errors.Is(getErr, credential.ErrUnavailable) {
				return credentialStoreUnavailableError("access system credential store", getErr)
			} else if getErr != nil && !errors.Is(getErr, credential.ErrNotFound) {
				return fmt.Errorf("access system credential store: %w", getErr)
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
					return fmt.Errorf("read device name: %w", readErr)
				}
				deviceName = strings.TrimSpace(line)
				if deviceName == "" {
					deviceName = defaultName
				}
			}
			deviceName = strings.TrimSpace(deviceName)
			if len([]rune(deviceName)) > 100 {
				return errors.New("device name must contain at most 100 characters")
			}
			if deviceCode == "" {
				fmt.Fprint(app.errOut, "Device code from the Sateia app: ")
				line, readErr := reader.ReadString('\n')
				if readErr != nil && len(line) == 0 {
					return fmt.Errorf("read device code: %w", readErr)
				}
				deviceCode = line
			}
			deviceCode = strings.ToUpper(strings.TrimSpace(deviceCode))
			if !pairingCodePattern.MatchString(deviceCode) {
				return errors.New("device code must match ABCD-EFGH using the characters shown in the app")
			}

			client, err := api.NewClient(baseURL, "", app.version, nil)
			if err != nil {
				return err
			}
			issued, err := client.ExchangePairingCode(command.Context(), deviceCode, deviceName)
			if err != nil {
				return loginErrorWithGuidance(err)
			}
			credentialSource := "system keyring"
			if prepared != nil {
				if err := prepared.Commit(issued.Token); err != nil {
					return fmt.Errorf("store token in token file: %w; the code was consumed, so secure the partial file, create a new code, and retry", err)
				}
				credentialSource = "token_file"
			} else if err := app.store.Set(baseURL, issued.Token); err != nil {
				if errors.Is(err, credential.ErrUnavailable) {
					return fmt.Errorf("store token in system credential store: %w; the code was consumed, so create a new code and retry with --token-file on headless Linux", err)
				}
				return fmt.Errorf("store token in system credential store: %w; the code was consumed, so create a new code and retry", err)
			}
			stored.BaseURL = baseURL
			stored.TokenID = issued.TokenID
			stored.ExpiresAt = &issued.ExpiresAt
			stored.DeviceName = deviceName
			if err := config.Save(stored); err != nil {
				return fmt.Errorf("credential stored but save non-secret metadata: %w", err)
			}
			fmt.Fprintf(app.out, `Authentication succeeded.
server: %s
device_name: %s
token_id: %s
token_expires_at: %s
credential_source: %s
`, baseURL, deviceName, issued.TokenID, issued.ExpiresAt.Format(time.RFC3339), credentialSource)
			if prepared != nil {
				fmt.Fprintf(app.out, "token_file: %s\nNext: set SATEIA_TOKEN_FILE=%q and run \"sateia auth status\".\n", prepared.Path(), prepared.Path())
			} else {
				fmt.Fprintln(app.out, `Next: run "sateia auth status" to verify the stored credential.`)
			}
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	command.Flags().StringVar(&deviceCode, "device-code", "", "one-time code displayed by the Sateia app")
	command.Flags().StringVar(&deviceName, "device-name", "", "stable name identifying this machine (default: prompt with hostname)")
	command.Flags().StringVar(&tokenFile, "token-file", "", "new file for the token on headless systems (created with mode 0600)")
	return command
}

func (app *application) newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verify the current token",
		Long: `Verify the effective credential with a read-only API request.

Credential precedence is SATEIA_TOKEN, then SATEIA_TOKEN_FILE, then the system
credential store. This command does not reveal the token secret and does not
write nutrition data.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			token, source, err := app.token(baseURL)
			if errors.Is(err, credential.ErrNotFound) {
				return errors.New("not logged in; run 'sateia auth login' or set SATEIA_TOKEN or SATEIA_TOKEN_FILE")
			}
			if errors.Is(err, credential.ErrUnavailable) {
				return credentialStoreUnavailableError("read token from system credential store", err)
			}
			if err != nil {
				return fmt.Errorf("read authentication token: %w", err)
			}
			client, err := api.NewClient(baseURL, token, app.version, nil)
			if err != nil {
				return err
			}
			if err := client.CheckAuthentication(command.Context()); err != nil {
				return authenticationCheckError(err, source)
			}
			fmt.Fprintf(app.out, "Authentication verified.\nserver: %s\ncredential_source: %s\n", baseURL, source)
			if source == "keyring" && stored.ExpiresAt != nil {
				fmt.Fprintf(app.out, "token_expires_at: %s\n", stored.ExpiresAt.Format(time.RFC3339))
			}
			fmt.Fprintln(app.out, `Next: run "sateia record create --help" before writing a record.`)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
}

func (app *application) newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored CLI token",
		Long: `Remove the token from the local operating system credential store.

This does not revoke the token on the server. Revoke the CLI token in the
Sateia app when the credential must become invalid everywhere. Environment and
token-file credentials are managed by their owner and are not deleted.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN")) != "" {
				return errors.New("SATEIA_TOKEN is set in the environment; unset it to log out")
			}
			if strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")) != "" {
				return errors.New("SATEIA_TOKEN_FILE is set; remove the token at its managed source or unset it before using keyring logout")
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
				return err
			}
			fmt.Fprintf(app.out, `Local authentication removed.
server: %s
To invalidate the token on the server, revoke it in Sateia app > Settings > CLI Access.
`, baseURL)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
}

func credentialStoreUnavailableError(action string, err error) error {
	return fmt.Errorf("%s: %w\nNext: on headless Linux, set SATEIA_TOKEN or SATEIA_TOKEN_FILE, or create a new code and run auth login with --token-file <new-path>; otherwise start a Secret Service provider and user D-Bus session", action, err)
}

func loginErrorWithGuidance(err error) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "INVALID_PAIRING_CODE":
			return fmt.Errorf("%w\nNext: create a new code in Sateia app > Settings > CLI Access and retry with the exact same device name", err)
		case "RATE_LIMIT_EXCEEDED":
			return fmt.Errorf("%w\nNext: wait before creating a new code and retrying", err)
		}
	}
	return err
}

func authenticationCheckError(err error, source string) error {
	wrapped := fmt.Errorf("authentication check failed: %w", err)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "UNAUTHENTICATED" {
		return wrapped
	}
	if source == "environment" {
		return fmt.Errorf("%w\nNext: replace or unset SATEIA_TOKEN, then run \"sateia auth status\" again", wrapped)
	}
	return fmt.Errorf("%w\nNext: run \"sateia auth logout\", create a new app code, and run \"sateia auth login\"", wrapped)
}
