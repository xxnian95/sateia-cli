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
		Use:           "sateia",
		Short:         "Write nutrition records to Sateia",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&app.server, "server", "", "Sateia API base URL (or SATEIA_SERVER)")
	root.AddCommand(app.newAuthCommand(), app.newRecordCommand())
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
