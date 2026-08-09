package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
)

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,6})?$`)

type createOptions struct {
	energy       string
	protein      string
	carbohydrate string
	fat          string
	note         string
	consumedAt   string
	recordID     string
	mutationID   string
	jsonOutput   bool
}

type listOptions struct {
	consumedFrom   string
	consumedBefore string
	includeDeleted bool
	cursor         string
	limit          int
	jsonOutput     bool
}

func (app *application) newRecordCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "record",
		Short: "Manage nutrition records",
		Long: `Query and manage server-side nutrition records.

Use "sateia record list --help" for read-only queries. Create, update, and
delete are write operations; verify authentication and inspect the selected
subcommand's help before invoking it. Use --json when another program or an AI
agent will consume the result.`,
	}
	command.AddCommand(
		app.newRecordListCommand(),
		app.newRecordCreateCommand(),
		app.newRecordUpdateCommand(),
		app.newRecordDeleteCommand(),
	)
	return command
}

func (app *application) newRecordListCommand() *cobra.Command {
	options := listOptions{}
	command := &cobra.Command{
		Use:   "list",
		Short: "List nutrition records in a consumed-time window",
		Long: `List one page of nutrition records with GET /v1/nutrition-records.

--consumed-from is inclusive and --consumed-before is exclusive. Both must be
RFC 3339 timestamps, and the lower bound must be earlier than the upper bound.
Results are ordered by consumed_at descending, then record_id descending.

The CLI does not paginate automatically. For the next page, repeat the command
with the same filters and pass the returned next_cursor as --cursor. A cursor is
opaque and is valid only with the same consumed-time bounds, deletion filter,
and limit. Omit --cursor for the first page. JSON output includes top-level
request_id for correlation with server logs and audit events.`,
		Example: `  sateia record list \
    --consumed-from 2026-08-01T00:00:00+08:00 \
    --consumed-before 2026-08-08T00:00:00+08:00 \
    --limit 50 \
    --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			from, err := time.Parse(time.RFC3339, options.consumedFrom)
			if err != nil {
				return fmt.Errorf("--consumed-from must be an RFC 3339 timestamp such as 2026-08-01T00:00:00+08:00: %w", err)
			}
			before, err := time.Parse(time.RFC3339, options.consumedBefore)
			if err != nil {
				return fmt.Errorf("--consumed-before must be an RFC 3339 timestamp such as 2026-08-08T00:00:00+08:00: %w", err)
			}
			if !from.Before(before) {
				return errors.New("--consumed-from must be earlier than --consumed-before; the lower bound is inclusive and the upper bound is exclusive")
			}
			if options.limit < 1 || options.limit > 100 {
				return errors.New("--limit must be between 1 and 100")
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
			page, metadata, err := client.ListNutritionRecords(command.Context(), api.ListNutritionRecordsOptions{
				ConsumedFrom: from, ConsumedBefore: before, IncludeDeleted: options.includeDeleted,
				Cursor: options.cursor, Limit: options.limit,
			})
			if err != nil {
				return listErrorWithGuidance(err)
			}
			if options.jsonOutput {
				output := struct {
					RequestID  string                `json:"request_id"`
					Records    []api.NutritionRecord `json:"records"`
					NextCursor *string               `json:"next_cursor"`
					HasMore    bool                  `json:"has_more"`
				}{RequestID: metadata.RequestID, Records: page.Records, NextCursor: page.NextCursor, HasMore: page.HasMore}
				if page.HasMore && page.NextCursor != nil {
					return app.writeJSONWithNotices(command.Context(), output, notice{
						Code:    "NEXT_PAGE",
						Message: "More records are available. Repeat the command with the same filters and pass next_cursor as --cursor.",
					})
				}
				return app.writeJSON(command.Context(), output)
			}
			printNutritionRecordPage(app.out, page, metadata.RequestID)
			app.writeHumanNotices(command.Context())
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.consumedFrom, "consumed-from", "", "inclusive RFC 3339 lower bound, for example 2026-08-01T00:00:00+08:00 (required)")
	flags.StringVar(&options.consumedBefore, "consumed-before", "", "exclusive RFC 3339 upper bound, for example 2026-08-08T00:00:00+08:00 (required)")
	flags.IntVar(&options.limit, "limit", 0, "maximum records in this page, from 1 to 100 (required)")
	flags.BoolVar(&options.includeDeleted, "include-deleted", false, "include soft-deleted records")
	flags.StringVar(&options.cursor, "cursor", "", "opaque next_cursor from the previous page, with the same filters")
	flags.BoolVar(&options.jsonOutput, "json", false, "print request ID, records, and pagination metadata as JSON")
	for _, name := range []string{"consumed-from", "consumed-before", "limit"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func listErrorWithGuidance(err error) error {
	wrapped := fmt.Errorf("list nutrition records failed: %w", err)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%w\nNext: this operation is read-only; after restoring connectivity, retry the same command with unchanged filters", wrapped)
	}
	switch apiErr.Code {
	case "UNAUTHENTICATED":
		return fmt.Errorf("%w\nNext: run \"sateia auth status\" with the same credential source, repair that credential, then retry the same list command", wrapped)
	case "INVALID_CURSOR":
		// Cursors are bound to the complete filter set and must never be repaired heuristically.
		return fmt.Errorf("%w\nNext: restore the exact filters that produced this cursor, or start again without --cursor; never edit or decode a cursor", wrapped)
	case "VALIDATION_ERROR", "INVALID_PARAMETER", "MALFORMED_REQUEST":
		return fmt.Errorf("%w\nNext: correct the reported flag or time window, then start the query again; do not reuse a cursor with changed filters", wrapped)
	case "RATE_LIMIT_EXCEEDED":
		return fmt.Errorf("%w\nNext: wait for the rate limit to clear, then retry the same read-only command", wrapped)
	}
	if apiErr.Retryable || apiErr.StatusCode >= 500 {
		return fmt.Errorf("%w\nNext: the read did not change server data; retry the same command after the temporary failure clears", wrapped)
	}
	return fmt.Errorf("%w\nNext: inspect the server error and request_id, if present, before changing filters or retrying", wrapped)
}

