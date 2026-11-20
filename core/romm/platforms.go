package romm

import (
	"context"
	"time"
)

type Platform struct {
	ID         int       `json:"id"`
	Slug       string    `json:"slug"`
	FSSlug     string    `json:"fs_slug"`
	Name       string    `json:"name"`
	CustomName string    `json:"custom_name"`
	RomCount   int       `json:"rom_count"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (c *Client) Platforms(ctx context.Context) ([]Platform, error) {
	var out []Platform
	if err := c.getJSON(ctx, "/api/platforms", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PlatformIDs(ctx context.Context) ([]int, error) {
	var out []int
	if err := c.getJSON(ctx, "/api/platforms/identifiers", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
