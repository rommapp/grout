//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// server is the RomM the tests run against.
type server struct {
	URL      string
	Username string
	Token    string
	// DeviceID is a device already known to the server, so a card can be
	// given one without walking the pairing flow.
	DeviceID string
}

var (
	once         sync.Once
	provisiond   *server
	provisionErr error
)

// romm returns a RomM holding a small library, provisioned on first use.
//
// One server is shared by every test in a run. Provisioning it costs a scan,
// and the tests only read from it, so doing that per test would buy nothing
// but minutes.
func romm(t *testing.T) *server {
	t.Helper()

	once.Do(func() { provisiond, provisionErr = provision() })
	if provisionErr != nil {
		t.Skipf("no RomM to test against: %v", provisionErr)
	}
	return provisiond
}

// provision creates the first user, scans the library and mints a token, by
// running the script that speaks RomM's API.
func provision() (*server, error) {
	url := os.Getenv("ROMM_URL")
	if url == "" {
		url = "http://romm:8080"
	}

	// A run against a handheld points at a RomM that is already set up, and a
	// device cannot reach the one inside the compose network anyway. Skipping
	// provisioning also keeps that run from needing python on the machine
	// driving it.
	if token := os.Getenv("ROMM_TOKEN"); token != "" {
		return &server{
			URL:      url,
			Username: envOr("ROMM_USERNAME", "e2e"),
			Token:    token,
			DeviceID: os.Getenv("ROMM_DEVICE_ID"),
		}, nil
	}

	cmd := exec.Command("python3", "/provision.py")
	cmd.Env = append(os.Environ(), "ROMM_URL="+url)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) < 2 {
		return nil, errNoToken
	}

	return &server{URL: url, Username: "e2e", Token: lines[0], DeviceID: lines[1]}, nil
}

type provisionFailure string

func (e provisionFailure) Error() string { return string(e) }

const errNoToken = provisionFailure("provisioning produced no token and device id")
