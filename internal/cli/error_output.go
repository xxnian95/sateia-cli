package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xxnian95/sateia-cli/internal/api"
	"github.com/xxnian95/sateia-cli/internal/credential"
)

const structuredErrorAnnotation = "sateia.io/structured-error"

type errorEnvelope struct {
	OK    bool               `json:"ok"`
	Error structuredCLIError `json:"error"`
}

type structuredCLIError struct {
	Type       string               `json:"type"`
	Code       string               `json:"code"`
	Message    string               `json:"message"`
	Hint       string               `json:"hint,omitempty"`
	Retryable  bool                 `json:"retryable"`
	StatusCode int                  `json:"status_code,omitempty"`
	RequestID  string               `json:"request_id,omitempty"`
	Violations []api.FieldViolation `json:"violations,omitempty"`
}

// WriteError preserves the CLI's stderr/non-zero contract while making --json failures parseable.
func WriteError(root *cobra.Command, args []string, err error, output io.Writer) error {
	if !structuredErrorRequested(root, args) {
		_, writeErr := io.WriteString(output, "Error: "+err.Error()+"\n")
		return writeErr
	}
	envelope := errorEnvelope{OK: false, Error: classifyStructuredError(err)}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(envelope)
}

func markStructuredErrorPreference(root, command *cobra.Command) {
	requested := false
	if flag := command.Flags().Lookup("json"); flag != nil {
		value, err := command.Flags().GetBool("json")
		requested = err == nil && value
	}
	if root.Annotations == nil {
		root.Annotations = map[string]string{}
	}
	root.Annotations[structuredErrorAnnotation] = "false"
	if requested {
		root.Annotations[structuredErrorAnnotation] = "true"
	}
}

func structuredErrorRequested(root *cobra.Command, args []string) bool {
	if root.Annotations != nil {
		if value, exists := root.Annotations[structuredErrorAnnotation]; exists {
			return value == "true"
		}
	}
	// Parsing may fail before PersistentPreRunE records the selected output mode.
	for _, argument := range args {
		if argument == "--json" || argument == "--json=true" {
			return true
		}
	}
	return false
}

func classifyStructuredError(err error) structuredCLIError {
	message, hint := splitErrorGuidance(err.Error())
	result := structuredCLIError{
		Type: "cli", Code: "CLI_ERROR", Message: message, Hint: hint, Retryable: false,
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		result.Type = "api"
		result.Code = apiErr.Code
		if result.Code == "" {
			result.Code = "HTTP_ERROR"
		}
		result.Retryable = apiErr.Retryable
		result.StatusCode = apiErr.StatusCode
		result.RequestID = apiErr.RequestID
		result.Violations = apiErr.Violations
		return result
	}
	if errors.Is(err, context.Canceled) {
		result.Code = "INTERRUPTED"
	} else if errors.Is(err, credential.ErrNotFound) {
		result.Code = "CREDENTIAL_NOT_FOUND"
	} else if errors.Is(err, credential.ErrUnavailable) {
		result.Code = "CREDENTIAL_STORE_UNAVAILABLE"
	}
	return result
}

func splitErrorGuidance(message string) (string, string) {
	lines := strings.Split(strings.TrimSpace(message), "\n")
	if len(lines) == 1 {
		return lines[0], ""
	}
	return lines[0], strings.Join(lines[1:], "\n")
}