func printNutritionRecordPage(output io.Writer, page api.NutritionRecordPage, requestID string) {
	fmt.Fprintln(output, "Nutrition records.")
	if requestID != "" {
		fmt.Fprintf(output, "request_id: %s\n", requestID)
	}
	fmt.Fprintf(output, "records: %d\n", len(page.Records))
	for index, record := range page.Records {
		fmt.Fprintf(output, "\n[%d]\nrecord_id: %s\nconsumed_at: %s\nconsumed_time_zone_offset_minutes: %d\n", index+1, record.RecordID, record.ConsumedAt.Format(time.RFC3339Nano), record.ConsumedTimeZoneOffsetMinutes)
		fmt.Fprintf(output, "energy_kcal: %s\nprotein_g: %s\ncarbohydrate_g: %s\nfat_g: %s\n", record.Nutrients["energy"], record.Nutrients["protein"], record.Nutrients["carbohydrate"], record.Nutrients["fat"])
		fmt.Fprintf(output, "source: %s\nversion: %d\n", record.Source, record.Version)
		if record.Note != nil {
			fmt.Fprintf(output, "note: %q\n", *record.Note)
		}
		if record.DeletedAt == nil {
			fmt.Fprintln(output, "status: active")
		} else {
			fmt.Fprintf(output, "status: deleted\ndeleted_at: %s\n", record.DeletedAt.Format(time.RFC3339Nano))
		}
	}
	fmt.Fprintf(output, "\nhas_more: %t\n", page.HasMore)
	if page.NextCursor == nil {
		fmt.Fprintln(output, "next_cursor: null")
	} else {
		fmt.Fprintf(output, "next_cursor: %s\n", *page.NextCursor)
	}
	if page.HasMore && page.NextCursor != nil {
		fmt.Fprintf(output, "Next: repeat this command with the same filters and --cursor %q.\n", *page.NextCursor)
	} else {
		fmt.Fprintln(output, "End of results. For machine-readable output, add --json.")
	}
}

