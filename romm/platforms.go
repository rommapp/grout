package romm

import (
	"fmt"
	"time"
)

type Platform struct {
	ID                  int        `json:"id"`
	Slug                string     `json:"slug"`
	FSSlug              string     `json:"fs_slug"`
	Name                string     `json:"name"`
	ApiName             string     `json:"-"` // Original name from API (not serialized, set by DisambiguatePlatformNames)
	CustomName          string     `json:"custom_name"`
	ShortName           string     `json:"short_name"`
	LogoPath            string     `json:"logo_path"`
	ROMCount            int        `json:"rom_count"`
	Firmware            []Firmware `json:"Firmware"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Manufacturer        string     `json:"manufacturer"`
	Generation          int        `json:"generation"`
	Type                string     `json:"type"`
	HasBIOS             bool       `json:"has_bios"`
	SupportedExtensions []string   `json:"supported_extensions"`
}

// DisplayName returns the platform's display name, preferring CustomName if set
func (p Platform) DisplayName() string {
	if p.CustomName != "" {
		return p.CustomName
	}
	return p.Name
}

type GetPlatformsQuery struct {
	UpdatedAfter string `qs:"updated_after,omitempty"` // ISO8601 timestamp with timezone
}

func (q GetPlatformsQuery) Valid() bool {
	return q.UpdatedAfter != ""
}

func (c *Client) GetPlatforms(query ...GetPlatformsQuery) ([]Platform, error) {
	var platforms []Platform

	var q GetPlatformsQuery
	if len(query) > 0 {
		q = query[0]
	}

	err := c.doRequest("GET", endpointPlatforms, q, nil, &platforms)
	return platforms, err
}

func (c *Client) GetPlatform(id int) (Platform, error) {
	var platform Platform
	path := fmt.Sprintf(endpointPlatformByID, id)
	err := c.doRequest("GET", path, nil, nil, &platform)
	return platform, err
}

func (c *Client) GetPlatformIdentifiers() ([]int, error) {
	var ids []int
	err := c.doRequest("GET", endpointPlatformIdentifiers, nil, nil, &ids)
	return ids, err
}

// DisambiguatePlatformNames sets each platform's Name field to its display name
// (preferring CustomName if set), and appends the FSSlug when multiple platforms
// share the same display name (e.g., "Arcade" becomes "Arcade (fbneo)")
// The original API name is preserved in ApiName before modification.
func DisambiguatePlatformNames(platforms []Platform) {
	// First pass: save original API name, set Name to DisplayName, and count occurrences
	nameCounts := make(map[string]int)
	for i := range platforms {
		platforms[i].ApiName = platforms[i].Name // Preserve original API name
		platforms[i].Name = platforms[i].DisplayName()
		nameCounts[platforms[i].Name]++
	}

	// Second pass: append FSSlug to names that appear more than once
	for i := range platforms {
		if nameCounts[platforms[i].Name] > 1 {
			platforms[i].Name = fmt.Sprintf("%s (%s)", platforms[i].Name, platforms[i].FSSlug)
		}
	}
}
