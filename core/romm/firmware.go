package romm

import (
	"context"
	"net/url"
	"strconv"
)

type Firmware struct {
	ID         int    `json:"id"`
	PlatformID int    `json:"platform_id"`
	FileName   string `json:"file_name"`
	FilePath   string `json:"file_path"`
	SizeBytes  int64  `json:"file_size_bytes"`
	IsVerified bool   `json:"is_verified"`
	MD5        string `json:"md5_hash"`
	Missing    bool   `json:"missing_from_fs"`
}

func (c *Client) PlatformFirmware(ctx context.Context, platformID int) ([]Firmware, error) {
	var out []Firmware
	q := url.Values{"platform_id": {strconv.Itoa(platformID)}}
	if err := c.getJSON(ctx, "/api/firmware", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
