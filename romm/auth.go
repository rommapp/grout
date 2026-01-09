package romm

import (
	"errors"
	"fmt"
	"net/http"
)

func (c *Client) ValidateConnection() error {
	req, err := http.NewRequest("GET", c.baseURL+endpointHeartbeat, nil)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to create validation request: %w", err))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		classifiedErr := ClassifyError(fmt.Errorf("failed to connect: %w", err))

		shouldTryProtocolSwitch := !errors.Is(classifiedErr, ErrTimeout) &&
			!errors.Is(classifiedErr, ErrConnectionRefused) &&
			!errors.Is(classifiedErr, ErrInvalidHostname)

		if shouldTryProtocolSwitch {
			if protocolErr := c.tryAlternateProtocol(req.URL.Scheme, func(r *http.Response) bool {
				return r.StatusCode >= 200 && r.StatusCode < 300
			}); protocolErr != nil {
				return protocolErr
			}
		}

		return classifiedErr
	}
	defer resp.Body.Close()

	if req.URL.Scheme != resp.Request.URL.Scheme {
		return &ProtocolError{
			RequestedProtocol: req.URL.Scheme,
			CorrectProtocol:   resp.Request.URL.Scheme,
			Err:               ErrWrongProtocol,
		}
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 500:
		return &AuthError{
			StatusCode: resp.StatusCode,
			Message:    "Server error",
			Err:        ErrServerError,
		}
	default:
		if protocolErr := c.tryAlternateProtocol(req.URL.Scheme, func(r *http.Response) bool {
			return r.StatusCode >= 200 && r.StatusCode < 300
		}); protocolErr != nil {
			return protocolErr
		}
		return fmt.Errorf("heartbeat check failed with status: %d", resp.StatusCode)
	}
}

func switchProtocol(baseURL string) string {
	if len(baseURL) > 8 && baseURL[:8] == "https://" {
		return "http://" + baseURL[8:]
	}
	if len(baseURL) > 7 && baseURL[:7] == "http://" {
		return "https://" + baseURL[7:]
	}
	return baseURL
}

// tryAlternateProtocol tests if the alternate protocol works and returns a ProtocolError if it does.
// The isSuccess function determines if the response indicates the alternate protocol is working.
func (c *Client) tryAlternateProtocol(originalScheme string, isSuccess func(resp *http.Response) bool) *ProtocolError {
	switchedURL := switchProtocol(c.baseURL)
	if switchedURL == c.baseURL {
		return nil
	}

	testReq, err := http.NewRequest("GET", switchedURL+endpointHeartbeat, nil)
	if err != nil {
		return nil
	}

	testResp, err := c.httpClient.Do(testReq)
	if err != nil {
		return nil
	}
	defer testResp.Body.Close()

	if isSuccess(testResp) {
		return &ProtocolError{
			RequestedProtocol: originalScheme,
			CorrectProtocol:   testReq.URL.Scheme,
			Err:               ErrWrongProtocol,
		}
	}
	return nil
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

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 401:
		return &AuthError{
			StatusCode: 401,
			Message:    "Invalid username or password",
			Err:        ErrUnauthorized,
		}
	case resp.StatusCode == 403:
		return &AuthError{
			StatusCode: 403,
			Message:    "Access forbidden",
			Err:        ErrForbidden,
		}
	case resp.StatusCode >= 500:
		return &AuthError{
			StatusCode: resp.StatusCode,
			Message:    "Server error",
			Err:        ErrServerError,
		}
	case resp.StatusCode == 405:
		if switchedURL := switchProtocol(c.baseURL); switchedURL != c.baseURL {
			testClient := NewClient(switchedURL, WithTimeout(c.httpClient.Timeout))
			if testReq, testErr := http.NewRequest("POST", switchedURL+endpointLogin, nil); testErr == nil {
				testReq.SetBasicAuth(username, password)
				if testResp, testRespErr := testClient.httpClient.Do(testReq); testRespErr == nil {
					defer testResp.Body.Close()
					if testResp.StatusCode != 405 && testResp.StatusCode < 500 {
						return &ProtocolError{
							RequestedProtocol: req.URL.Scheme,
							CorrectProtocol:   testReq.URL.Scheme,
							Err:               ErrWrongProtocol,
						}
					}
				}
			}
		}
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	default:
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}
}
