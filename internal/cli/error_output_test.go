package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/api"
)

func TestWriteErrorUsesStructuredEnvelopeForJSONCommands(t *testing.T) {
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	args := []string{"record", "create", "--json"}
	command.SetArgs(args)
	executeErr := command.Execute()
	if executeErr == nil {
		t.Fatal("expected missing required flag error")
	}
	var output bytes.Buffer
	if err := WriteError(command, args, executeErr, &output); err != nil {
		t.Fatal(err)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Error.Code != "CLI_ERROR" || envelope.Error.Message == "" {
		t.Fatalf("unexpected error envelope: %#v", envelope)
	}
}

func TestStructuredAPIErrorPreservesOperationalFields(t *testing.T) {
	apiErr := &api.APIError{
		StatusCode: http.StatusConflict, Code: "VERSION_CONFLICT", Message: "record changed",
		Retryable: false, RequestID: "request-123",
		Violations: []api.FieldViolation{{Field: "expected_version", Reason: "does not match"}},
	}
	result := classifyStructuredError(fmt.Errorf("update failed: %w\nNext: read the record again", apiErr))
	if result.Type != "api" || result.Code != "VERSION_CONFLICT" || result.StatusCode != http.StatusConflict || result.RequestID != "request-123" {
		t.Fatalf("unexpected API error: %#v", result)
	}
	if result.Hint != "Next: read the record again" || len(result.Violations) != 1 {
		t.Fatalf("missing structured details: %#v", result)
	}
}

func TestWriteErrorKeepsHumanOutputByDefault(t *testing.T) {
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	var output bytes.Buffer
	if err := WriteError(command, []string{"environment"}, fmt.Errorf("example"), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Error: example\n" {
		t.Fatalf("unexpected human error: %q", output.String())
	}
}
