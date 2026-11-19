package romm

import (
	"context"
	"errors"
)

var ErrServerTooOld = errors.New("this RomM does not support save channels; update RomM to use Grout v3")

type Heartbeat struct {
	System struct {
		Version string `json:"VERSION"`
	} `json:"SYSTEM"`
	SaveSync struct {
		Snapshots bool `json:"SNAPSHOTS"`
	} `json:"SAVE_SYNC"`
}

func (c *Client) Heartbeat(ctx context.Context) (Heartbeat, error) {
	var h Heartbeat
	err := c.getJSON(ctx, "/api/heartbeat", nil, &h)
	return h, err
}

func (h Heartbeat) RequireSnapshots() error {
	if !h.SaveSync.Snapshots {
		return ErrServerTooOld
	}
	return nil
}
