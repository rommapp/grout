package romm

import (
	"bytes"
	"fmt"
	"grout/models"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"time"
)

type PaginatedRoms struct {
	Items  []Rom `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type Rom struct {
	ID                  int    `json:"id,omitempty"`
	GameListID          any    `json:"gamelist_id,omitempty"`
	PlatformID          int    `json:"platform_id,omitempty"`
	PlatformSlug        string `json:"platform_slug,omitempty"`
	PlatformCustomName  string `json:"platform_custom_name,omitempty"`
	PlatformDisplayName string `json:"platform_display_name,omitempty"`
	FsName              string `json:"fs_name,omitempty"`
	FsNameNoTags        string `json:"fs_name_no_tags,omitempty"`
	FsNameNoExt         string `json:"fs_name_no_ext,omitempty"`
	FsExtension         string `json:"fs_extension,omitempty"`
	FsPath              string `json:"fs_path,omitempty"`
	FsSizeBytes         int    `json:"fs_size_bytes,omitempty"`
	Name                string `json:"name,omitempty"`
	DisplayName         string
	ListName            string
	Slug                string       `json:"slug,omitempty"`
	Summary             string       `json:"summary,omitempty"`
	AlternativeNames    []string     `json:"alternative_names,omitempty"`
	Metadatum           RomMetadata  `json:"metadatum,omitempty"`
	PathCoverSmall      string       `json:"path_cover_small,omitempty"`
	PathCoverLarge      string       `json:"path_cover_large,omitempty"`
	URLCover            string       `json:"url_cover,omitempty"`
	HasManual           bool         `json:"has_manual,omitempty"`
	PathManual          string       `json:"path_manual,omitempty"`
	URLManual           string       `json:"url_manual,omitempty"`
	UserScreenshots     []Screenshot `json:"user_screenshots,omitempty"`
	MergedScreenshots   []string     `json:"merged_screenshots,omitempty"`
	IsIdentifying       bool         `json:"is_identifying,omitempty"`
	IsUnidentified      bool         `json:"is_unidentified,omitempty"`
	IsIdentified        bool         `json:"is_identified,omitempty"`
	Revision            string       `json:"revision,omitempty"`
	Regions             []string     `json:"regions,omitempty"`
	Languages           []string     `json:"languages,omitempty"`
	Tags                []any        `json:"tags,omitempty"`
	CrcHash             string       `json:"crc_hash,omitempty"`
	Md5Hash             string       `json:"md5_hash,omitempty"`
	Sha1Hash            string       `json:"sha1_hash,omitempty"`
	Multi               bool         `json:"multi,omitempty"`
	HasSimpleSingleFile bool         `json:"has_simple_single_file,omitempty"`
	HasNestedSingleFile bool         `json:"has_nested_single_file,omitempty"`
	HasMultipleFiles    bool         `json:"has_multiple_files,omitempty"`
	Files               []RomFile    `json:"files,omitempty"`
	FullPath            string       `json:"full_path,omitempty"`
	CreatedAt           time.Time    `json:"created_at,omitempty"`
	UpdatedAt           time.Time    `json:"updated_at,omitempty"`
	MissingFromFs       bool         `json:"missing_from_fs,omitempty"`
	Siblings            []any        `json:"siblings,omitempty"`
}

func (r Rom) GetGamePage(host models.Host) string {
	u, _ := url.JoinPath(host.URL(), "rom", strconv.Itoa(r.ID))
	return u
}

type Screenshot struct {
	ID       int    `json:"id,omitempty"`
	RomID    int    `json:"rom_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	URLPath  string `json:"url_path,omitempty"`
	Order    int    `json:"order,omitempty"`
}

