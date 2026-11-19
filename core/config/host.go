package config

import (
	"fmt"
	"strings"
)

type Host struct {
	DisplayName         string `json:"display_name,omitempty"`
	RootURI             string `json:"root_uri,omitempty"`
	Port                int    `json:"port,omitempty"`
	Username            string `json:"username,omitempty"`
	Token               string `json:"token,omitempty"`
	TokenName           string `json:"token_name,omitempty"`
	TokenExpiresAt      string `json:"token_expires_at,omitempty"`
	InsecureSkipVerify  bool   `json:"insecure_skip_verify,omitempty"`
	ClientDeviceID      string `json:"client_device_identifier,omitempty"`
	DeviceID            string `json:"device_id,omitempty"`
	DeviceName          string `json:"device_name,omitempty"`
	DeviceClientVersion string `json:"device_client_version,omitempty"`
}

func (h Host) URL() string {
	root := strings.TrimSuffix(h.RootURI, "/")
	if h.Port != 0 {
		return fmt.Sprintf("%s:%d", root, h.Port)
	}
	return root
}

func (h Host) AuthHeader() string {
	if h.Token == "" {
		return ""
	}
	return "Bearer " + h.Token
}
