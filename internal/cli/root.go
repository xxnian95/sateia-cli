package cli

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/config"
	"github.com/xxnian95/sateia-cli/internal/credential"
)

const defaultServer = "https://xxnian.site/sateia-server"

const rootDescription = `Sateia writes server-side nutrition records that can be synchronized to the
Sateia app.

Quick start:
  1. In the Sateia app, open Settings > CLI Access and create a code.
  2. Run "sateia auth login" and enter the same device name and code.
  3. Run "sateia auth status" to verify the credential.
  4. Run "sateia record create --help" before the first write.

For headless automation, set SATEIA_TOKEN instead of running interactive login.
Run "sateia environment" for credential storage, precedence, and agent guidance.`

type application struct {
	version string
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	server  string
	store   credential.Store
}

func New(version string, in io.Reader, out, errOut io.Writer) *cobra.Command {
	app := &application{
		version: version,
		in:      in,
		out:     out,
		errOut:  errOut,
		store:   credential.KeyringStore{},
	}
	root := &cobra.Command{
		Use:   "sateia",
		Short: "Write nutrition records to Sateia",
		Long:  rootDescription,
		Example: `  # Interactive authentication
  sateia auth login
  sateia auth status

  # Inspect the write contract
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
	value, err := app.store.Get(baseURL)
	if err != nil {
		return "", "", err
	}
	return value, "keyring", nil
}
