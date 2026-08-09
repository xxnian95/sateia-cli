package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/config"
	"github.com/xxnian95/sateia-cli/internal/credential"
	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

const defaultServer = "https://xxnian.site/sateia-server"

const rootDescription = `Sateia reads and mutates server-side nutrition records that synchronize to the
Sateia app.

Quick start:
  1. In Sateia app > Settings > CLI Access, create a one-time code.
  2. Identify this machine with hostname and choose a stable device name.
  3. Run "sateia auth login" with the chosen device name and code.
  4. Run "sateia auth status" to verify the credential.
  5. Run "sateia record list --help" for a read. Before a write, inspect
     "sateia record create --help", "sateia record update --help", or
     "sateia record delete --help".

If you already have a token, use --token for one invocation or prefer
SATEIA_TOKEN or SATEIA_TOKEN_FILE for automation. Command-line tokens may be
visible in shell history and process listings. On headless Linux, device-code
login can store a new token in a private file with --token-file.

Run "sateia environment" for credential precedence, secret-storage behavior,
failure recovery, and AI-agent guidance. Use --json for machine-readable
responses; inspect the top-level _notice list for pagination and update hints.`

type application struct {
	version        string
	in             io.Reader
	out            io.Writer
	errOut         io.Writer
	server         string
	manualToken    string
	manualTokenSet bool
	store          credential.Store
	updateChecker  updateChecker
}

func New(version string, in io.Reader, out, errOut io.Writer) *cobra.Command {
	return newWithDependencies(version, in, out, errOut, credential.KeyringStore{}, updatecheck.New())
}

func newWithStore(version string, in io.Reader, out, errOut io.Writer, store credential.Store) *cobra.Command {
	return newWithDependencies(version, in, out, errOut, store, updatecheck.New())
}

func newWithDependencies(version string, in io.Reader, out, errOut io.Writer, store credential.Store, checker updateChecker) *cobra.Command {
	app := &application{
		version:       version,
		in:            in,
		out:           out,
		errOut:        errOut,
		store:         store,
		updateChecker: checker,
	}
	root := &cobra.Command{
		Use:   "sateia",
		Short: "Query and write nutrition records with Sateia",
		Long:  rootDescription,
		Example: `  # Interactive authentication
  sateia auth login
  sateia auth status

  # Verify an existing token without storing it
  sateia --token "$TOKEN" auth status

  # Inspect each operation before running it
  sateia record list --help
  sateia record create --help
  sateia record update --help
  sateia record delete --help`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		app.manualTokenSet = root.PersistentFlags().Changed("token")
		if app.manualTokenSet && strings.TrimSpace(app.manualToken) == "" {
			return errors.New("--token must not be empty\nNext: supply a non-empty token, or omit --token and use SATEIA_TOKEN, SATEIA_TOKEN_FILE, or the system keyring")
		}
		return nil
	}
	root.PersistentFlags().StringVar(&app.server, "server", "", "Sateia API base URL (or SATEIA_SERVER)")
	root.PersistentFlags().StringVar(&app.manualToken, "token", "", "token for this invocation (highest precedence; may be exposed by the shell or process list)")
	root.AddCommand(app.newAuthCommand(), app.newRecordCommand(), app.newEnvironmentCommand())
	return root
}

func (app *application) baseURL() (string, config.Config, error) {
	stored, err := config.Load()
	if err != nil {
		return "", config.Config{}, fmt.Errorf("load CLI configuration: %w\nNext: fix the configuration directory, or set SATEIA_CONFIG_DIR to a readable location", err)
	}
	raw := app.server
	if raw == "" {
		raw = os.Getenv("SATEIA_SERVER")
	}
	if raw == "" {
		raw = stored.BaseURL
	}
	if raw == "" {
		raw = defaultServer
	}
	normalized, err := api.NormalizeBaseURL(raw)
	if err != nil {
		return "", stored, fmt.Errorf("invalid server configuration: %w\nNext: pass --server with a valid HTTPS base URL, or fix or unset SATEIA_SERVER", err)
	}
	return normalized, stored, nil
}

func (app *application) token(baseURL string) (string, string, error) {
	// An explicit command argument overrides every managed credential source for this invocation.
	if app.manualTokenSet {
		return strings.TrimSpace(app.manualToken), "argument", nil
	}
	if value := strings.TrimSpace(os.Getenv("SATEIA_TOKEN")); value != "" {
		return value, "environment", nil
	}
	if path := strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")); path != "" {
		value, err := credential.ReadTokenFile(path)
		if err != nil {
			return "", "token_file", fmt.Errorf("read SATEIA_TOKEN_FILE: %w", err)
		}
		return value, "token_file", nil
	}
	value, err := app.store.Get(baseURL)
	if err != nil {
		return "", "keyring", err
	}
	return value, "keyring", nil
}
