package romm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxErrorBody = 4096

type Options struct {
	BaseURL            string
	AuthHeader         string
	InsecureSkipVerify bool
	Timeout            time.Duration
}

type Client struct {
	baseURL    string
	authHeader string
	http       *http.Client
}

func New(o Options) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		baseURL:    strings.TrimSuffix(o.BaseURL, "/"),
		authHeader: o.AuthHeader,
		http:       &http.Client{Transport: transport, Timeout: o.Timeout},
	}
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return nil, &StatusError{Code: resp.StatusCode, Body: string(msg)}
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.send(ctx, http.MethodGet, path, query, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("romm: decoding %s: %w", path, err)
	}
	return nil
}
