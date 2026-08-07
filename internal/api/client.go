package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const clientType = "CLI"

type Client struct {
	baseURL    string
	token      string
	version    string
	httpClient *http.Client
}

type IssuedToken struct {
	Token     string    `json:"token"`
	TokenID   string    `json:"token_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type NutritionRecordInput struct {
	RecordID                      string            `json:"record_id"`
	ConsumedAt                    time.Time         `json:"consumed_at"`
	ConsumedTimeZoneOffsetMinutes int               `json:"consumed_time_zone_offset_minutes"`
	Nutrients                     map[string]string `json:"nutrients"`
	Note                          *string           `json:"note,omitempty"`
}

type CreateRecordRequest struct {
	MutationID string               `json:"mutation_id"`
	Record     NutritionRecordInput `json:"record"`
}

type NutritionRecord struct {
	RecordID                      string            `json:"record_id"`
	ConsumedAt                    time.Time         `json:"consumed_at"`
	ConsumedTimeZoneOffsetMinutes int               `json:"consumed_time_zone_offset_minutes"`
	Nutrients                     map[string]string `json:"nutrients"`
	Source                        string            `json:"source"`
	Note                          *string           `json:"note"`
	Version                       int64             `json:"version"`
	CreatedAt                     time.Time         `json:"created_at"`
	UpdatedAt                     time.Time         `json:"updated_at"`
	DeletedAt                     *time.Time        `json:"deleted_at"`
}

type ListNutritionRecordsOptions struct {
	ConsumedFrom   time.Time
	ConsumedBefore time.Time
	IncludeDeleted bool
	Cursor         string
	Limit          int
}

type NutritionRecordPage struct {
	Records    []NutritionRecord `json:"records"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
	Violations []FieldViolation
}

type FieldViolation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (err *APIError) Error() string {
	if err.Code == "" {
		return fmt.Sprintf("server returned HTTP %d", err.StatusCode)
	}
	message := fmt.Sprintf("%s: %s", err.Code, err.Message)
	for _, violation := range err.Violations {
		message += fmt.Sprintf("; %s %s", violation.Field, violation.Reason)
	}
	if err.Retryable {
		message += "; retryable=true"
	}
	return message
}

func NewClient(baseURL, token, version string, httpClient *http.Client) (*Client, error) {
	normalized, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(version) == "" {
		return nil, errors.New("client version is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: normalized, token: token, version: version, httpClient: httpClient}, nil
}

func NormalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse server URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("server URL must include a scheme and host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must not contain credentials, a query, or a fragment")
	}
	host := parsed.Hostname()
	isLoopback := strings.EqualFold(host, "localhost")
	if address := net.ParseIP(host); address != nil && address.IsLoopback() {
		isLoopback = true
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopback) {
		return "", errors.New("server URL must use HTTPS; HTTP is allowed only for loopback development")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (client *Client) ExchangePairingCode(ctx context.Context, deviceCode, deviceName string) (IssuedToken, error) {
	body := struct {
		DeviceCode string `json:"device_code"`
		DeviceName string `json:"device_name"`
	}{DeviceCode: deviceCode, DeviceName: deviceName}
	var response IssuedToken
	if err := client.do(ctx, http.MethodPost, "/v1/cli-pairing-codes:exchange", body, false, http.StatusCreated, &response); err != nil {
		return IssuedToken{}, err
	}
	if response.Token == "" || response.TokenID == "" || response.ExpiresAt.IsZero() {
		return IssuedToken{}, errors.New("server returned an incomplete CLI token")
	}
	return response, nil
}

func (client *Client) CreateNutritionRecord(ctx context.Context, request CreateRecordRequest) (NutritionRecord, error) {
	var response struct {
		Record NutritionRecord `json:"record"`
	}
	if err := client.do(ctx, http.MethodPost, "/v1/nutrition-records", request, true, http.StatusCreated, &response); err != nil {
		return NutritionRecord{}, err
	}
	if response.Record.RecordID == "" || response.Record.Version < 1 {
		return NutritionRecord{}, errors.New("server returned an incomplete nutrition record")
	}
	return response.Record, nil
}

func (client *Client) ListNutritionRecords(ctx context.Context, options ListNutritionRecordsOptions) (NutritionRecordPage, error) {
	if options.ConsumedFrom.IsZero() {
		return NutritionRecordPage{}, errors.New("consumed-from is required")
	}
	if options.ConsumedBefore.IsZero() {
		return NutritionRecordPage{}, errors.New("consumed-before is required")
	}
	if !options.ConsumedFrom.Before(options.ConsumedBefore) {
		return NutritionRecordPage{}, errors.New("consumed-from must be earlier than consumed-before")
	}
	if options.Limit < 1 || options.Limit > 100 {
		return NutritionRecordPage{}, errors.New("limit must be between 1 and 100")
	}

	query := url.Values{}
	query.Set("consumedFrom", options.ConsumedFrom.Format(time.RFC3339Nano))
	query.Set("consumedBefore", options.ConsumedBefore.Format(time.RFC3339Nano))
	query.Set("limit", strconv.Itoa(options.Limit))
	if options.IncludeDeleted {
		query.Set("includeDeleted", "true")
	}
	if options.Cursor != "" {
		query.Set("cursor", options.Cursor)
	}

	var response struct {
		Records    *[]NutritionRecord `json:"records"`
		NextCursor *string            `json:"next_cursor"`
		HasMore    *bool              `json:"has_more"`
	}
	path := "/v1/nutrition-records?" + query.Encode()
	if err := client.do(ctx, http.MethodGet, path, nil, true, http.StatusOK, &response); err != nil {
		return NutritionRecordPage{}, err
	}
	if response.Records == nil || response.HasMore == nil {
		return NutritionRecordPage{}, errors.New("server returned an incomplete nutrition record page")
	}
	if *response.HasMore && (response.NextCursor == nil || *response.NextCursor == "") {
		return NutritionRecordPage{}, errors.New("server returned has_more without a next_cursor")
	}
	return NutritionRecordPage{
		Records:    *response.Records,
		NextCursor: response.NextCursor,
		HasMore:    *response.HasMore,
	}, nil
}

func (client *Client) CheckAuthentication(ctx context.Context) error {
	return client.do(ctx, http.MethodGet, "/v1/nutrients", nil, true, http.StatusOK, nil)
}

func (client *Client) do(ctx context.Context, method, path string, body any, authenticated bool, expectedStatus int, output any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Sateia-Client-Type", clientType)
	request.Header.Set("Sateia-Client-Version", client.version)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		if client.token == "" {
			return errors.New("authentication token is required")
		}
		request.Header.Set("Authorization", "Bearer "+client.token)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 2<<20)
	if response.StatusCode != expectedStatus {
		return decodeAPIError(response.StatusCode, limited)
	}
	if output == nil {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func decodeAPIError(statusCode int, body io.Reader) error {
	var envelope struct {
		Error struct {
			Code            string           `json:"code"`
			Message         string           `json:"message"`
			Retryable       bool             `json:"retryable"`
			FieldViolations []FieldViolation `json:"field_violations"`
		} `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&envelope); err != nil {
		return &APIError{StatusCode: statusCode}
	}
	return &APIError{
		StatusCode: statusCode,
		Code:       envelope.Error.Code,
		Message:    envelope.Error.Message,
		Retryable:  envelope.Error.Retryable,
		Violations: envelope.Error.FieldViolations,
	}
}
