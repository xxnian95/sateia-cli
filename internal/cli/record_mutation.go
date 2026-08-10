package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
)

type updateOptions struct {
	recordID        string
	expectedVersion int64
	mutationID      string
	energy          string
	protein         string
	carbohydrate    string
	fat             string
	note            string
	clearNote       bool
	consumedAt      string
}

type updateSelection struct {
	energy       bool
	protein      bool
	carbohydrate bool
	fat          bool
	note         bool
	consumedAt   bool
}

type deleteOptions struct {
	recordID        string
	expectedVersion int64
	mutationID      string
}

func (app *application) newRecordUpdateCommand() *cobra.Command {
	options := updateOptions{}
	command := &cobra.Command{
		Use:   "update",
		Short: "Update one nutrition record",
		Long: `Update mutable fields on one server-side nutrition record with PATCH.

--record-id and --expected-version identify the exact record state to change.
Omitted mutable flags remain unchanged. If any nutrient flag is supplied, all
four nutrient flags are required because the server replaces the complete
nutrient map. --consumed-at carries its UTC offset, so the CLI submits the time
and offset together. Use --note to replace a note or --clear-note to store null.

The CLI generates mutation_id. After an ambiguous network or temporary server
failure, retry the exact same payload with the printed mutation ID and expected
version. If any field changes, generate a new mutation ID. A version conflict
requires reviewing the current record before issuing a new update.

Field guidance:
  --record-id is the exact UUID returned by a trusted read, and
  --expected-version is that record's current positive integer version.
  Nutrient values are total kilocalories or grams as plain non-negative
  decimals. Supply all four together or omit all four.
  --note stores the exact string, including an empty string; --clear-note
  stores null. For a user-facing note, keep the food name and quantity first
  and append provenance or external IDs afterward.
  --consumed-at is the replacement consumption time in RFC 3339 form with its
  original UTC offset. Omit unchanged fields.
  Omit --mutation-id for a new update; reuse it only for an exact retry.

AI agents: immediately before every update attempt, run
"sateia record update --help" again, including before a retry. Installed CLI
updates may change these instructions.`,
		Example: `  sateia record update \
    --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
    --expected-version 1 \
    --energy 610 \
    --protein 32 \
    --carbohydrate 70 \
    --fat 22 \
    --note "Corrected lunch" \
    --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selection := updateSelection{
				energy:       command.Flags().Changed("energy"),
				protein:      command.Flags().Changed("protein"),
				carbohydrate: command.Flags().Changed("carbohydrate"),
				fat:          command.Flags().Changed("fat"),
				note:         command.Flags().Changed("note"),
				consumedAt:   command.Flags().Changed("consumed-at"),
			}
			request, recordID, err := buildUpdateRequest(options, selection)
			if err != nil {
				return err
			}
			baseURL, _, err := app.baseURL()
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
			record, metadata, err := client.UpdateNutritionRecord(command.Context(), recordID, request)
			if err != nil {
				return recordMutationErrorWithGuidance("update", err, recordID, request.MutationID, request.ExpectedVersion)
			}
			if app.jsonOutput {
				output := struct {
					MutationID string              `json:"mutation_id"`
					RequestID  string              `json:"request_id"`
					Record     api.NutritionRecord `json:"record"`
				}{MutationID: request.MutationID, RequestID: metadata.RequestID, Record: record}
				return app.writeJSON(command.Context(), output)
			}
			fmt.Fprintf(app.out, `Nutrition record updated.
record_id: %s
mutation_id: %s
version: %d
source: %s
consumed_at: %s
`, record.RecordID, request.MutationID, record.Version, record.Source, record.ConsumedAt.Format(time.RFC3339))
			if metadata.RequestID != "" {
				fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
			}
			fmt.Fprintln(app.out, "For machine-readable output, add --json.")
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskWrite)
	flags := command.Flags()
	flags.StringVar(&options.recordID, "record-id", "", "exact record UUID from a trusted read (required)")
	flags.Int64Var(&options.expectedVersion, "expected-version", 0, "current positive integer version from the same trusted read (required)")
	flags.StringVar(&options.mutationID, "mutation-id", "", "mutation UUID from a failed update; omit initially and reuse only for the exact unchanged retry")
	flags.StringVar(&options.energy, "energy", "", "total replacement kilocalories as a non-negative decimal; requires all nutrient flags")
	flags.StringVar(&options.protein, "protein", "", "total replacement protein grams as a non-negative decimal; requires all nutrient flags")
	flags.StringVar(&options.carbohydrate, "carbohydrate", "", "total replacement carbohydrate grams as a non-negative decimal; requires all nutrient flags")
	flags.StringVar(&options.fat, "fat", "", "total replacement fat grams as a non-negative decimal; requires all nutrient flags")
	flags.StringVar(&options.note, "note", "", "exact replacement note up to 500 characters; food name and quantity should come first")
	flags.BoolVar(&options.clearNote, "clear-note", false, "replace the note with null; mutually exclusive with --note")
	flags.StringVar(&options.consumedAt, "consumed-at", "", "actual replacement consumption time as RFC 3339 with its original UTC offset")
	for _, name := range []string{"record-id", "expected-version"} {
		_ = command.MarkFlagRequired(name)
	}
	command.MarkFlagsRequiredTogether("energy", "protein", "carbohydrate", "fat")
	command.MarkFlagsMutuallyExclusive("note", "clear-note")
	return command
}

func buildUpdateRequest(options updateOptions, selection updateSelection) (api.UpdateRecordRequest, string, error) {
	recordID, err := canonicalRequiredUUID(options.recordID, "--record-id")
	if err != nil {
		return api.UpdateRecordRequest{}, "", err
	}
	if options.expectedVersion < 1 {
		return api.UpdateRecordRequest{}, "", errors.New("--expected-version must be at least one")
	}
	mutationID, err := canonicalMutationID(options.mutationID)
	if err != nil {
		return api.UpdateRecordRequest{}, "", err
	}
	request := api.UpdateRecordRequest{MutationID: mutationID, ExpectedVersion: options.expectedVersion}

	nutrientSelections := []bool{selection.energy, selection.protein, selection.carbohydrate, selection.fat}
	selectedNutrients := 0
	for _, selected := range nutrientSelections {
		if selected {
			selectedNutrients++
		}
	}
	// The API treats a supplied nutrient map as a full replacement, never a partial patch.
	if selectedNutrients != 0 && selectedNutrients != len(nutrientSelections) {
		return api.UpdateRecordRequest{}, "", errors.New("--energy, --protein, --carbohydrate, and --fat must be supplied together because an update replaces the complete nutrient map")
	}
	if selectedNutrients == len(nutrientSelections) {
		request.Nutrients = map[string]string{
			"energy": options.energy, "protein": options.protein,
			"carbohydrate": options.carbohydrate, "fat": options.fat,
		}
		for name, amount := range request.Nutrients {
			if !decimalPattern.MatchString(amount) {
				return api.UpdateRecordRequest{}, "", fmt.Errorf("--%s must be a non-negative decimal with at most six fractional digits", name)
			}
		}
	}

	if selection.consumedAt {
		consumedAt, parseErr := time.Parse(time.RFC3339, options.consumedAt)
		if parseErr != nil {
			return api.UpdateRecordRequest{}, "", fmt.Errorf("--consumed-at must be an RFC 3339 timestamp with an explicit UTC offset, such as 2026-08-07T12:30:00+08:00: %w", parseErr)
		}
		_, offsetSeconds := consumedAt.Zone()
		if offsetSeconds%60 != 0 || offsetSeconds/60 < -840 || offsetSeconds/60 > 840 {
			return api.UpdateRecordRequest{}, "", errors.New("--consumed-at must use a UTC offset between -14:00 and +14:00 in whole minutes")
		}
		offsetMinutes := offsetSeconds / 60
		request.ConsumedAt = &consumedAt
		request.ConsumedTimeZoneOffsetMinutes = &offsetMinutes
	}

	if selection.note && options.clearNote {
		return api.UpdateRecordRequest{}, "", errors.New("--note and --clear-note are mutually exclusive")
	}
	if selection.note {
		if len([]rune(options.note)) > 500 {
			return api.UpdateRecordRequest{}, "", errors.New("--note must contain at most 500 characters")
		}
		note := options.note
		request.Note = api.OptionalString{Set: true, Value: &note}
	} else if options.clearNote {
		request.Note = api.OptionalString{Set: true}
	}

	if request.Nutrients == nil && request.ConsumedAt == nil && !request.Note.Set {
		return api.UpdateRecordRequest{}, "", errors.New("update requires at least one mutable flag: --consumed-at, all four nutrient flags, --note, or --clear-note")
	}
	return request, recordID, nil
}

func (app *application) newRecordDeleteCommand() *cobra.Command {
	options := deleteOptions{}
	command := &cobra.Command{
		Use:   "delete",
		Short: "Soft-delete one nutrition record",
		Long: `Soft-delete one server-side nutrition record.

--expected-version prevents deleting a record state that changed after it was
reviewed. The CLI generates mutation_id. If the result is ambiguous, retry the
same record ID, expected version, and mutation ID. Replaying the same successful
deletion returns its original tombstone without incrementing the version again.
The CLI cannot restore a deleted record.

Field guidance:
  --record-id is the exact UUID returned by a trusted read.
  --expected-version is that record's current positive integer version.
  Omit --mutation-id initially; supply the exact UUID printed by a failed
  delete only when retrying the same record and version.

AI agents: immediately before every delete attempt, run
"sateia record delete --help" again, including before a retry. Installed CLI
updates may change these instructions.`,
		Example: `  sateia record delete \
    --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
    --expected-version 2 \
    --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			recordID, err := canonicalRequiredUUID(options.recordID, "--record-id")
			if err != nil {
				return err
			}
			if options.expectedVersion < 1 {
				return errors.New("--expected-version must be at least one")
			}
			mutationID, err := canonicalMutationID(options.mutationID)
			if err != nil {
				return err
			}
			baseURL, _, err := app.baseURL()
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
			tombstone, metadata, err := client.DeleteNutritionRecord(command.Context(), recordID, mutationID, options.expectedVersion)
			if err != nil {
				return recordMutationErrorWithGuidance("delete", err, recordID, mutationID, options.expectedVersion)
			}
			if app.jsonOutput {
				output := struct {
					MutationID string                `json:"mutation_id"`
					RequestID  string                `json:"request_id"`
					Tombstone  api.DeletionTombstone `json:"tombstone"`
				}{MutationID: mutationID, RequestID: metadata.RequestID, Tombstone: tombstone}
				return app.writeJSON(command.Context(), output)
			}
			fmt.Fprintf(app.out, `Nutrition record deleted.
record_id: %s
mutation_id: %s
version: %d
deleted_at: %s
`, tombstone.RecordID, mutationID, tombstone.Version, tombstone.DeletedAt.Format(time.RFC3339Nano))
			if metadata.RequestID != "" {
				fmt.Fprintf(app.out, "request_id: %s\n", metadata.RequestID)
			}
			fmt.Fprintln(app.out, "For machine-readable output, add --json.")
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	setCommandRisk(command, riskSoftDelete)
	flags := command.Flags()
	flags.StringVar(&options.recordID, "record-id", "", "exact record UUID from a trusted read (required)")
	flags.Int64Var(&options.expectedVersion, "expected-version", 0, "current positive integer version from the same trusted read (required)")
	flags.StringVar(&options.mutationID, "mutation-id", "", "mutation UUID from a failed delete; omit initially and reuse only for the exact unchanged retry")
	for _, name := range []string{"record-id", "expected-version"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func canonicalRequiredUUID(value, flagName string) (string, error) {
	canonical := strings.ToLower(strings.TrimSpace(value))
	if !validUUID(canonical) {
		return "", fmt.Errorf("%s must be a UUID", flagName)
	}
	return canonical, nil
}

func canonicalMutationID(value string) (string, error) {
	canonical := strings.ToLower(strings.TrimSpace(value))
	if canonical == "" {
		generated, err := newUUID()
		if err != nil {
			return "", err
		}
		return generated, nil
	}
	if !validUUID(canonical) {
		return "", errors.New("--mutation-id must be a UUID")
	}
	return canonical, nil
}

func recordMutationErrorWithGuidance(operation string, err error, recordID, mutationID string, expectedVersion int64) error {
	wrapped := fmt.Errorf("%s record failed (record_id=%s, expected_version=%d, mutation_id=%s): %w", operation, recordID, expectedVersion, mutationID, err)
	exactRetry := fmt.Sprintf("retry the exact same %s with --record-id %s --expected-version %d --mutation-id %s and unchanged mutable fields", operation, recordID, expectedVersion, mutationID)

	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%w\nNext: after connectivity is restored, %s", wrapped, exactRetry)
	}
	switch apiErr.Code {
	case "UNAUTHENTICATED":
		return fmt.Errorf("%w\nNext: verify and repair the same credential source with \"sateia auth status\", then %s", wrapped, exactRetry)
	case "VALIDATION_ERROR", "INVALID_PARAMETER", "MALFORMED_REQUEST":
		return fmt.Errorf("%w\nNext: correct the reported input and submit a new request with a new mutation ID; never reuse a mutation_id with changed content", wrapped)
	case "IDEMPOTENCY_CONFLICT":
		return fmt.Errorf("%w\nStop: recover the original request for this mutation_id instead of guessing or changing values", wrapped)
	case "VERSION_CONFLICT":
		return fmt.Errorf("%w\nStop: review the current record and version, then submit the intended %s with a new mutation ID; never guess the version", wrapped, operation)
	case "RECORD_NOT_FOUND":
		return fmt.Errorf("%w\nStop: verify the record ID and whether the record was already deleted before attempting another mutation", wrapped)
	}
	if apiErr.Retryable || apiErr.StatusCode >= 500 {
		return fmt.Errorf("%w\nNext: %s", wrapped, exactRetry)
	}
	return fmt.Errorf("%w\nNext: inspect the server error and request_id, if present, before retrying", wrapped)
}
