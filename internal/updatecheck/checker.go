package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xxnian95/sateia-cli/internal/config"
)

const (
	defaultEndpoint = "https://api.github.com/repos/xxnian95/sateia-cli/tags?per_page=100"
	cacheLifetime   = 24 * time.Hour
	responseLimit   = 1 << 20
)

var versionPattern = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

type Available struct {
	CurrentVersion string
	LatestVersion  string
}

type Checker struct {
	Endpoint   string
	HTTPClient *http.Client
	CachePath  string
	Now        func() time.Time
}

type cachedResult struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version,omitempty"`
}

type semanticVersion struct {
	major int
	minor int
	patch int
}

func New() Checker {
	return Checker{}
}

func (checker Checker) Check(ctx context.Context, currentVersion string) (*Available, error) {
	current, ok := parseVersion(currentVersion)
	if !ok {
		return nil, nil
	}
	now := time.Now
	if checker.Now != nil {
		now = checker.Now
	}
	currentTime := now()
	cachePath, err := checker.cachePath()
	if err != nil {
		return nil, err
	}
	if cached, cacheErr := loadCache(cachePath); cacheErr == nil && currentTime.Sub(cached.CheckedAt) >= 0 && currentTime.Sub(cached.CheckedAt) < cacheLifetime {
		return availableUpdate(currentVersion, current, cached.LatestVersion), nil
	}

	endpoint := checker.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	client := checker.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create tag request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "sateia-cli/"+currentVersion)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("check GitHub tags: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, responseLimit))
		return nil, fmt.Errorf("check GitHub tags: HTTP %d", response.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, responseLimit))
	if err := decoder.Decode(&tags); err != nil {
		return nil, fmt.Errorf("decode GitHub tags: %w", err)
	}
	latestName := ""
	latest := semanticVersion{}
	for _, tag := range tags {
		candidate, valid := parseVersion(tag.Name)
		if valid && (latestName == "" || candidate.greaterThan(latest)) {
			latestName = tag.Name
			latest = candidate
		}
	}
	_ = saveCache(cachePath, cachedResult{CheckedAt: currentTime, LatestVersion: latestName})
	return availableUpdate(currentVersion, current, latestName), nil
}

func (checker Checker) cachePath() (string, error) {
	if checker.CachePath != "" {
		return checker.CachePath, nil
	}
	configPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), "update-check.json"), nil
}

func availableUpdate(currentName string, current semanticVersion, latestName string) *Available {
	latest, ok := parseVersion(latestName)
	if !ok || !latest.greaterThan(current) {
		return nil
	}
	return &Available{CurrentVersion: currentName, LatestVersion: latestName}
}

func parseVersion(value string) (semanticVersion, bool) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return semanticVersion{}, false
	}
	parts := [3]int{}
	for index := range parts {
		parsed, err := strconv.Atoi(matches[index+1])
		if err != nil {
			return semanticVersion{}, false
		}
		parts[index] = parsed
	}
	return semanticVersion{major: parts[0], minor: parts[1], patch: parts[2]}, true
}

func (version semanticVersion) greaterThan(other semanticVersion) bool {
	if version.major != other.major {
		return version.major > other.major
	}
	if version.minor != other.minor {
		return version.minor > other.minor
	}
	return version.patch > other.patch
}

func loadCache(path string) (cachedResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cachedResult{}, err
	}
	var cached cachedResult
	if err := json.Unmarshal(data, &cached); err != nil {
		return cachedResult{}, err
	}
	if cached.CheckedAt.IsZero() {
		return cachedResult{}, errors.New("update cache is missing checked_at")
	}
	return cached, nil
}

func saveCache(path string, cached cachedResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".update-check-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
