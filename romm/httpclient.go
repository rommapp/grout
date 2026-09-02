package romm

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPClient returns an http.Client for host, honouring its
// InsecureSkipVerify setting. Build every client here: artwork, cover art and
// firmware downloads bypass Client and must not lose that setting.
//
// A zero timeout falls back to DefaultClientTimeout, since a client with no
// timeout can hang a screen on a flaky connection.
func NewHTTPClient(host Host, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}

	client := &http.Client{Timeout: timeout}
	if host.InsecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return client
}
