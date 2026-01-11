package romm

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type Host struct {
	DisplayName string `json:"display_name,omitempty"`
	RootURI     string `json:"root_uri,omitempty"`
	Port        int    `json:"port,omitempty"`

	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

func (h Host) ToLoggable() map[string]any {
	temp := map[string]any{
		"display_name":         h.DisplayName,
		"root_uri":             h.RootURI,
		"port":                 h.Port,
		"username":             h.Username,
		"password":             strings.Repeat("*", len(h.Password)),
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

func (h Host) BasicAuthHeader() string {
	auth := h.Username + ":" + h.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}