type RomMetadata struct {
	RomID            int      `json:"rom_id,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	Franchises       []any    `json:"franchises,omitempty"`
	Collections      []string `json:"collections,omitempty"`
	Companies        []string `json:"companies,omitempty"`
	GameModes        []string `json:"game_modes,omitempty"`
	AgeRatings       []string `json:"age_ratings,omitempty"`
	FirstReleaseDate int64    `json:"first_release_date,omitempty"`
	AverageRating    float64  `json:"average_rating,omitempty"`
}

type RomFile struct {
	ID            int       `json:"id,omitempty"`
	RomID         int       `json:"rom_id,omitempty"`
	FileName      string    `json:"file_name,omitempty"`
	FilePath      string    `json:"file_path,omitempty"`
	FileSizeBytes int       `json:"file_size_bytes,omitempty"`
	FullPath      string    `json:"full_path,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	CrcHash       string    `json:"crc_hash,omitempty"`
	Md5Hash       string    `json:"md5_hash,omitempty"`
	Sha1Hash      string    `json:"sha1_hash,omitempty"`
	Category      any       `json:"category,omitempty"`
}

type GetRomsOptions struct {
	Page         int
	Limit        int
	PlatformID   *int
	CollectionID *int
	Search       string
	OrderBy      string
	OrderDir     string
}

func (c *Client) GetRoms(opts *GetRomsOptions) (*PaginatedRoms, error) {
	params := map[string]string{}

	if opts != nil {
		if opts.Page > 0 {
			params["page"] = strconv.Itoa(opts.Page)
		}
		if opts.Limit > 0 {
			params["limit"] = strconv.Itoa(opts.Limit)
		}
		if opts.PlatformID != nil {
			params["platform_id"] = strconv.Itoa(*opts.PlatformID)
		}
		if opts.CollectionID != nil {
			params["collection_id"] = strconv.Itoa(*opts.CollectionID)
		}
		if opts.Search != "" {
			params["search"] = opts.Search
		}
		if opts.OrderBy != "" {
			params["order_by"] = opts.OrderBy
		}
		if opts.OrderDir != "" {
			params["order_dir"] = opts.OrderDir
		}
	}

	var result PaginatedRoms
	path := "/api/roms" + buildQueryString(params)
	err := c.doRequest("GET", path, nil, &result)

	return &result, err
}

func (c *Client) AddRom(platformID int, file io.Reader, filename string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add platform ID
	if err := writer.WriteField("platform_id", strconv.Itoa(platformID)); err != nil {
		return fmt.Errorf("failed to write platform_id field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create file form field: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	writer.Close()

	return c.doMultipartRequest("POST", "/api/roms", &body, writer.FormDataContentType(), nil)
}

func (c *Client) GetRom(id int) (*Rom, error) {
	var rom Rom
	path := fmt.Sprintf("/api/roms/%d", id)
	err := c.doRequest("GET", path, nil, &rom)
	return &rom, err
}

func (c *Client) GetRomContent(id int, fileName string) ([]byte, error) {
	path := fmt.Sprintf("/api/roms/%d/content/%s", id, fileName)
	return c.doRequestRaw("GET", path, nil)
}

func (c *Client) GetRomFile(id int) (*RomFile, error) {
	var romFile RomFile
	path := fmt.Sprintf("/api/romsfiles/%d", id)
	err := c.doRequest("GET", path, nil, &romFile)
	return &romFile, err
}

func (c *Client) GetRomFileContent(id int, fileName string) ([]byte, error) {
	path := fmt.Sprintf("/api/romsfiles/%d/content/%s", id, fileName)
	return c.doRequestRaw("GET", path, nil)
}

func (c *Client) DownloadRoms(romIDs []int) ([]byte, error) {
	params := map[string]string{}

	// Build comma-separated list of ROM IDs
	if len(romIDs) > 0 {
		ids := ""
		for i, id := range romIDs {
			if i > 0 {
				ids += ","
			}
			ids += strconv.Itoa(id)
		}
		params["rom_ids"] = ids
	}

	path := "/api/roms/download" + buildQueryString(params)
	return c.doRequestRaw("GET", path, nil)
}

// DownloadMultiFileRom downloads a multi-file ROM as a zip archive
// The zip contains all ROM files and a m3u playlist file
func (c *Client) DownloadMultiFileRom(romID int) ([]byte, error) {
	return c.DownloadRoms([]int{romID})
}
