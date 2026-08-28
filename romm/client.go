package romm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sonh/qs"
)

const (
	DefaultClientTimeout             = 30 * time.Second
	defaultSaveListMaxDecodedBytes   = int64(16 * 1024 * 1024)
	defaultSaveListMaxRecords        = 10000
	defaultSaveListMaxErrorBodyBytes = int64(64 * 1024)

	saveListFailureTransport    = "transport"
	saveListFailureHTTPStatus   = "http_status"
	saveListFailureInvalidQuery = "invalid_query"
	saveListFailureDecode       = "decode"
	saveListFailureByteLimit    = "byte_limit"
	saveListFailureRecordLimit  = "record_limit"
	saveListFailureTrailingData = "trailing_data"
)

type saveListLimits struct {
	maxDecodedBytes   int64
	maxRecords        int
	maxErrorBodyBytes int64
}

var defaultSaveListLimits = saveListLimits{
	maxDecodedBytes:   defaultSaveListMaxDecodedBytes,
	maxRecords:        defaultSaveListMaxRecords,
	maxErrorBodyBytes: defaultSaveListMaxErrorBodyBytes,
}

type SaveListStreamStats struct {
	RecordsSeen     int
	BytesRead       int64
	FilteredRecords int
}

type saveListRequestError struct {
	reason       string
	statusCode   int
	retainedBody []byte
	err          error
}

func (e *saveListRequestError) Error() string {
	if e.statusCode != 0 {
		return fmt.Sprintf("save list request failed: status %d, body: %q", e.statusCode, e.retainedBody)
	}
	return fmt.Sprintf("save list request failed (%s): %v", e.reason, e.err)
}

func (e *saveListRequestError) Unwrap() error { return e.err }

func SaveListFallbackReason(err error) string {
	var requestErr *saveListRequestError
	if errors.As(err, &requestErr) {
		return requestErr.reason
	}
	return "unknown"
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	authHeader string
}

type queryParam interface {
	Valid() bool
}

type ClientOption func(*Client)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

func WithAuthHeader(header string) ClientOption {
	return func(c *Client) {
		c.authHeader = header
	}
}

func WithInsecureSkipVerify(skip bool) ClientOption {
	return func(c *Client) {
		if skip {
			c.httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}
}

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: DefaultClientTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func NewClientFromHost(host Host, timeout ...time.Duration) *Client {
	opts := []ClientOption{
		WithAuthHeader(host.AuthHeader()),
		WithInsecureSkipVerify(host.InsecureSkipVerify),
	}
	if len(timeout) > 0 {
		opts = append(opts, WithTimeout(timeout[0]))
	}
	return NewClient(host.URL(), opts...)
}

func (c *Client) doRequest(method string, path string, queryParams queryParam, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	u := c.baseURL + path

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if queryParams != nil && queryParams.Valid() {
		values, err := qs.NewEncoder().Values(queryParams)
		if err == nil {
			req.URL.RawQuery = values.Encode()
		}
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// GetSavesForROMIDs performs the bounded save-list request used by remote-only
// discovery. Positive ROM IDs are copied into a private set before I/O. Matching
// records are returned only after a complete top-level array and clean EOF.
func (c *Client) GetSavesForROMIDs(query SaveQuery, romIDs []int) ([]Save, SaveListStreamStats, error) {
	return c.streamSavesForROMIDs(query, romIDs, defaultSaveListLimits)
}

func (c *Client) streamSavesForROMIDs(query SaveQuery, romIDs []int, limits saveListLimits) ([]Save, SaveListStreamStats, error) {
	wanted := make(map[int]struct{}, len(romIDs))
	for _, romID := range romIDs {
		if romID > 0 {
			wanted[romID] = struct{}{}
		}
	}
	if !query.Valid() {
		return nil, SaveListStreamStats{}, &saveListRequestError{
			reason: saveListFailureInvalidQuery,
			err:    fmt.Errorf("save list query is invalid"),
		}
	}
	if len(wanted) == 0 {
		return make([]Save, 0), SaveListStreamStats{}, nil
	}

	u := c.baseURL + endpointSaves
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, SaveListStreamStats{}, &saveListRequestError{reason: saveListFailureTransport, err: err}
	}
	values, encodeErr := qs.NewEncoder().Values(query)
	if encodeErr != nil {
		return nil, SaveListStreamStats{}, &saveListRequestError{reason: saveListFailureDecode, err: encodeErr}
	}
	req.URL.RawQuery = values.Encode()
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, SaveListStreamStats{}, &saveListRequestError{reason: saveListFailureTransport, err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, limits.maxErrorBodyBytes))
		return nil, SaveListStreamStats{BytesRead: int64(len(body))}, &saveListRequestError{
			reason:       saveListFailureHTTPStatus,
			statusCode:   resp.StatusCode,
			retainedBody: body,
			err:          readErr,
		}
	}
	if resp.StatusCode == http.StatusNoContent {
		return make([]Save, 0), SaveListStreamStats{}, nil
	}

	return decodeSaveList(resp.Body, wanted, limits)
}

