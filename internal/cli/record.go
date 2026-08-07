package cli

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/credential"
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

func (app *application) newRecordCommand() *cobra.Command {
	command := &cobra.Command{Use: "record", Short: "Manage nutrition records"}
	command.AddCommand(app.newRecordCreateCommand())
	return command
}

func (app *application) newRecordCreateCommand() *cobra.Command {
	options := createOptions{}
	command := &cobra.Command{
		Use:   "create",
		Short: "Create one nutrition record",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			request, err := buildCreateRequest(options, time.Now())
			if err != nil {
				return err
			}
			baseURL, _, err := app.baseURL()
			if err != nil {
				return err
			}
			token, _, err := app.token(baseURL)
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
			record, err := client.CreateNutritionRecord(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create record (record_id=%s, mutation_id=%s): %w", request.Record.RecordID, request.MutationID, err)
			}
			if options.jsonOutput {
				encoder := json.NewEncoder(app.out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(record)
			}
			fmt.Fprintf(app.out, "Created record %s (version %d).\n", record.RecordID, record.Version)
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.energy, "energy", "", "energy in kilocalories")
	flags.StringVar(&options.protein, "protein", "", "protein in grams")
	flags.StringVar(&options.carbohydrate, "carbohydrate", "", "carbohydrate in grams")
	flags.StringVar(&options.fat, "fat", "", "fat in grams")
	flags.StringVar(&options.note, "note", "", "optional note (maximum 500 characters)")
	flags.StringVar(&options.consumedAt, "consumed-at", "", "RFC 3339 timestamp with offset (default: now)")
	flags.StringVar(&options.recordID, "record-id", "", "UUID to reuse when retrying a create")
	flags.StringVar(&options.mutationID, "mutation-id", "", "UUID to reuse when retrying a create")
	flags.BoolVar(&options.jsonOutput, "json", false, "print the created record as JSON")
	for _, name := range []string{"energy", "protein", "carbohydrate", "fat"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
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
			return api.CreateRecordRequest{}, fmt.Errorf("parse --consumed-at as RFC 3339: %w", err)
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
