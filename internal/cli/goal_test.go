package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/api"
)

func TestGoalSetSendsAndPrintsNote(t *testing.T) {
	var received api.DailyGoalInput
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/v1/daily-goals/2026-08-16" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-goal")
		_, _ = writer.Write([]byte(`{"daily_goal":{"goal_date":"2026-08-16","nutrients":{"energy":"2200","protein":"140","carbohydrate":"240","fat":"70"},"note":"Training day"}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{
		"--server", server.URL, "goal", "set", "2026-08-16",
		"--energy", "2200", "--protein", "140", "--carbohydrate", "240", "--fat", "70",
		"--note", "Training day",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if received.Note == nil || *received.Note != "Training day" {
		t.Fatalf("request note = %#v", received.Note)
	}
	if !strings.Contains(output.String(), "note: Training day\n") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), dailyGoalNoteRecommendedNotice.Code) {
		t.Fatalf("output contains an unexpected note notice: %q", output.String())
	}
}

func TestGoalSetWithoutNoteReturnsRecommendationNotice(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		var received api.DailyGoalInput
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		if received.Note != nil {
			t.Fatalf("request note = %#v, want null", received.Note)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"daily_goal":{"goal_date":"2026-08-16","nutrients":{"energy":"2200","protein":"140","carbohydrate":"240","fat":"70"},"note":null}}`))
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	commonArgs := []string{
		"--server", server.URL, "goal", "set", "2026-08-16",
		"--energy", "2200", "--protein", "140", "--carbohydrate", "240", "--fat", "70",
	}

	var jsonOutput bytes.Buffer
	jsonCommand := New("test", strings.NewReader(""), &jsonOutput, &jsonOutput)
	jsonCommand.SetArgs(append(commonArgs, "--json"))
	if err := jsonCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Notices []notice `json:"_notice"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !containsNotice(payload.Notices, dailyGoalNoteRecommendedNotice.Code) {
		t.Fatalf("JSON notices = %#v", payload.Notices)
	}

	var humanOutput bytes.Buffer
	humanCommand := New("test", strings.NewReader(""), &humanOutput, &humanOutput)
	humanCommand.SetArgs(commonArgs)
	if err := humanCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(humanOutput.String(), "["+dailyGoalNoteRecommendedNotice.Code+"] "+dailyGoalNoteRecommendedNotice.Message) {
		t.Fatalf("human output = %q", humanOutput.String())
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func containsNotice(notices []notice, code string) bool {
	for _, item := range notices {
		if item.Code == code {
			return true
		}
	}
	return false
}

func TestGoalSetRejectsOversizedNoteBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()

	t.Setenv("SATEIA_CONFIG_DIR", t.TempDir())
	t.Setenv("SATEIA_TOKEN", "environment-secret")
	command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{
		"--server", server.URL, "goal", "set", "2026-08-16",
		"--energy", "2200", "--protein", "140", "--carbohydrate", "240", "--fat", "70",
		"--note", strings.Repeat("界", maxNoteLength+1),
	})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "at most 5,000 characters") {
		t.Fatalf("error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestCanonicalGoalDate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "leap day", value: "2028-02-29", valid: true},
		{name: "impossible date", value: "2026-02-30", valid: false},
		{name: "timestamp", value: "2026-08-09T00:00:00Z", valid: false},
		{name: "unpadded", value: "2026-8-9", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := canonicalGoalDate(test.value)
			if test.valid && (err != nil || result != test.value) {
				t.Fatalf("canonicalGoalDate(%q) = %q, %v", test.value, result, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("canonicalGoalDate(%q) unexpectedly succeeded", test.value)
			}
		})
	}
}