func (app *application) newRecordCreateCommand() *cobra.Command {
	options := createOptions{}
	command := &cobra.Command{
		Use:   "create",
		Short: "Create one nutrition record",
		Long: `Create one server-side nutrition record.

Energy, protein, carbohydrate, and fat are required non-negative decimal
strings with at most six fractional digits. Energy is measured in kilocalories;
the other values are measured in grams. --consumed-at must be RFC 3339 with an
explicit UTC offset and defaults to now.

The CLI generates record_id and mutation_id. After an ambiguous network
failure, retry the exact same payload with both identifiers printed in the
error. Using new identifiers may create a duplicate record. JSON output
includes top-level request_id for correlation with server logs and audit
events.`,
		Example: `  sateia record create \
    --energy 520 \
    --protein 28.5 \
    --carbohydrate 62 \
    --fat 18 \
    --note "Pengnian lunch" \
    --consumed-at 2026-08-07T12:30:00+08:00 \
    --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			request, err := buildCreateRequest(options, time.Now())
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
			record, metadata, err := client.CreateNutritionRecord(command.Context(), request)
			if err != nil {
				return createErrorWithGuidance(err, request)
			}
			if options.jsonOutput {
				output := struct {
					MutationID string              `json:"mutation_id"`
					RequestID  string              `json:"request_id"`
					Record     api.NutritionRecord `json:"record"`
				}{MutationID: request.MutationID, RequestID: metadata.RequestID, Record: record}
				return app.writeJSON(command.Context(), output)
			}
			fmt.Fprintf(app.out, `Nutrition record created.
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
	flags := command.Flags()
	flags.StringVar(&options.energy, "energy", "", "energy in kilocalories (required)")
	flags.StringVar(&options.protein, "protein", "", "protein in grams (required)")
	flags.StringVar(&options.carbohydrate, "carbohydrate", "", "carbohydrate in grams (required)")
	flags.StringVar(&options.fat, "fat", "", "fat in grams (required)")
	flags.StringVar(&options.note, "note", "", "optional note (maximum 500 characters)")
	flags.StringVar(&options.consumedAt, "consumed-at", "", "RFC 3339 timestamp with offset (default: now)")
	flags.StringVar(&options.recordID, "record-id", "", "UUID to reuse when retrying a create")
	flags.StringVar(&options.mutationID, "mutation-id", "", "UUID to reuse when retrying a create")
	flags.BoolVar(&options.jsonOutput, "json", false, "print the request ID and created record as JSON")
	for _, name := range []string{"energy", "protein", "carbohydrate", "fat"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func createErrorWithGuidance(err error, request api.CreateRecordRequest) error {
	wrapped := fmt.Errorf("create record failed (record_id=%s, mutation_id=%s): %w", request.Record.RecordID, request.MutationID, err)
	exactRetry := fmt.Sprintf("Retry the exact same request with --record-id %s --mutation-id %s; do not generate new IDs", request.Record.RecordID, request.MutationID)

	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%w\n%s", wrapped, exactRetry)
	}
	switch apiErr.Code {
	case "UNAUTHENTICATED":
		return fmt.Errorf("%w\nNext: verify and repair the same credential source with \"sateia auth status\", then %s", wrapped, exactRetry)
	case "VALIDATION_ERROR", "INVALID_PARAMETER", "MALFORMED_REQUEST":
		return fmt.Errorf("%w\nNext: correct the reported input and submit a new request; never reuse a mutation_id with a different payload", wrapped)
	case "IDEMPOTENCY_CONFLICT":
		return fmt.Errorf("%w\nStop: recover the original payload for this mutation_id instead of guessing or changing values", wrapped)
	}
	if apiErr.Retryable || apiErr.StatusCode >= 500 {
		return fmt.Errorf("%w\n%s", wrapped, exactRetry)
	}
	return fmt.Errorf("%w\nNext: inspect the server error and do not retry blindly", wrapped)
}

func buildCreateRequest(options createOptions, now time.Time) (api.CreateRecordRequest, error) {
	nutrients := map[string]string{
		"energy":       options.energy,
		"protein":      options.protein,
		"carbohydrate": options.carbohydrate,
		"fat":          options.fat,
	}
	for name, amount := range nutrients {
		if !decimalPattern.MatchString(amount) {
			return api.CreateRecordRequest{}, fmt.Errorf("--%s must be a non-negative decimal with at most six fractional digits", name)
		}
	}
	if len([]rune(options.note)) > 500 {
		return api.CreateRecordRequest{}, errors.New("--note must contain at most 500 characters")
	}

	consumedAt := now
	var err error
	if options.consumedAt != "" {
		consumedAt, err = time.Parse(time.RFC3339, options.consumedAt)
		if err != nil {
			return api.CreateRecordRequest{}, fmt.Errorf("--consumed-at must be an RFC 3339 timestamp with an explicit UTC offset, such as 2026-08-07T12:30:00+08:00: %w", err)
		}
	}
	_, offsetSeconds := consumedAt.Zone()
	if offsetSeconds%60 != 0 || offsetSeconds/60 < -840 || offsetSeconds/60 > 840 {
		return api.CreateRecordRequest{}, errors.New("--consumed-at must use a UTC offset between -14:00 and +14:00 in whole minutes")
	}

	recordID := strings.ToLower(strings.TrimSpace(options.recordID))
	if recordID == "" {
		recordID, err = newUUID()
		if err != nil {
			return api.CreateRecordRequest{}, err
		}
	} else if !validUUID(recordID) {
		return api.CreateRecordRequest{}, errors.New("--record-id must be a UUID")
	}
	mutationID := strings.ToLower(strings.TrimSpace(options.mutationID))
	if mutationID == "" {
		mutationID, err = newUUID()
		if err != nil {
			return api.CreateRecordRequest{}, err
		}
	} else if !validUUID(mutationID) {
		return api.CreateRecordRequest{}, errors.New("--mutation-id must be a UUID")
	}

	var note *string
	if options.note != "" {
		note = &options.note
	}
	return api.CreateRecordRequest{
		MutationID: mutationID,
		Record: api.NutritionRecordInput{
			RecordID:                      recordID,
			ConsumedAt:                    consumedAt,
			ConsumedTimeZoneOffsetMinutes: offsetSeconds / 60,
			Nutrients:                     nutrients,
			Note:                          note,
		},
	}, nil
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}
