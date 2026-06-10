package romm

import (
	"fmt"
	"io"
	"net/http"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
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

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 500:
		logResponseDebug("ValidateConnection: server error", resp)
		return &AuthError{
			StatusCode: resp.StatusCode,
			Message:    "Server error",
			Err:        ErrServerError,
		}
	default:
		logResponseDebug("ValidateConnection: unexpected status", resp)
		return fmt.Errorf("heartbeat check failed with status: %d", resp.StatusCode)
	}
}

func (c *Client) Login(username, password string) error {
	req, err := http.NewRequest("POST", c.baseURL+endpointLogin, nil)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to create login request: %w", err))
	}

	req.SetBasicAuth(username, password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to login: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logResponseDebug("Login: failed", resp)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 401:
		return &AuthError{StatusCode: 401, Message: "Invalid username or password", Err: ErrUnauthorized}
	case resp.StatusCode == 403:
		return &AuthError{StatusCode: 403, Message: "Access forbidden", Err: ErrForbidden}
	case resp.StatusCode >= 500:
		return &AuthError{StatusCode: resp.StatusCode, Message: "Server error", Err: ErrServerError}
	default:
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}
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
	logger := gaba.GetLogger()
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
