package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
)

type goalSetOptions struct {
	energy       string
	protein      string
	carbohydrate string
	fat          string
}

func (app *application) newGoalCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "goal",
		Short: "Manage date-specific nutrition goals",
		Long: `Manage custom goals keyed by an exact calendar date.

Dates use yyyy-MM-dd and carry no time or time-zone semantics. A custom goal
overrides the app's Settings goals for that date. Deleting it restores the
Settings fallback.`,
	}
	setCommandRisk(command, riskNone)
	command.AddCommand(app.newGoalGetCommand(), app.newGoalSetCommand(), app.newGoalDeleteCommand())
	return command
}

func (app *application) newGoalGetCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "get DATE",
		Short: "Get the custom goal for a calendar date",
		Long: `Get the custom goal for one exact calendar date.

DATE must be a valid yyyy-MM-dd value, such as 2026-08-09. It has no time or
time-zone semantics. Use --json for machine-readable output.

AI agents: immediately before every lookup, run "sateia goal get --help"
again. Installed CLI updates may change these instructions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			date, err := canonicalGoalDate(args[0])
			if err != nil {
				return err
			}
			client, err := app.authenticatedAPIClient()
			if err != nil {
				return err
			}
			goal, metadata, err := client.GetDailyGoal(command.Context(), date)
			if err != nil {
				return fmt.Errorf("get daily goal failed: %w", err)
			}
			return app.printDailyGoal(command, goal, metadata, app.jsonOutput)
		},
	}
	setCommandRisk(command, riskReadOnly)
	return command
}

func (app *application) newGoalSetCommand() *cobra.Command {
	options := goalSetOptions{}
	command := &cobra.Command{
		Use:   "set DATE",
		Short: "Create or replace the custom goal for a calendar date",
		Long: `Create or replace the custom goal for one exact calendar date.

DATE must be a valid yyyy-MM-dd value, such as 2026-08-09, with no time or
time-zone semantics. Supply all four goals as plain non-negative decimals with
at most six fractional digits. Energy is in kilocalories; protein,
carbohydrate, and fat are in grams. Do not include unit suffixes.

AI agents: immediately before every set operation, run
"sateia goal set --help" again. Installed CLI updates may change these
instructions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			date, err := canonicalGoalDate(args[0])
			if err != nil {
				return err
			}
			nutrients := map[string]string{
				"energy": options.energy, "protein": options.protein,
				"carbohydrate": options.carbohydrate, "fat": options.fat,
			}
			for name, amount := range nutrients {
				if !decimalPattern.MatchString(amount) {
					return fmt.Errorf("--%s must be a non-negative decimal with at most six fractional digits", name)
				}
			}
			client, err := app.authenticatedAPIClient()
			if err != nil {
				return err
			}
			goal, metadata, err := client.PutDailyGoal(command.Context(), date, api.DailyGoalInput{Nutrients: nutrients})
			if err != nil {
				return fmt.Errorf("set daily goal failed: %w", err)
			}
			return app.printDailyGoal(command, goal, metadata, app.jsonOutput)
		},
	}
	setCommandRisk(command, riskWrite)
	flags := command.Flags()
	flags.StringVar(&options.energy, "energy", "", "kilocalorie goal as a non-negative decimal without a unit suffix (required)")
	flags.StringVar(&options.protein, "protein", "", "protein gram goal as a non-negative decimal without a unit suffix (required)")
	flags.StringVar(&options.carbohydrate, "carbohydrate", "", "carbohydrate gram goal as a non-negative decimal without a unit suffix (required)")
	flags.StringVar(&options.fat, "fat", "", "fat gram goal as a non-negative decimal without a unit suffix (required)")
	for _, name := range []string{"energy", "protein", "carbohydrate", "fat"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func (app *application) newGoalDeleteCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "delete DATE",
		Short: "Delete the custom goal and restore the Settings fallback",
		Long: `Delete the custom goal for one exact calendar date.

DATE must be a valid yyyy-MM-dd value, such as 2026-08-09, with no time or
time-zone semantics. Deletion restores the app Settings fallback for that
date. Use --json for machine-readable output.

AI agents: immediately before every delete operation, run
"sateia goal delete --help" again. Installed CLI updates may change these
instructions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			date, err := canonicalGoalDate(args[0])
			if err != nil {
				return err
			}
			client, err := app.authenticatedAPIClient()
			if err != nil {
				return err
			}
			metadata, err := client.DeleteDailyGoal(command.Context(), date)
			if err != nil {
				return fmt.Errorf("delete daily goal failed: %w", err)
			}
			if app.jsonOutput {
				return app.writeJSON(command.Context(), struct {
					RequestID string `json:"request_id"`
					GoalDate  string `json:"goal_date"`
					Deleted   bool   `json:"deleted"`
				}{metadata.RequestID, date, true})
			}
			fmt.Fprintf(app.out, "Daily goal deleted.\ngoal_date: %s\n", date)
			if metadata.RequestID != "" {
				fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
			}
			return nil
		},
	}
	setCommandRisk(command, riskWrite)
	return command
}

func (app *application) authenticatedAPIClient() (*api.Client, error) {
	baseURL, _, err := app.baseURL()
	if err != nil {
		return nil, err
	}
	token, source, err := app.token(baseURL)
	if err != nil {
		return nil, authenticationTokenError(err, source)
	}
	return api.NewClient(baseURL, token, app.version, nil)
}

func (app *application) printDailyGoal(command *cobra.Command, goal api.DailyGoal, metadata api.ResponseMetadata, jsonOutput bool) error {
	if jsonOutput {
		return app.writeJSON(command.Context(), struct {
			RequestID string        `json:"request_id"`
			DailyGoal api.DailyGoal `json:"daily_goal"`
		}{metadata.RequestID, goal})
	}
	fmt.Fprintf(app.out, "Daily goal.\ngoal_date: %s\nenergy_kcal: %s\nprotein_g: %s\ncarbohydrate_g: %s\nfat_g: %s\n", goal.GoalDate, goal.Nutrients["energy"], goal.Nutrients["protein"], goal.Nutrients["carbohydrate"], goal.Nutrients["fat"])
	if metadata.RequestID != "" {
		fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
	}
	return nil
}

func canonicalGoalDate(value string) (string, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", errors.New("DATE must be a valid calendar date in yyyy-MM-dd form, such as 2026-08-09")
	}
	return value, nil
}
