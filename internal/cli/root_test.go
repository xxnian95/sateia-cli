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
	if got := output.String(); got != "Created record 014b2680-df5b-4c8d-97ef-abde0a9746d6 (version 1).\n" {
		t.Fatalf("unexpected output %q", got)
	}
}
