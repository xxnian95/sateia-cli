package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

func TestDoctorReportsHealthyWithoutExposingCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/nutrients" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer doctor-secret" {
			t.Errorf("unexpected authorization header")
		}
		writer.Header().Set("X-Request-ID", "doctor-request")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "doctor-secret")
	checker := stubUpdateChecker{check: func(context.Context, string) (*updatecheck.Available, error) {
		return nil, nil
	}}
	var output bytes.Buffer
	command := newWithDependencies("v1.2.3", strings.NewReader(""), &output, &output, stubCredentialStore{}, checker)
	command.SetArgs([]string{"--server", server.URL, "doctor", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "doctor-secret") {
		t.Fatalf("doctor output exposed the credential: %s", output.String())
	}
	var report struct {
		Healthy bool          `json:"healthy"`
		Checks  []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Healthy {
		t.Fatalf("unexpected unhealthy report: %#v", report)
	}
	assertDoctorCheck(t, report.Checks, "authentication", "PASS")
	assertDoctorCheck(t, report.Checks, "skill", "WARN")
	assertDoctorCheck(t, report.Checks, "update", "PASS")
}

func TestDoctorReportsCredentialFailureWithoutFailingToRender(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "")
	t.Setenv("SATEIA_TOKEN_FILE", "")
	checker := stubUpdateChecker{check: func(context.Context, string) (*updatecheck.Available, error) {
		return nil, nil
	}}
	store := stubCredentialStore{get: func(string) (string, error) { return "", context.Canceled }}
	var output bytes.Buffer
	command := newWithDependencies("v1.2.3", strings.NewReader(""), &output, &output, store, checker)
	command.SetArgs([]string{"doctor", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var report doctorReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Healthy {
		t.Fatalf("unexpected healthy report: %#v", report)
	}
	assertDoctorCheck(t, report.Checks, "credential", "FAIL")
	assertDoctorCheck(t, report.Checks, "authentication", "SKIP")
}

func assertDoctorCheck(t *testing.T, checks []doctorCheck, name, status string) {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("check %s has status %s, want %s", name, check.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing doctor check %s", name)
}
