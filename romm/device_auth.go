package romm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Device authorization flow (RomM 5.0+). Grout initiates pairing, shows a user
// code / QR, and polls for a token while the user approves in the RomM web UI.

// DeviceAuthScopes are the scopes grout requests when pairing: read scopes for
// browsing/downloading plus the SyncRequiredScopes for save sync.
var DeviceAuthScopes = []string{
	"me.read",
	"platforms.read",
	"roms.read",
	"collections.read",
	"firmware.read",
	"assets.read",
	"assets.write",
	"devices.read",
	"devices.write",
}

type DeviceAuthInitRequest struct {
	ClientDeviceIdentifier string   `json:"client_device_identifier"`
	Name                   string   `json:"name"`
	Client                 string   `json:"client"`
	Platform               string   `json:"platform,omitempty"`
	ClientVersion          string   `json:"client_version,omitempty"`
	RequestedScopes        []string `json:"requested_scopes"`
}

type DeviceAuthInitResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	// VerificationPath is a relative web-UI path (e.g. /pair/device); the
	// client joins it with the origin it was configured to reach.
	VerificationPath string `json:"verification_path"`
	// VerificationPathComplete is the same path with ?user_code= appended,
	// intended for QR display.
	VerificationPathComplete string `json:"verification_path_complete"`
	ExpiresIn                int    `json:"expires_in"`
	Interval                 int    `json:"interval"`
}

type DeviceAuthTokenResponse struct {
	AccessToken string   `json:"access_token"`
	DeviceID    string   `json:"device_id"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expires_at"`
}

// DeviceAuthPollState classifies one poll of the device-auth token endpoint.
type DeviceAuthPollState int

const (
	DeviceAuthPending DeviceAuthPollState = iota
	DeviceAuthSlowDown
	DeviceAuthDenied
	DeviceAuthExpired
	DeviceAuthSuccess
)

func (c *Client) InitDeviceAuth(req DeviceAuthInitRequest) (*DeviceAuthInitResponse, error) {
	var resp DeviceAuthInitResponse
	if err := c.doRequest("POST", endpointDeviceAuthInit, nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PollDeviceToken polls the token endpoint once. The server reports pending /
// slow_down / denied / expired as 400s with a detail string; those are flow
// states, not errors. Any other failure is returned as an error.
func (c *Client) PollDeviceToken(deviceCode string) (*DeviceAuthTokenResponse, DeviceAuthPollState, error) {
	payload, err := json.Marshal(map[string]string{"device_code": deviceCode})
	if err != nil {
		return nil, DeviceAuthPending, fmt.Errorf("failed to marshal poll request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+endpointDeviceAuthToken, bytes.NewReader(payload))
	if err != nil {
		return nil, DeviceAuthPending, fmt.Errorf("failed to create poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, DeviceAuthPending, ClassifyError(fmt.Errorf("failed to poll device token: %w", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, DeviceAuthPending, fmt.Errorf("failed to read poll response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var token DeviceAuthTokenResponse
		if err := json.Unmarshal(body, &token); err != nil {
			return nil, DeviceAuthPending, fmt.Errorf("failed to decode token response: %w", err)
		}
		return &token, DeviceAuthSuccess, nil
	}

	var detail struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(body, &detail)

	switch detail.Detail {
	case "authorization_pending":
		return nil, DeviceAuthPending, nil
	case "slow_down":
		return nil, DeviceAuthSlowDown, nil
	case "access_denied":
		return nil, DeviceAuthDenied, nil
	case "expired_token":
		return nil, DeviceAuthExpired, nil
	}

	return nil, DeviceAuthPending, fmt.Errorf("device token poll failed: status %d, body: %s", resp.StatusCode, string(body))
}
