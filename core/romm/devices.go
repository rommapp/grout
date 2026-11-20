package romm

import (
	"context"
	"net/url"
)

type SyncMode string

const SyncModeAPI SyncMode = "api"

type DeviceRegistration struct {
	Name          string   `json:"name"`
	Platform      string   `json:"platform"`
	Client        string   `json:"client"`
	ClientVersion string   `json:"client_version"`
	SyncMode      SyncMode `json:"sync_mode"`
}

type DeviceRef struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
}

func (c *Client) RegisterDevice(ctx context.Context, r DeviceRegistration) (DeviceRef, error) {
	var out DeviceRef
	err := c.postJSON(ctx, "/api/devices", r, &out)
	return out, err
}

type DeviceUpdate struct {
	Name          string `json:"name,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
}

func (c *Client) UpdateDevice(ctx context.Context, deviceID string, u DeviceUpdate) error {
	return c.putJSON(ctx, "/api/devices/"+url.PathEscape(deviceID), u, nil)
}
