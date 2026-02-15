package romm

import (
	"fmt"
	"time"
)

type Device struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Platform      string    `json:"platform"`
	Client        string    `json:"client"`
	ClientVersion string    `json:"client_version"`
	IPAddress     string    `json:"ip_address"`
	MACAddress    string    `json:"mac_address"`
	Hostname      string    `json:"hostname"`
	SyncMode      string    `json:"sync_mode"`
	SyncEnabled   bool      `json:"sync_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RegisterDeviceRequest struct {
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Client        string `json:"client"`
	ClientVersion string `json:"client_version"`
	IPAddress     string `json:"ip_address,omitempty"`
	MACAddress    string `json:"mac_address,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
}

type UpdateDeviceRequest struct {
	Name          string `json:"name,omitempty"`
	Platform      string `json:"platform,omitempty"`
	Client        string `json:"client,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
	IPAddress     string `json:"ip_address,omitempty"`
	MACAddress    string `json:"mac_address,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	SyncMode      string `json:"sync_mode,omitempty"`
	SyncEnabled   *bool  `json:"sync_enabled,omitempty"`
}

func (c *Client) RegisterDevice(req RegisterDeviceRequest) (Device, error) {
	var resp struct {
		DeviceID  string    `json:"device_id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	}
	err := c.doRequest("POST", endpointDevices, nil, req, &resp)
	if err != nil {
		return Device{}, err
	}
	return Device{
		ID:        resp.DeviceID,
		Name:      resp.Name,
		CreatedAt: resp.CreatedAt,
	}, nil
}

func (c *Client) GetDevices() ([]Device, error) {
	var devices []Device
	err := c.doRequest("GET", endpointDevices, nil, nil, &devices)
	return devices, err
}

func (c *Client) GetDevice(deviceID string) (Device, error) {
	var device Device
	err := c.doRequest("GET", fmt.Sprintf(endpointDeviceByID, deviceID), nil, nil, &device)
	return device, err
}

func (c *Client) UpdateDevice(deviceID string, req UpdateDeviceRequest) (Device, error) {
	var device Device
	err := c.doRequest("PUT", fmt.Sprintf(endpointDeviceByID, deviceID), nil, req, &device)
	return device, err
}

func (c *Client) DeleteDevice(deviceID string) error {
	return c.doRequest("DELETE", fmt.Sprintf(endpointDeviceByID, deviceID), nil, nil, nil)
}
