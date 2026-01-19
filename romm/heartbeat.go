package romm

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
