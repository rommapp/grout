package romm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

var Scopes = []string{"me.read", "platforms.read", "roms.read", "collections.read", "firmware.read", "assets.read", "assets.write", "devices.read", "devices.write"}

type ClientToken struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
	DeviceID  string   `json:"device_id"`
	RawToken  string   `json:"raw_token"`
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (ClientToken, error) {
	var out ClientToken
	err := c.postJSONNoAuth(ctx, "/api/client-tokens/exchange", map[string]string{"code": code}, &out)
	return out, err
}

type DeviceInitRequest struct {
	ClientDeviceIdentifier string   `json:"client_device_identifier"`
	Name                   string   `json:"name"`
	Client                 string   `json:"client"`
	Platform               string   `json:"platform"`
	ClientVersion          string   `json:"client_version"`
	RequestedScopes        []string `json:"requested_scopes"`
}

type DeviceInit struct {
	DeviceCode               string `json:"device_code"`
	UserCode                 string `json:"user_code"`
	VerificationPath         string `json:"verification_path"`
	VerificationPathComplete string `json:"verification_path_complete"`
	ExpiresIn                int    `json:"expires_in"`
	Interval                 int    `json:"interval"`
}

func (c *Client) DeviceInitiate(ctx context.Context, r DeviceInitRequest) (DeviceInit, error) {
	var out DeviceInit
	err := c.postJSONNoAuth(ctx, "/api/auth/device/init", r, &out)
	return out, err
}

type DeviceToken struct {
	AccessToken string   `json:"access_token"`
	DeviceID    string   `json:"device_id"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expires_at"`
}

type PairState string

const (
	PairPending  PairState = "authorization_pending"
	PairSlowDown PairState = "slow_down"
	PairDenied   PairState = "access_denied"
	PairExpired  PairState = "expired_token"
)

type PairStateError struct{ State PairState }

func (e *PairStateError) Error() string { return "romm: device pairing " + string(e.State) }

func (c *Client) PollDeviceToken(ctx context.Context, deviceCode string) (DeviceToken, error) {
	var out DeviceToken
	err := c.postJSONNoAuth(ctx, "/api/auth/device/token", map[string]string{"device_code": deviceCode}, &out)
	var se *StatusError
	if errors.As(err, &se) && se.Code == http.StatusBadRequest {
		var d struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal([]byte(se.Body), &d) == nil {
			switch st := PairState(d.Detail); st {
			case PairPending, PairSlowDown, PairDenied, PairExpired:
				return DeviceToken{}, &PairStateError{State: st}
			}
		}
	}
	return out, err
}

type User struct {
	Username string `json:"username"`
}

func (c *Client) Me(ctx context.Context) (User, error) {
	var out User
	err := c.getJSON(ctx, "/api/users/me", nil, &out)
	return out, err
}
