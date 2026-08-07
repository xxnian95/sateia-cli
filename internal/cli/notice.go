package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

const updateCommandPrefix = "go install github.com/xxnian95/sateia-cli/cmd/sateia@"

type updateChecker interface {
	Check(context.Context, string) (*updatecheck.Available, error)
}

type notice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Command string `json:"command,omitempty"`
}

func (app *application) notices(ctx context.Context) []notice {
	if strings.TrimSpace(os.Getenv("SATEIA_NO_UPDATE_NOTIFIER")) != "" || app.updateChecker == nil {
		return []notice{}
	}
	available, err := app.updateChecker.Check(ctx, app.version)
	if err != nil || available == nil {
		return []notice{}
	}
	return []notice{{
		Code:    "UPDATE_AVAILABLE",
		Message: fmt.Sprintf("Sateia CLI %s is available; current version is %s.", available.LatestVersion, available.CurrentVersion),
		Command: updateCommandPrefix + available.LatestVersion,
	}}
}

func (app *application) writeJSON(ctx context.Context, payload any) error {
	return app.writeJSONWithNotices(ctx, payload)
}

func (app *application) writeJSONWithNotices(ctx context.Context, payload any, commandNotices ...notice) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("build JSON response: %w", err)
	}
	allNotices := make([]notice, 0, len(commandNotices)+1)
	allNotices = append(allNotices, commandNotices...)
	allNotices = append(allNotices, app.notices(ctx)...)
	noticeData, err := json.Marshal(allNotices)
	if err != nil {
		return err
	}
	envelope["_notice"] = noticeData
	encoder := json.NewEncoder(app.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(envelope)
}

func (app *application) writeHumanNotices(ctx context.Context) {
	notices := app.notices(ctx)
	if len(notices) == 0 {
		return
	}
	fmt.Fprintln(app.out, "Notices:")
	for _, item := range notices {
		fmt.Fprintf(app.out, "- [%s] %s\n", item.Code, item.Message)
		if item.Command != "" {
			fmt.Fprintf(app.out, "  Run: %s\n", item.Command)
		}
	}
}
