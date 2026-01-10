package romm

import (
	"fmt"
	"grout/internal/fileutil"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sonh/qs"
)

type PlatformDirResolver interface {
	GetPlatformRomDirectory(Platform) string
}

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
	PlatformFSSlug      string `json:"platform_fs_slug,omitempty"`
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
	UserSaves           []Save       `json:"user_saves,omitempty"`
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

type GetRomsQuery struct {
	Offset              int    `qs:"offset,omitempty"`
	Limit               int    `qs:"limit,omitempty"`
	PlatformID          int    `qs:"platform_id,omitempty"`
	CollectionID        int    `qs:"collection_id,omitempty"`
	SmartCollectionID   int    `qs:"smart_collection_id,omitempty"`
	VirtualCollectionID string `qs:"virtual_collection_id,omitempty"`
	Search              string `qs:"search,omitempty"`
	OrderBy             string `qs:"order_by,omitempty"`
	OrderDir            string `qs:"order_dir,omitempty"`
}

func (q GetRomsQuery) Valid() bool {
	return q.Limit > 0 || q.PlatformID > 0 || q.CollectionID > 0 || q.SmartCollectionID > 0 || q.VirtualCollectionID != "" || q.Search != "" || q.OrderBy != "" || q.OrderDir != ""
}

type GetRomByHashQuery struct {
	CrcHash  string `qs:"crc_hash,omitempty"`
	Md5Hash  string `qs:"md5_hash,omitempty"`
	Sha1Hash string `qs:"sha1_hash,omitempty"`
}

func (q GetRomByHashQuery) Valid() bool {
	return q.CrcHash != "" || q.Md5Hash != "" || q.Sha1Hash != ""
}

type DownloadRomsQuery struct {
	RomIDs string `qs:"rom_ids"`
}

func (q DownloadRomsQuery) Valid() bool {
	return q.RomIDs != ""
}

func (c *Client) GetRoms(query GetRomsQuery) (PaginatedRoms, error) {
	var result PaginatedRoms
	err := c.doRequest("GET", endpointRoms, query, nil, &result)
	return result, err
}
func (c *Client) GetRomByHash(query GetRomByHashQuery) (Rom, error) {
	var rom Rom
	err := c.doRequest("GET", endpointRomsByHash, query, nil, &rom)
	return rom, err
}
func (c *Client) GetRom(id int) (Rom, error) {
	var rom Rom
	path := fmt.Sprintf(endpointRomByID, id)
	err := c.doRequest("GET", path, nil, nil, &rom)
	return rom, err
}
func (c *Client) DownloadRoms(romIDs []int) ([]byte, error) {
	if len(romIDs) == 0 {
		return c.doRequestRaw("GET", endpointRomsDownload, nil)
	}

	ids := ""
	for i, id := range romIDs {
		if i > 0 {
			ids += ","
		}
		ids += strconv.Itoa(id)
	}

	query := DownloadRomsQuery{RomIDs: ids}
	values, err := qs.NewEncoder().Values(query)
	if err != nil {
		return nil, err
	}

	path := endpointRomsDownload + "?" + values.Encode()
	return c.doRequestRaw("GET", path, nil)
}

func (r Rom) GetGamePage(host Host) string {
	u, _ := url.JoinPath(host.URL(), "rom", strconv.Itoa(r.ID))
	return u
}

func (r Rom) GetLocalPath(resolver PlatformDirResolver) string {
	if r.PlatformFSSlug == "" {
		return ""
	}

	platform := Platform{
		ID:     r.PlatformID,
		FSSlug: r.PlatformFSSlug,
		Name:   r.PlatformDisplayName,
	}

	romDirectory := resolver.GetPlatformRomDirectory(platform)

	if r.HasMultipleFiles {
		return filepath.Join(romDirectory, r.FsNameNoExt+".m3u")
	} else if len(r.Files) > 0 {
		return filepath.Join(romDirectory, r.Files[0].FileName)
	}

	return ""
}

func (r Rom) IsDownloaded(resolver PlatformDirResolver) bool {
	if r.PlatformFSSlug == "" {
		return false
	}

	platform := Platform{
		ID:     r.PlatformID,
		FSSlug: r.PlatformFSSlug,
		Name:   r.PlatformDisplayName,
	}
	romDirectory := resolver.GetPlatformRomDirectory(platform)

	// For multi-disk games, check the m3u file
	if r.HasMultipleFiles {
		m3uPath := filepath.Join(romDirectory, r.FsNameNoExt+".m3u")
		return fileutil.FileExists(m3uPath)
	}

	// Check if any of the associated files exist
	for _, file := range r.Files {
		filePath := filepath.Join(romDirectory, file.FileName)
		if fileutil.FileExists(filePath) {
			return true
		}
	}

	return false
}

func (r Rom) IsFileDownloaded(resolver PlatformDirResolver, fileName string) bool {
	if r.PlatformFSSlug == "" {
		return false
	}

	platform := Platform{
		ID:     r.PlatformID,
		FSSlug: r.PlatformFSSlug,
		Name:   r.PlatformDisplayName,
	}
	romDirectory := resolver.GetPlatformRomDirectory(platform)
	filePath := filepath.Join(romDirectory, fileName)
	return fileutil.FileExists(filePath)
}
