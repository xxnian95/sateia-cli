package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/config"
	"github.com/xxnian95/sateia-cli/internal/credential"
)

var pairingCodePattern = regexp.MustCompile(`^[A-HJ-KM-NP-Z2-9]{4}-[A-HJ-KM-NP-Z2-9]{4}$`)

func (app *application) newAuthCommand() *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Manage authentication"}
	command.AddCommand(app.newLoginCommand(), app.newStatusCommand(), app.newLogoutCommand())
	return command
}

func (app *application) newLoginCommand() *cobra.Command {
	var deviceCode string
	var deviceName string
	command := &cobra.Command{
		Use:   "login",
		Short: "Exchange an app-issued device code for a CLI token",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			if os.Getenv("SATEIA_TOKEN") != "" {
				return errors.New("SATEIA_TOKEN is set; unset it before storing a different token")
			}
			if existing, getErr := app.store.Get(baseURL); getErr == nil && existing != "" {
				return errors.New("a token is already stored for this server; run 'sateia auth logout' first")
			} else if getErr != nil && !errors.Is(getErr, credential.ErrNotFound) {
				return fmt.Errorf("access system credential store: %w", getErr)
			}

			reader := bufio.NewReader(app.in)
			if deviceName == "" {
				defaultName, hostnameErr := os.Hostname()
				if hostnameErr != nil || strings.TrimSpace(defaultName) == "" {
					defaultName = "Sateia CLI"
				}
				fmt.Fprintf(app.errOut, "Device name used in the Sateia app [%s]: ", defaultName)
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
				return err
			}
			if err := app.store.Set(baseURL, issued.Token); err != nil {
				return fmt.Errorf("store token in system credential store: %w; the code was consumed, so create a new code and retry", err)
			}
			stored.BaseURL = baseURL
			stored.TokenID = issued.TokenID
			stored.ExpiresAt = &issued.ExpiresAt
			stored.DeviceName = deviceName
			if err := config.Save(stored); err != nil {
				_ = app.store.Delete(baseURL)
				return err
			}
			fmt.Fprintf(app.out, "Logged in to %s as %s. Token expires %s.\n", baseURL, deviceName, issued.ExpiresAt.Format("2006-01-02 15:04 MST"))
			return nil
		},
	}
	command.Flags().StringVar(&deviceCode, "device-code", "", "one-time code displayed by the Sateia app")
	command.Flags().StringVar(&deviceName, "device-name", "", "name shown in the Sateia app (default: hostname)")
	return command
}

func (app *application) newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verify the current token",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			token, source, err := app.token(baseURL)
			if errors.Is(err, credential.ErrNotFound) {
				return errors.New("not logged in; run 'sateia auth login' or set SATEIA_TOKEN")
			}
			if err != nil {
				return fmt.Errorf("read token from system credential store: %w", err)
			}
			client, err := api.NewClient(baseURL, token, app.version, nil)
			if err != nil {
				return err
			}
			if err := client.CheckAuthentication(command.Context()); err != nil {
				return fmt.Errorf("authentication check failed: %w", err)
			}
			fmt.Fprintf(app.out, "Authenticated to %s using %s credentials", baseURL, source)
			if source == "keyring" && stored.ExpiresAt != nil {
				fmt.Fprintf(app.out, "; token expires %s", stored.ExpiresAt.Format("2006-01-02 15:04 MST"))
			}
			fmt.Fprintln(app.out, ".")
			return nil
		},
	}
}

func (app *application) newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored CLI token",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if os.Getenv("SATEIA_TOKEN") != "" {
				return errors.New("SATEIA_TOKEN is set in the environment; unset it to log out")
			}
			baseURL, stored, err := app.baseURL()
			if err != nil {
				return err
			}
			if err := app.store.Delete(baseURL); err != nil && !errors.Is(err, credential.ErrNotFound) {
				return fmt.Errorf("delete token from system credential store: %w", err)
			}
			stored.TokenID = ""
			stored.ExpiresAt = nil
			stored.DeviceName = ""
			if err := config.Save(stored); err != nil {
				return err
			}
			fmt.Fprintf(app.out, "Logged out from %s.\n", baseURL)
			return nil
		},
	}
}