func decodeSaveList(body io.Reader, wanted map[int]struct{}, limits saveListLimits) ([]Save, SaveListStreamStats, error) {
	reader := &countingReader{r: io.LimitReader(body, limits.maxDecodedBytes+1)}
	decoder := json.NewDecoder(reader)
	stats := SaveListStreamStats{}
	fail := func(reason string, err error) ([]Save, SaveListStreamStats, error) {
		stats.BytesRead = reader.n
		if reader.n > limits.maxDecodedBytes {
			reason = saveListFailureByteLimit
		}
		return nil, stats, &saveListRequestError{reason: reason, err: err}
	}

	token, err := decoder.Token()
	if err != nil {
		return fail(saveListFailureDecode, err)
	}
	opening, ok := token.(json.Delim)
	if !ok || opening != '[' {
		return fail(saveListFailureDecode, fmt.Errorf("save list must be a top-level array"))
	}

	retained := make([]Save, 0)
	for decoder.More() {
		var save Save
		if err := decoder.Decode(&save); err != nil {
			return fail(saveListFailureDecode, err)
		}
		stats.RecordsSeen++
		if stats.RecordsSeen > limits.maxRecords {
			return fail(saveListFailureRecordLimit, fmt.Errorf("save list record limit exceeded"))
		}
		if _, ok := wanted[save.RomID]; ok && save.RomID > 0 {
			retained = append(retained, save)
			stats.FilteredRecords++
		}
	}

	closing, err := decoder.Token()
	if err != nil {
		return fail(saveListFailureDecode, err)
	}
	if delim, ok := closing.(json.Delim); !ok || delim != ']' {
		return fail(saveListFailureDecode, fmt.Errorf("save list closing array token missing"))
	}

	var trailing json.RawMessage
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return fail(saveListFailureTrailingData, err)
	}
	stats.BytesRead = reader.n
	if reader.n > limits.maxDecodedBytes {
		return fail(saveListFailureByteLimit, fmt.Errorf("save list decoded byte limit exceeded"))
	}
	return retained, stats, nil
}

func (c *Client) doRequestRaw(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := c.baseURL + strings.ReplaceAll(path, " ", "%20")

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}

func (c *Client) doRequestRawWithQuery(method, path string, queryParams queryParam) ([]byte, error) {
	fullURL := c.baseURL + path

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if queryParams != nil && queryParams.Valid() {
		values, err := qs.NewEncoder().Values(queryParams)
		if err == nil {
			req.URL.RawQuery = values.Encode()
		}
	}

	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}

func (c *Client) doMultipartRequest(method, path string, queryParams queryParam, body io.Reader, contentType string, result interface{}) error {
	u := c.baseURL + path
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	if queryParams != nil && queryParams.Valid() {
		values, err := qs.NewEncoder().Values(queryParams)
		if err == nil {
			req.URL.RawQuery = values.Encode()
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return parseConflictError(bodyBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// parseConflictError attempts to parse a 409 response body into a ConflictError.
func parseConflictError(body []byte) error {
	// Try parsing as a direct ConflictError
	var conflict ConflictError
	if err := json.Unmarshal(body, &conflict); err == nil && conflict.ErrorType != "" {
		return &conflict
	}

	// Try parsing as {"detail": {...}} wrapper (FastAPI style)
	var wrapper struct {
		Detail ConflictError `json:"detail"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Detail.ErrorType != "" {
		return &wrapper.Detail
	}

	// Fallback: return a generic conflict error
	return &ConflictError{
		ErrorType: "conflict",
		Message:   string(body),
	}
}
