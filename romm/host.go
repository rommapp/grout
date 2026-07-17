package romm

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Host struct {
	DisplayName string `json:"display_name,omitempty"`
	RootURI     string `json:"root_uri,omitempty"`
	Port        int    `json:"port,omitempty"`

	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	Token              string `json:"token,omitempty"`
	TokenName          string `json:"token_name,omitempty"`
	TokenExpiresAt     string `json:"token_expires_at,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`

	// ClientDeviceID is the client-generated stable identifier sent as
	// client_device_identifier when initiating device-auth pairing.
	// DeviceID (below) remains the server-issued device UUID.
	ClientDeviceID string `json:"client_device_identifier,omitempty"`
	DeviceID       string `json:"device_id,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	// DeviceClientVersion is the grout version last reported to the server for this
	// device; used to refresh the server's record after an app upgrade.
	DeviceClientVersion string `json:"device_client_version,omitempty"`
}

func (h Host) HasTokenAuth() bool {
	return h.Token != ""
}

func (h Host) ToLoggable() map[string]any {
	temp := map[string]any{
		"display_name":         h.DisplayName,
		"root_uri":             h.RootURI,
		"port":                 h.Port,
		"username":             h.Username,
		"password":             strings.Repeat("*", len(h.Password)),
		"token":                strings.Repeat("*", len(h.Token)),
		"insecure_skip_verify": h.InsecureSkipVerify,
	}

	return temp
}

func (h Host) URL() string {
	if h.Port != 0 {
		return fmt.Sprintf("%s:%d", h.RootURI, h.Port)
	}
	return h.RootURI
}

func (h Host) AuthHeader() string {
	if h.Token != "" {
		return "Bearer " + h.Token
	}
	auth := h.Username + ":" + h.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// NewClientDeviceID returns a random stable identifier for this install, sent
// as client_device_identifier when initiating device-auth pairing.
func NewClientDeviceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively fatal elsewhere; fall back to a
		// timestamp so pairing still works.
		return fmt.Sprintf("grout-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
