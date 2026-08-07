package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExchangePairingCodeUsesContractHeadersAndBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/base/v1/cli-pairing-codes:exchange" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatal("exchange must not send authorization")
		}
		if request.Header.Get("Sateia-Client-Type") != "CLI" || request.Header.Get("Sateia-Client-Version") != "test" {
			t.Fatalf("missing client metadata headers: %v", request.Header)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["device_code"] != "ABCD-EFGH" || body["device_name"] != "Pengnian Mac" {
			t.Fatalf("unexpected body: %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-exchange")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"token":"secret","token_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","created_at":"2026-08-07T08:00:00Z","expires_at":"2026-11-05T08:00:00Z"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/base", "", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	issued, metadata, err := client.ExchangePairingCode(context.Background(), "ABCD-EFGH", "Pengnian Mac")
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token != "secret" {
		t.Fatalf("unexpected token %q", issued.Token)
	}
	if metadata.RequestID != "request-exchange" {
		t.Fatalf("unexpected request ID %q", metadata.RequestID)
	}
}

func TestCreateNutritionRecordUsesBearerToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		var body struct {
			MutationID string `json:"mutation_id"`
			Record     struct {
				RecordID  string            `json:"record_id"`
				Nutrients map[string]string `json:"nutrients"`
				Offset    int               `json:"consumed_time_zone_offset_minutes"`
			} `json:"record"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.MutationID != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || body.Record.RecordID != "014b2680-df5b-4c8d-97ef-abde0a9746d6" {
			t.Fatalf("unexpected identifiers: %#v", body)
		}
		if body.Record.Offset != 480 || body.Record.Nutrients["energy"] != "100" {
			t.Fatalf("unexpected record payload: %#v", body.Record)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-create")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"100","protein":"2","carbohydrate":"20","fat":"1"},"source":"CLI","note":null,"version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	record, metadata, err := client.CreateNutritionRecord(context.Background(), CreateRecordRequest{
		MutationID: "3fe5867d-f8cb-48d4-90b2-529a15531db8",
		Record: NutritionRecordInput{
			RecordID:                      "014b2680-df5b-4c8d-97ef-abde0a9746d6",
			ConsumedTimeZoneOffsetMinutes: 480,
			Nutrients:                     map[string]string{"energy": "100", "protein": "2", "carbohydrate": "20", "fat": "1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Source != "CLI" || record.Version != 1 {
		t.Fatalf("unexpected record: %#v", record)
	}
	if metadata.RequestID != "request-create" {
		t.Fatalf("unexpected request ID %q", metadata.RequestID)
	}
}

func TestNormalizeBaseURLRejectsRemoteHTTP(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeBaseURL("http://example.com"); err == nil {
		t.Fatal("expected remote HTTP URL to be rejected")
	}
	if got, err := NormalizeBaseURL("http://127.0.0.1:8080/"); err != nil || got != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected loopback result %q, %v", got, err)
	}
}

func TestListNutritionRecordsUsesRequiredFiltersAndCursor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/nutrition-records" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		query := request.URL.Query()
		if query.Get("consumedFrom") != "2026-08-01T00:00:00+08:00" {
			t.Errorf("unexpected consumedFrom %q", query.Get("consumedFrom"))
		}
		if query.Get("consumedBefore") != "2026-08-08T00:00:00+08:00" {
			t.Errorf("unexpected consumedBefore %q", query.Get("consumedBefore"))
		}
		if query.Get("includeDeleted") != "true" || query.Get("cursor") != "opaque+/=" || query.Get("limit") != "25" {
			t.Errorf("unexpected query: %v", query)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-list")
		_, _ = writer.Write([]byte(`{"records":[{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":"Pengnian lunch","version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}],"next_cursor":"next+/=","has_more":true}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	from, _ := time.Parse(time.RFC3339, "2026-08-01T00:00:00+08:00")
	before, _ := time.Parse(time.RFC3339, "2026-08-08T00:00:00+08:00")
	page, metadata, err := client.ListNutritionRecords(context.Background(), ListNutritionRecordsOptions{
		ConsumedFrom: from, ConsumedBefore: before, IncludeDeleted: true, Cursor: "opaque+/=", Limit: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || page.Records[0].RecordID != "014b2680-df5b-4c8d-97ef-abde0a9746d6" {
		t.Fatalf("unexpected records: %#v", page.Records)
	}
	if !page.HasMore || page.NextCursor == nil || *page.NextCursor != "next+/=" {
		t.Fatalf("unexpected pagination: %#v", page)
	}
	if metadata.RequestID != "request-list" {
		t.Fatalf("unexpected request ID %q", metadata.RequestID)
	}
}

func TestListNutritionRecordsRejectsInvalidRangeBeforeRequest(t *testing.T) {
	t.Parallel()
	client, err := NewClient("https://example.com", "secret", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	boundary, _ := time.Parse(time.RFC3339, "2026-08-08T00:00:00Z")
	_, _, err = client.ListNutritionRecords(context.Background(), ListNutritionRecordsOptions{
		ConsumedFrom: boundary, ConsumedBefore: boundary, Limit: 50,
	})
	if err == nil {
		t.Fatal("expected equal time bounds to be rejected")
	}
}

func TestCheckAuthenticationUsesReadOnlyNutrientsEndpoint(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/nutrients" || request.URL.RawQuery != "" {
			t.Fatalf("unexpected authentication check %s %s", request.Method, request.URL.RequestURI())
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-auth-status")
		_, _ = writer.Write([]byte(`{"nutrients":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := client.CheckAuthentication(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metadata.RequestID != "request-auth-status" {
		t.Fatalf("unexpected request ID %q", metadata.RequestID)
	}
}

func TestAPIErrorIncludesResponseRequestID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-failure")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"code":"DATABASE_UNAVAILABLE","message":"Database unavailable","retryable":true}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.CreateNutritionRecord(context.Background(), CreateRecordRequest{})
	if err == nil {
		t.Fatal("expected API error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("unexpected error type %T", err)
	}
	if apiErr.RequestID != "request-failure" || !strings.Contains(apiErr.Error(), "request_id=request-failure") {
		t.Fatalf("unexpected API error: %#v, %v", apiErr, apiErr)
	}
}

func TestResponseRequestIDRejectsUnsafeValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "contains spaces", "contains:semicolon", strings.Repeat("a", 129)} {
		if got := responseRequestID(value); got != "" {
			t.Errorf("responseRequestID(%q) = %q", value, got)
		}
	}
	if got := responseRequestID("  request/ABC-123_test.json  "); got != "request/ABC-123_test.json" {
		t.Fatalf("unexpected normalized request ID %q", got)
	}
}
