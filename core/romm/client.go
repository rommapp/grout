package romm

import (
	"bytes"
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
	return c.do(ctx, method, path, query, body, contentType, true)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, withAuth bool) (*http.Response, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if withAuth && c.authHeader != "" {
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
	_, err := c.getJSONHeader(ctx, path, query, out)
	return err
}

func (c *Client) getJSONHeader(ctx context.Context, path string, query url.Values, out any) (http.Header, error) {
	resp, err := c.send(ctx, http.MethodGet, path, query, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("romm: decoding %s: %w", path, err)
	}
	return resp.Header, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	return c.sendJSON(ctx, http.MethodPost, path, in, out, true)
}

func (c *Client) putJSON(ctx context.Context, path string, in, out any) error {
	return c.sendJSON(ctx, http.MethodPut, path, in, out, true)
}

func (c *Client) postJSONNoAuth(ctx context.Context, path string, in, out any) error {
	return c.sendJSON(ctx, http.MethodPost, path, in, out, false)
}

func (c *Client) sendJSON(ctx context.Context, method, path string, in, out any, withAuth bool) error {
	var body io.Reader
	contentType := ""
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("romm: encoding %s: %w", path, err)
		}
		body = bytes.NewReader(b)
		contentType = "application/json"
	}
	resp, err := c.do(ctx, method, path, nil, body, contentType, withAuth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("romm: decoding %s: %w", path, err)
	}
	return nil
}
