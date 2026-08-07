package cli

import (
	"testing"
	"time"
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
