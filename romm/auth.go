package romm

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type TokenExchangeRequest struct {
	Code string `json:"code"`
}

type TokenExchangeResponse struct {
	RawToken  string   `json:"raw_token"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
}

type CurrentUser struct {
	Username string `json:"username"`
}

// SyncRequiredScopes are the client-token scopes save sync needs end-to-end:
// reading/writing assets (saves) and reading/writing devices (negotiate, session
// complete, device registration, /downloaded). A token missing these will 403 on the
// sync endpoints.
var SyncRequiredScopes = []string{"assets.read", "assets.write", "devices.read", "devices.write"}

// MissingSyncScopes returns the SyncRequiredScopes not present in have. Advisory:
// RomM may model scopes more broadly, so treat a non-empty result as a likely (not
// certain) cause of sync permission failures.
func MissingSyncScopes(have []string) []string {
	present := make(map[string]bool, len(have))
	for _, s := range have {
		present[s] = true
	}
	var missing []string
	for _, s := range SyncRequiredScopes {
		if !present[s] {
			missing = append(missing, s)
		}
	}
	return missing
}

func (c *Client) ValidateConnection() error {
	req, err := http.NewRequest("GET", c.baseURL+endpointHeartbeat, nil)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to create validation request: %w", err))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to connect: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// logResponseDebug consumes the body, so it is in the log rather than on
	// the error. The status is what callers act on.
	logResponseDebug("ValidateConnection: unexpected status", resp)
	return statusError(resp.StatusCode, nil)
}

func ExchangeToken(baseURL string, code string, insecureSkipVerify bool) (*TokenExchangeResponse, error) {
	client := NewClient(baseURL, WithInsecureSkipVerify(insecureSkipVerify))
	var result TokenExchangeResponse
	err := client.doRequest("POST", endpointTokenExchange, nil, TokenExchangeRequest{Code: code}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ValidateToken() error {
	var platforms []Platform
	return c.doRequest("GET", endpointPlatforms, nil, nil, &platforms)
}

func (c *Client) GetCurrentUser() (CurrentUser, error) {
	var user CurrentUser
	err := c.doRequest("GET", endpointCurrentUser, nil, nil, &user)
	return user, err
}

func logResponseDebug(label string, resp *http.Response) {
	logger := slog.Default()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	logger.Debug(label,
		"status", resp.StatusCode,
		"url", resp.Request.URL.String(),
		"headers", headers,
		"body", string(body),
	)
}
