package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpdateNutritionRecordPreservesPatchSemantics(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.Path != "/v1/nutrition-records/014b2680-df5b-4c8d-97ef-abde0a9746d6" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if string(body["mutation_id"]) != `"3fe5867d-f8cb-48d4-90b2-529a15531db8"` || string(body["expected_version"]) != "1" {
			t.Fatalf("unexpected mutation metadata: %#v", body)
		}
		if string(body["note"]) != "null" {
			t.Fatalf("clear-note must encode explicit null: %s", body["note"])
		}
		if _, exists := body["nutrients"]; exists {
			t.Fatal("omitted nutrients must remain absent from the patch")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-update")
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":null,"version":2,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-08T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	record, metadata, err := client.UpdateNutritionRecord(context.Background(), "014b2680-df5b-4c8d-97ef-abde0a9746d6", UpdateRecordRequest{
		MutationID: "3fe5867d-f8cb-48d4-90b2-529a15531db8", ExpectedVersion: 1,
		Note: OptionalString{Set: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Version != 2 || metadata.RequestID != "request-update" {
		t.Fatalf("unexpected response: %#v %#v", record, metadata)
	}
}

func TestUpdateNutritionRecordRequiresCompleteTimePair(t *testing.T) {
	t.Parallel()
	client, err := NewClient("https://example.com", "secret", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	consumedAt := time.Date(2026, 8, 7, 8, 0, 0, 0, time.UTC)
	_, _, err = client.UpdateNutritionRecord(context.Background(), "record", UpdateRecordRequest{
		MutationID: "mutation", ExpectedVersion: 1, ConsumedAt: &consumedAt,
	})
	if err == nil || err.Error() != "consumed time and time zone offset must be updated together" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteNutritionRecordUsesContractQueryAndReturnsTombstone(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/v1/nutrition-records/014b2680-df5b-4c8d-97ef-abde0a9746d6" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.URL.Query().Get("mutationId") != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || request.URL.Query().Get("expectedVersion") != "2" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-delete")
		_, _ = writer.Write([]byte(`{"tombstone":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","version":3,"deleted_at":"2026-08-08T09:00:00Z"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	tombstone, metadata, err := client.DeleteNutritionRecord(
		context.Background(),
		"014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"3fe5867d-f8cb-48d4-90b2-529a15531db8",
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if tombstone.Version != 3 || tombstone.DeletedAt.IsZero() || metadata.RequestID != "request-delete" {
		t.Fatalf("unexpected response: %#v %#v", tombstone, metadata)
	}
}
