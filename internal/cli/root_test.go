package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/credential"
	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

type stubUpdateChecker struct {
	check func(context.Context, string) (*updatecheck.Available, error)
}

func (checker stubUpdateChecker) Check(ctx context.Context, version string) (*updatecheck.Available, error) {
	return checker.check(ctx, version)
}

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
		writer.Header().Set("X-Request-ID", "request-create-human")
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
request_id: request-create-human
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
				"SATEIA_TOKEN_FILE",
				"_notice list",
				"sateia environment",
			},
		},
		{
			name: "login",
			args: []string{"auth", "login", "--help"},
			required: []string{
				"does not need to match",
				"current machine",
				"run hostname",
				"stable, recognizable",
				"sateia auth login --device-code ABCD-EFGH",
				"--token-file",
				"never printed",
			},
		},
		{
			name: "record create",
			args: []string{"record", "create", "--help"},
			required: []string{
				"non-negative decimal strings",
				"ambiguous network failure",
				"top-level request_id",
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
				"top-level request_id",
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
	normalizedOutput := strings.Join(strings.Fields(output.String()), " ")
	for _, required := range []string{
		"Precedence: SATEIA_TOKEN, SATEIA_TOKEN_FILE, then the system keyring",
		"SATEIA_TOKEN_FILE",
		"current machine",
		"never the token secret",
		"removes only a keyring credential",
		"both the printed --record-id and --mutation-id",
		"top-level _notice list",
		"SATEIA_NO_UPDATE_NOTIFIER",
		"top-level request_id",
		"structured logs",
		"audit events",
	} {
		if !strings.Contains(normalizedOutput, required) {
			t.Errorf("environment output does not contain %q", required)
		}
	}
}

type stubCredentialStore struct {
	get    func(string) (string, error)
	set    func(string, string) error
	delete func(string) error
}

func (store stubCredentialStore) Get(account string) (string, error) {
	if store.get == nil {
		panic("unexpected credential Get")
	}
	return store.get(account)
}

func (store stubCredentialStore) Set(account, token string) error {
	if store.set == nil {
		panic("unexpected credential Set")
	}
	return store.set(account, token)
}

func (store stubCredentialStore) Delete(account string) error {
	if store.delete == nil {
		panic("unexpected credential Delete")
	}
	return store.delete(account)
}

func TestAuthStatusExplainsUnavailableCredentialStore(t *testing.T) {
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	t.Setenv("SATEIA_TOKEN_FILE", "")
	store := stubCredentialStore{get: func(string) (string, error) {
		return "", credential.ErrUnavailable
	}}
	command := newWithStore("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, store)
	command.SetArgs([]string{"auth", "status"})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected unavailable credential store error")
	}
	for _, required := range []string{"system credential store is unavailable", "SATEIA_TOKEN", "SATEIA_TOKEN_FILE", "Secret Service"} {
		if !strings.Contains(err.Error(), required) {
			t.Errorf("error does not contain %q: %v", required, err)
		}
	}
}

func TestAuthStatusUsesTokenFileBeforeCredentialStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer file-secret" {
			t.Errorf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-auth-token-file")
		_, _ = writer.Write([]byte(`{"nutrients":[]}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SATEIA_TOKEN_FILE", path)
	var output bytes.Buffer
	command := newWithStore("test", strings.NewReader(""), &output, &output, stubCredentialStore{})
	command.SetArgs([]string{"--server", server.URL, "auth", "status"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "credential_source: token_file") {
		t.Fatalf("unexpected output: %s", output.String())
	}
	if !strings.Contains(output.String(), "request_id: request-auth-token-file") {
		t.Fatalf("missing request ID: %s", output.String())
	}
}

func TestEnvironmentTokenTakesPrecedenceOverTokenFile(t *testing.T) {
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	t.Setenv("SATEIA_TOKEN_FILE", filepath.Join(t.TempDir(), "missing-token"))
	app := application{store: stubCredentialStore{}}
	token, source, err := app.token("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if token != "environment-secret" || source != "environment" {
		t.Fatalf("unexpected credential %q from %q", token, source)
	}
}

func TestAuthLoginWritesTokenFileWithoutKeyring(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/cli-pairing-codes:exchange" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["device_name"] != "agent-host-01 (Sateia CLI)" {
			t.Fatalf("unexpected device name %q", body["device_name"])
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"token":"file-secret","token_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","created_at":"2026-08-07T08:00:00Z","expires_at":"2026-11-05T08:00:00Z"}`))
	}))
	defer server.Close()

	configDirectory := t.TempDir()
	t.Setenv("SATEIA_CONFIG_DIR", configDirectory)
	t.Setenv("SATEIA_TOKEN", "")
	t.Setenv("SATEIA_TOKEN_FILE", "")
	path := filepath.Join(t.TempDir(), "token")
	var output bytes.Buffer
	store := stubCredentialStore{
		get: func(string) (string, error) { return "", errors.New("keyring must not be accessed") },
		set: func(string, string) error { return errors.New("keyring must not be accessed") },
	}
	command := newWithStore("test", strings.NewReader(""), &output, &output, store)
	command.SetArgs([]string{
		"--server", server.URL, "auth", "login",
		"--device-code", "ABCD-EFGH",
		"--device-name", "agent-host-01 (Sateia CLI)",
		"--token-file", path,
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(token) != "file-secret" {
		t.Fatalf("unexpected token file contents %q", token)
	}
	if strings.Contains(output.String(), "file-secret") || !strings.Contains(output.String(), "credential_source: token_file") {
		t.Fatalf("unsafe or incomplete output: %s", output.String())
	}
	configData, err := os.ReadFile(filepath.Join(configDirectory, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configData), "file-secret") {
		t.Fatalf("config contains token secret: %s", configData)
	}
}

func TestAuthLoginRejectsExistingTokenFileBeforeExchange(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	t.Setenv("SATEIA_TOKEN_FILE", "")
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("existing-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := newWithStore("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, stubCredentialStore{})
	command.SetArgs([]string{
		"--server", server.URL, "auth", "login",
		"--device-code", "ABCD-EFGH", "--device-name", "agent-host-01",
		"--token-file", path,
	})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 0 {
		t.Fatalf("pairing code must not be consumed, requests=%d", requests)
	}
}

func TestAuthLogoutExplainsUnavailableCredentialStore(t *testing.T) {
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	t.Setenv("SATEIA_TOKEN_FILE", "")
	store := stubCredentialStore{delete: func(string) error {
		return credential.ErrUnavailable
	}}
	command := newWithStore("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, store)
	command.SetArgs([]string{"auth", "logout"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "Secret Service") || !strings.Contains(err.Error(), "SATEIA_TOKEN_FILE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordCreateJSONIncludesMutationIdentifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-create-json")
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
		RequestID  string `json:"request_id"`
		Record     struct {
			RecordID string `json:"record_id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.MutationID != "3fe5867d-f8cb-48d4-90b2-529a15531db8" || decoded.Record.RecordID != "014b2680-df5b-4c8d-97ef-abde0a9746d6" || decoded.RequestID != "request-create-json" {
		t.Fatalf("unexpected output: %#v", decoded)
	}
}

func TestJSONResponseIncludesUpdateInNoticeListAfterCommand(t *testing.T) {
	order := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		order = append(order, "command")
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"record":{"record_id":"014b2680-df5b-4c8d-97ef-abde0a9746d6","consumed_at":"2026-08-07T16:00:00+08:00","consumed_time_zone_offset_minutes":480,"nutrients":{"energy":"520","protein":"28.5","carbohydrate":"62","fat":"18"},"source":"CLI","note":null,"version":1,"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z","deleted_at":null}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	checker := stubUpdateChecker{check: func(_ context.Context, version string) (*updatecheck.Available, error) {
		order = append(order, "check")
		return &updatecheck.Available{CurrentVersion: version, LatestVersion: "v1.1.0"}, nil
	}}
	var output bytes.Buffer
	command := newWithDependencies("v1.0.0", strings.NewReader(""), &output, &output, credential.KeyringStore{}, checker)
	command.SetArgs([]string{
		"--server", server.URL, "record", "create",
		"--energy", "520", "--protein", "28.5", "--carbohydrate", "62", "--fat", "18",
		"--consumed-at", "2026-08-07T16:00:00+08:00",
		"--record-id", "014b2680-df5b-4c8d-97ef-abde0a9746d6",
		"--mutation-id", "3fe5867d-f8cb-48d4-90b2-529a15531db8", "--json",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "command,check" {
		t.Fatalf("update check order = %q", got)
	}
	var decoded struct {
		Notices []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Command string `json:"command"`
		} `json:"_notice"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Notices) != 1 || decoded.Notices[0].Code != "UPDATE_AVAILABLE" {
		t.Fatalf("unexpected notices: %#v", decoded.Notices)
	}
	if decoded.Notices[0].Command != "go install github.com/xxnian95/sateia-cli/cmd/sateia@v1.1.0" {
		t.Fatalf("unexpected update command: %#v", decoded.Notices[0])
	}
}

func TestJSONResponseAlwaysIncludesNoticeList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"records":[],"next_cursor":null,"has_more":false}`))
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
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["_notice"]) != "[]" {
		t.Fatalf("unexpected _notice: %s", decoded["_notice"])
	}
	if requestID, exists := decoded["request_id"]; !exists || string(requestID) != `""` {
		t.Fatalf("unexpected request_id: %s (exists=%t)", requestID, exists)
	}
}

func TestRecordCreateFailurePrintsIdempotentRetryCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-create-failure")
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
		"request_id=request-create-failure",
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
		writer.Header().Set("X-Request-ID", "request-list-json")
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
		RequestID  string            `json:"request_id"`
		Notices    []struct {
			Code string `json:"code"`
		} `json:"_notice"`
	}
	if err := json.Unmarshal(output.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "next-page" || page.RequestID != "request-list-json" {
		t.Fatalf("unexpected page output: %#v", page)
	}
	if len(page.Notices) != 1 || page.Notices[0].Code != "NEXT_PAGE" {
		t.Fatalf("unexpected page notices: %#v", page.Notices)
	}
}

func TestRecordListHumanOutputTeachesNextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-list-human")
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
		"request_id: request-list-human",
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
