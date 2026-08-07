package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckFindsLatestStableTagAndCachesIt(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("User-Agent") != "sateia-cli/v1.0.0" {
			t.Errorf("unexpected user agent %q", request.Header.Get("User-Agent"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"name":"v1.0.1"},{"name":"not-a-version"},{"name":"v1.2.0"}]`))
	}))
	defer server.Close()

	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	checker := Checker{
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
		CachePath:  filepath.Join(t.TempDir(), "update-check.json"),
		Now:        func() time.Time { return now },
	}

	available, err := checker.Check(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if available == nil || available.LatestVersion != "v1.2.0" || available.CurrentVersion != "v1.0.0" {
		t.Fatalf("unexpected update result: %#v", available)
	}

	server.Close()
	available, err = checker.Check(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if available == nil || available.LatestVersion != "v1.2.0" {
		t.Fatalf("unexpected cached result: %#v", available)
	}
	if requests != 1 {
		t.Fatalf("expected one network request, got %d", requests)
	}
	info, err := os.Stat(checker.CachePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected cache permissions: %o", info.Mode().Perm())
	}
}

func TestCheckReturnsNoUpdateForDevelopmentOrCurrentVersion(t *testing.T) {
	checker := Checker{}
	for _, version := range []string{"dev", "test"} {
		available, err := checker.Check(context.Background(), version)
		if err != nil || available != nil {
			t.Fatalf("version %q returned %#v, %v", version, available, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"name":"v1.2.0"},{"name":"v1.1.9"}]`))
	}))
	defer server.Close()
	checker = Checker{
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
		CachePath:  filepath.Join(t.TempDir(), "update-check.json"),
	}
	available, err := checker.Check(context.Background(), "v1.2.0")
	if err != nil || available != nil {
		t.Fatalf("current version returned %#v, %v", available, err)
	}
}
