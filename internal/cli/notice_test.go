package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

func TestUpdateNotifierCanBeDisabled(t *testing.T) {
	t.Setenv("SATEIA_NO_UPDATE_NOTIFIER", "1")
	called := false
	app := application{
		version: "v1.0.0",
		updateChecker: stubUpdateChecker{check: func(context.Context, string) (*updatecheck.Available, error) {
			called = true
			return &updatecheck.Available{CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0"}, nil
		}},
	}
	if notices := app.notices(context.Background()); len(notices) != 0 {
		t.Fatalf("unexpected notices: %#v", notices)
	}
	if called {
		t.Fatal("disabled notifier called the update checker")
	}
}

func TestUpdateCheckFailureDoesNotCreateNotice(t *testing.T) {
	app := application{
		version: "v1.0.0",
		updateChecker: stubUpdateChecker{check: func(context.Context, string) (*updatecheck.Available, error) {
			return nil, errors.New("network unavailable")
		}},
	}
	if notices := app.notices(context.Background()); len(notices) != 0 {
		t.Fatalf("unexpected notices: %#v", notices)
	}
}
