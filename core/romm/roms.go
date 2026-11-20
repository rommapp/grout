package romm

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RomMetadata struct {
	Genres           []string `json:"genres"`
	Franchises       []string `json:"franchises"`
	Companies        []string `json:"companies"`
	GameModes        []string `json:"game_modes"`
	AgeRatings       []string `json:"age_ratings"`
	PlayerCount      string   `json:"player_count"`
	FirstReleaseDate *int64   `json:"first_release_date"`
	AverageRating    *float64 `json:"average_rating"`
}

type RomFile struct {
	ID            int    `json:"id"`
	FileName      string `json:"file_name"`
	FilePath      string `json:"file_path"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	Category      string `json:"category"`
	CRCHash       string `json:"crc_hash"`
	MD5Hash       string `json:"md5_hash"`
	SHA1Hash      string `json:"sha1_hash"`
}

type Rom struct {
	ID                  int         `json:"id"`
	PlatformID          int         `json:"platform_id"`
	Name                string      `json:"name"`
	NameSortKey         string      `json:"name_sort_key"`
	Summary             string      `json:"summary"`
	FSName              string      `json:"fs_name"`
	FSNameNoExt         string      `json:"fs_name_no_ext"`
	FSExtension         string      `json:"fs_extension"`
	FSSizeBytes         int64       `json:"fs_size_bytes"`
	Regions             []string    `json:"regions"`
	Languages           []string    `json:"languages"`
	Tags                []string    `json:"tags"`
	HasManual           bool        `json:"has_manual"`
	HasSimpleSingleFile bool        `json:"has_simple_single_file"`
	HasNestedSingleFile bool        `json:"has_nested_single_file"`
	HasMultipleFiles    bool        `json:"has_multiple_files"`
	MissingFromFS       bool        `json:"missing_from_fs"`
	PathCoverSmall      string      `json:"path_cover_small"`
	PathCoverLarge      string      `json:"path_cover_large"`
	URLCover            string      `json:"url_cover"`
	Metadatum           RomMetadata `json:"metadatum"`
	Files               []RomFile   `json:"files"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

func (r Rom) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.FSNameNoExt
}

func extraCategory(category string) bool {
	switch strings.ToLower(category) {
	case "manual", "walkthrough", "soundtrack", "screenshot", "cheat", "patch", "update", "dlc":
		return true
	}
	return false
}

func (r Rom) PlayableFiles() []RomFile {
	var playable []RomFile
	for _, f := range r.Files {
		if !extraCategory(f.Category) {
			playable = append(playable, f)
		}
	}
	if len(playable) == 0 {
		return r.Files
	}
	return playable
}

type RomPage struct {
	Items  []Rom `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`

	ServerTime time.Time `json:"-"`
}

type RomsQuery struct {
	PlatformID   int
	UpdatedAfter string
	Limit        int
	Offset       int
	WithTotal    bool
}

func (c *Client) Roms(ctx context.Context, q RomsQuery) (RomPage, error) {
	v := url.Values{}
	v.Set("platform_ids", strconv.Itoa(q.PlatformID))
	v.Set("limit", strconv.Itoa(q.Limit))
	v.Set("offset", strconv.Itoa(q.Offset))
	v.Set("order_by", "id")
	v.Set("order_dir", "asc")
	v.Set("with_files", "true")
	v.Set("with_rom_id_index", "false")
	v.Set("with_char_index", "false")
	v.Set("with_filter_values", "false")
	v.Set("with_total", strconv.FormatBool(q.WithTotal))
	v.Set("with_provider_metadata", "false")
	if q.UpdatedAfter != "" {
		v.Set("updated_after", q.UpdatedAfter)
	}
	var page RomPage
	h, err := c.getJSONHeader(ctx, "/api/roms", v, &page)
	if err != nil {
		return RomPage{}, err
	}
	if t, err := http.ParseTime(h.Get("Date")); err == nil {
		page.ServerTime = t.UTC()
	}
	return page, nil
}

func (c *Client) RomIDs(ctx context.Context) ([]int, error) {
	var out []int
	if err := c.getJSON(ctx, "/api/roms/identifiers", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
