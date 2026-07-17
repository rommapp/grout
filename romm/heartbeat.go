package romm

import (
	"strconv"
	"strings"
)

type HeartbeatResponse struct {
	System struct {
		Version string `json:"VERSION"`
	} `json:"SYSTEM"`
}

func (c *Client) GetHeartbeat() (HeartbeatResponse, error) {
	var heartbeat HeartbeatResponse
	err := c.doRequest("GET", endpointHeartbeat, nil, nil, &heartbeat)
	return heartbeat, err
}

// SupportsDeviceAuth reports whether the server version indicates the
// device-authorization pairing flow is available (RomM 5.0+). Unparsable
// versions are treated as unsupported so older servers get the safe fallback.
func (h HeartbeatResponse) SupportsDeviceAuth() bool {
	v := strings.TrimPrefix(h.System.Version, "v")
	major := strings.SplitN(v, ".", 2)[0]
	// Strip any prerelease suffix on a bare major like "5-beta".
	major = strings.SplitN(major, "-", 2)[0]
	n, err := strconv.Atoi(major)
	if err != nil {
		return false
	}
	return n >= 5
}
