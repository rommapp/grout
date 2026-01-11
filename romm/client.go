package romm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sonh/qs"
)

const (
	DefaultClientTimeout = 30 * time.Second
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
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

func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) {
		c.username = username
		c.password = password
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
		WithBasicAuth(host.Username, host.Password),
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

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
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

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
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

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
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
