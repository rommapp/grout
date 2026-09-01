package romm

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPClient returns an http.Client for talking to host, honouring its
// InsecureSkipVerify setting.
//
// Not every request to a RomM instance goes through Client: artwork, cover art
// and firmware downloads fetch bytes directly. Those callers used to build a
// bare http.Client, which silently ignored the host's self-signed certificate
// setting -- so on a self-signed instance roms downloaded while artwork and
// BIOS failed. Build clients here instead, so there is one place where that
// decision is made.
//
// A zero timeout falls back to DefaultClientTimeout; a client with no timeout
// can hang a screen indefinitely on a flaky handheld connection.
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
