package romm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const assetPrefix = "/assets/romm/resources"

type Body struct {
	io.ReadCloser
	Size int64
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (c *Client) WithContentTimeout(d time.Duration) *Client {
	h := *c.http
	h.Timeout = 0
	cp := *c
	cp.http = &h
	cp.contentTimeout = d
	cp.hasContentTimeout = true
	return &cp
}

func (c *Client) Download(ctx context.Context, path string, query url.Values) (*Body, error) {
	return c.stream(ctx, func(ctx context.Context) (*http.Response, error) {
		return c.send(ctx, http.MethodGet, path, query, nil, "")
	})
}

var ErrBadScheme = errors.New("romm: URL is not http or https")

func (c *Client) DownloadURL(ctx context.Context, rawURL string) (*Body, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: %q", ErrBadScheme, rawURL)
	}
	withAuth := c.sameOrigin(u)
	return c.stream(ctx, func(ctx context.Context) (*http.Response, error) {
		return c.doURL(ctx, http.MethodGet, rawURL, nil, "", withAuth)
	})
}

func (c *Client) stream(ctx context.Context, open func(context.Context) (*http.Response, error)) (*Body, error) {
	cancel := context.CancelFunc(func() {})
	if c.hasContentTimeout && c.contentTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.contentTimeout)
	}
	resp, err := open(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Body{ReadCloser: cancelBody{ReadCloser: resp.Body, cancel: cancel}, Size: resp.ContentLength}, nil
}

func RomContentPath(romID int, fileName string) string {
	return fmt.Sprintf("/api/roms/%d/content/%s", romID, url.PathEscape(fileName))
}

func FirmwareContentPath(firmwareID int, fileName string) string {
	return fmt.Sprintf("/api/firmware/%d/content/%s", firmwareID, url.PathEscape(fileName))
}

func (c *Client) AssetURL(p string) string {
	if p == "" {
		return ""
	}
	var u string
	switch {
	case strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://"):
		u = p
	case strings.Contains(p, assetPrefix):
		u = c.baseURL + "/" + strings.TrimPrefix(p, "/")
	default:
		u = c.baseURL + assetPrefix + "/" + strings.TrimPrefix(p, "/")
	}
	return strings.ReplaceAll(u, " ", "%20")
}
