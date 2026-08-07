package cli

import (
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

const rootDescription = `Sateia queries and writes server-side nutrition records that can be synchronized
to the Sateia app.

Quick start:
  1. In Sateia app > Settings > CLI Access, create a one-time code.
  2. Identify this machine with hostname and choose a stable device name.
  3. Run "sateia auth login" with the chosen device name and code.
  4. Run "sateia auth status" to verify the credential.
  5. Run "sateia record list --help" or "sateia record create --help".

For headless automation, set SATEIA_TOKEN or SATEIA_TOKEN_FILE. Device-code
login can create a private token file with --token-file when Linux Secret
Service is unavailable. Run "sateia environment" for credential precedence
and agent guidance. JSON responses include an additive _notice list for next
steps and available CLI updates.`

type application struct {
	version       string
	in            io.Reader
	out           io.Writer
	errOut        io.Writer
	server        string
	store         credential.Store
	updateChecker updateChecker
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

  # Inspect the read and write contracts
  sateia record list --help
  sateia record create --help`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&app.server, "server", "", "Sateia API base URL (or SATEIA_SERVER)")
	root.AddCommand(app.newAuthCommand(), app.newRecordCommand(), app.newEnvironmentCommand())
	return root
}

func (app *application) baseURL() (string, config.Config, error) {
	stored, err := config.Load()
	if err != nil {
		return "", config.Config{}, err
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
	return normalized, stored, err
}

func (app *application) token(baseURL string) (string, string, error) {
	if value := strings.TrimSpace(os.Getenv("SATEIA_TOKEN")); value != "" {
		return value, "environment", nil
	}
	if path := strings.TrimSpace(os.Getenv("SATEIA_TOKEN_FILE")); path != "" {
		value, err := credential.ReadTokenFile(path)
		if err != nil {
			return "", "", fmt.Errorf("read SATEIA_TOKEN_FILE: %w", err)
		}
		return value, "token_file", nil
	}
	value, err := app.store.Get(baseURL)
	if err != nil {
		return "", "", err
	}
	return value, "keyring", nil
}
