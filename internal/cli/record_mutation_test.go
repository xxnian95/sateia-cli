package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/api"
)

func TestRecordUpdateEndToEndWithClearNote(t *testing.T) {
	var received map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.Path != "/v1/nutrition-records/014b2680-df5b-4c8d-97ef-abde0a9746d6" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-update-json")
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T12:30:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"610","protein":"32","carbohydrate":"70","fat":"22"},"source":"CLI","note":null,"version":2,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-08T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "record", "update",
		"--record-id", "014B2680-DF5B-4C8D-97EF-ABDE0A9746D6",
		"--expected-version", "1",
		"--mutation-id", "3FE5867D-F8CB-48D4-90B2-529A15531DB8",
		"--energy", "610", "--protein", "32", "--carbohydrate", "70", "--fat", "22",
		"--consumed-at", "2026-08-07T12:30:00+08:00",
		"--clear-note", "--json",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if string(received["mutation_id"]) != `"3fe5867d-f8cb-48d4-90b2-529a15531db8"` || string(received["expected_version"]) != "1" {
		t.Fatalf("unexpected mutation metadata: %#v", received)
	}
	if string(received["note"]) != "null" || string(received["consumed_time_zone_offset_minutes"]) != "480" {
		t.Fatalf("unexpected patch semantics: %#v", received)
	}
	var decoded struct {
		MutationID string              `json:"mutation_id"`
		RequestID  string              `json:"request_id"`
		Record     api.NutritionRecord `json:"record"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.MutationID != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || decoded.RequestID != "request-update-json" || decoded.Record.Version != 2 {
		t.Fatalf("unexpected output: %#v", decoded)
	}
}

func TestRecordUpdateRejectsPartialNutrientsBeforeAuthentication(t *testing.T) {
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{
		"record", "update",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--expected-version", "1",
		"--energy", "610",
	})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "must be supplied together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordUpdateRejectsMissingMutableField(t *testing.T) {
	_, _, err := buildUpdateRequest(updateOptions{
		recordID: "014b2680-df5b-4c8d-97ef-abde0a9746d6", expectedVersion: 1,
	}, updateSelection{})
	if err == nil || !strings.Contains(err.Error(), "at least one mutable flag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordUpdatePreservesExplicitEmptyNote(t *testing.T) {
	request, _, err := buildUpdateRequest(updateOptions{
		recordID: "014b2680-df5b-4c8d-97ef-abde0a9746d6", expectedVersion: 1,
	}, updateSelection{note: true})
	if err != nil {
		t.Fatal(err)
	}
	if !request.Note.Set || request.Note.Value == nil || *request.Note.Value != "" {
		t.Fatalf("empty note was not preserved: %#v", request.Note)
	}
}

func TestRecordDeleteEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Query().Get("expectedVersion") != "2" || request.URL.Query().Get("mutationId") != "3fe5867d-f8cb-48d4-90b2-529a15531db8" {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-delete-human")
		_, _ = writer.Write([]byte(`{"tombstone":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","version":3,"deleted_at":"2026-08-08T09:00:00Z"}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "record", "delete",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--expected-version", "2",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"Nutrition record deleted.",
		"record_id: 014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"mutation_id: 3fe5867d-f8cb-48d4-90b2-529a15531db8",
		"version: 3",
		"deleted_at: 2026-08-08T09:00:00Z",
		"request_id: request-delete-human",
	} {
		if !strings.Contains(output.String(), required) {
			t.Errorf("output does not contain %q:\n%s", required, output.String())
		}
	}
}

func TestRecordMutationErrorsProtectIdempotencyAndVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		required string
	}{
		{
			name:     "temporary failure",
			err:      &api.APIError{StatusCode: http.StatusServiceUnavailable, Code: "DATABASE_UNAVAILABLE", Retryable: true},
			required: "retry the exact same update with --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 --expected-version 2 --mutation-id 3fe5867d-f8cb-48d4-90b2-529a15531db8",
		},
		{
			name:     "version conflict",
			err:      &api.APIError{StatusCode: http.StatusConflict, Code: "VERSION_CONFLICT"},
			required: "never guess the version",
		},
		{
			name:     "idempotency conflict",
			err:      &api.APIError{StatusCode: http.StatusConflict, Code: "IDEMPOTENCY_CONFLICT"},
			required: "recover the original request",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			message := recordMutationErrorWithGuidance(
				"update", test.err,
				"014b2680-df5b-4c8d-97ef-abde0a9746d6",
				"3fe5867d-f8cb-48d4-90b2-529a15531db8",
				2,
			).Error()
			if !strings.Contains(message, test.required) {
				t.Fatalf("error does not contain %q: %s", test.required, message)
			}
		})
	}
}
