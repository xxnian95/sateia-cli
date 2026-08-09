package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/xxnian95/sateia-cli/internal/api"
)

func TestBuildCreateRequestDerivesOffsetAndGeneratesIdentifiers(t *testing.T) {
	t.Parallel()
	request, err := buildCreateRequest(createOptions{
		energy: "120.5", protein: "3", carbohydrate: "20", fat: "4.25",
		consumedAt: "2026-08-07T16:30:00+08:00",
	}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Record.ConsumedTimeZoneOffsetMinutes != 480 {
		t.Fatalf("unexpected offset %d", request.Record.ConsumedTimeZoneOffsetMinutes)
	}
	if !validUUID(request.Record.RecordID) || !validUUID(request.MutationID) {
		t.Fatalf("invalid generated identifiers: %#v", request)
	}
}

func TestBuildCreateRequestRejectsContractInvalidDecimal(t *testing.T) {
	t.Parallel()
	_, err := buildCreateRequest(createOptions{
		energy: "1e3", protein: "3", carbohydrate: "20", fat: "4",
	}, time.Now())
	if err == nil {
		t.Fatal("expected scientific notation to be rejected")
	}
}

func TestListErrorWithGuidanceDistinguishesRecovery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		required string
	}{
		{
			name:     "invalid cursor",
			err:      &api.APIError{StatusCode: 400, Code: "INVALID_CURSOR", Message: "The record cursor does not match the current filters"},
			required: "start again without --cursor",
		},
		{
			name:     "authentication",
			err:      &api.APIError{StatusCode: 401, Code: "UNAUTHENTICATED", Message: "Invalid token"},
			required: "sateia auth status",
		},
		{
			name:     "rate limit",
			err:      &api.APIError{StatusCode: 429, Code: "RATE_LIMIT_EXCEEDED", Message: "Too many requests", Retryable: true},
			required: "retry the same read-only command",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			message := listErrorWithGuidance(test.err).Error()
			if !strings.Contains(message, test.required) {
				t.Fatalf("error does not contain %q: %s", test.required, message)
			}
		})
	}
}
