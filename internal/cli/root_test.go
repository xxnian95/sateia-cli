package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecordCreateEndToEndWithEnvironmentToken(t *testing.T) {
	var received struct {
		MutationID string `json:"mutation_id"`
		Record     struct {
			RecordID  string            `json:"record_id"`
			Nutrients map[string]string `json:"nutrients"`
		} `json:"record"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer environment-secret" {
			t.Errorf("unexpected authorization header %q", request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":"Pengnian lunch","version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL,
		"record", "create",
		"--energy", "520",
		"--protein", "28.5",
		"--carbohydrate", "62",
		"--fat", "18",
		"--note", "Pengnian lunch",
		"--consumed-at", "2026-08-07T16:00:00+08:00",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if received.MutationID != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || received.Record.Nutrients["protein"] != "28.5" {
		t.Fatalf("unexpected request payload: %#v", received)
	}
	expected := `Nutrition record created.
record_id: 014b2680-df5b-4c8d-97ef-abde0a9746d6
mutation_id: 3fe5867d-f8cb-48d4-90b2-529a15531db8
version: 1
source: CLI
consumed_at: 2026-08-07T16:00:00+08:00
For machine-readable output, add --json.
`
	if got := output.String(); got != expected {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestHelpTeachesCompleteAgentWorkflow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		required []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			required: []string{
				"Quick start:",
				"Settings > CLI Access",
				"For headless automation, set SATEIA_TOKEN",
				"sateia environment",
			},
		},
		{
			name: "login",
			args: []string{"auth", "login", "--help"},
			required: []string{
				"must exactly match",
				"sateia auth login --device-code ABCD-EFGH",
				"never printed",
			},
		},
		{
			name: "record create",
			args: []string{"record", "create", "--help"},
			required: []string{
				"non-negative decimal strings",
				"ambiguous network failure",
				"--energy string",
				"(required)",
				"--json",
			},
		},
		{
			name: "record list",
			args: []string{"record", "list", "--help"},
			required: []string{
				"GET /v1/nutrition-records",
				"--consumed-from",
				"--consumed-before",
				"--include-deleted",
				"same filters",
				"--json",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			command := New("test", strings.NewReader(""), &output, &output)
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			normalizedOutput := strings.Join(strings.Fields(output.String()), " ")
			for _, required := range test.required {
				if !strings.Contains(normalizedOutput, strings.Join(strings.Fields(required), " ")) {
					t.Errorf("help output does not contain %q:\n%s", required, output.String())
				}
			}
		})
	}
}

func TestEnvironmentCommandTeachesCredentialSafetyAndRetry(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{"environment"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"SATEIA_TOKEN overrides",
		"never the token secret",
		"removes the local credential only",
		"both the printed --record-id and --mutation-id",
	} {
		if !strings.Contains(output.String(), required) {
			t.Errorf("environment output does not contain %q", required)
		}
	}
}

func TestRecordCreateJSONIncludesMutationIdentifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":null,"version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "record", "create",
		"--energy", "520", "--protein", "28.5", "--carbohydrate", "62", "--fat", "18",
		"--consumed-at", "2026-08-07T16:00:00+08:00",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8",
		"--json",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		MutationID string `json:"mutation_id"`
		Record     struct {
			RecordID string `json:"record_id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.MutationID != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || decoded.Record.RecordID != "014b2680-df5b-4c8d-97ef-abde0a9746d6" {
		t.Fatalf("unexpected output: %#v", decoded)
	}
}

func TestRecordCreateFailurePrintsIdempotentRetryCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"code":"DATABASE_UNAVAILABLE","message":"Database unavailable","retryable":true}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{
		"--server", server.URL, "record", "create",
		"--energy", "520", "--protein", "28.5", "--carbohydrate", "62", "--fat", "18",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8",
	})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected create failure")
	}
	for _, required := range []string{
		"retryable=true",
		"--record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id 3fe5867d-f8cb-48d4-90b2-529a15531db8",
		"do not generate new IDs",
	} {
		if !strings.Contains(err.Error(), required) {
			t.Errorf("error does not contain %q: %v", required, err)
		}
	}
}

func TestRecordValidationFailureDoesNotRecommendExactRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"Invalid nutrition data","retryable":false,"field_violations":[{"field":"record.nutrients.energy","reason":"is too large"}]}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{
		"--server", server.URL, "record", "create",
		"--energy", "520", "--protein", "28.5", "--carbohydrate", "62", "--fat", "18",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8",
	})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if !strings.Contains(err.Error(), "never reuse a mutation_id with a different payload") {
		t.Fatalf("missing validation guidance: %v", err)
	}
	if strings.Contains(err.Error(), "Retry the exact same request") {
		t.Fatalf("validation error must not recommend an unchanged retry: %v", err)
	}
}

func TestRecordListJSONPreservesPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("limit") != "25" {
			t.Errorf("unexpected limit %q", request.URL.Query().Get("limit"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"records":[{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":"Pengnian lunch","version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}],"next_cursor":"next-page","has_more":true}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "record", "list",
		"--consumed-from", "2026-08-01T00:00:00+08:00",
		"--consumed-before", "2026-08-08T00:00:00+08:00",
		"--limit", "25", "--json",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var page struct {
		Records    []json.RawMessage `json:"records"`
		NextCursor *string           `json:"next_cursor"`
		HasMore    bool              `json:"has_more"`
	}
	if err := json.Unmarshal(output.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "next-page" {
		t.Fatalf("unexpected page output: %#v", page)
	}
}

func TestRecordListHumanOutputTeachesNextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"records":[{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":"Pengnian lunch","version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}],"next_cursor":"next-page","has_more":true}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "record", "list",
		"--consumed-from", "2026-08-01T00:00:00+08:00",
		"--consumed-before", "2026-08-08T00:00:00+08:00",
		"--limit", "25",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"records: 1",
		"record_id: 014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"energy_kcal: 520",
		"note: \"Pengnian lunch\"",
		"has_more: true",
		"next_cursor: next-page",
		"repeat this command with the same filters and --cursor \"next-page\"",
	} {
		if !strings.Contains(output.String(), required) {
			t.Errorf("output does not contain %q:\n%s", required, output.String())
		}
	}
}

func TestRecordListRejectsInvalidLimitBeforeAuthentication(t *testing.T) {
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{
		"record", "list",
		"--consumed-from", "2026-08-01T00:00:00+08:00",
		"--consumed-before", "2026-08-08T00:00:00+08:00",
		"--limit", "101",
	})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 100") {
		t.Fatalf("unexpected error: %v", err)
	}
}
