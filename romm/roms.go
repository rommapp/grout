package romm

import (
	"fmt"
	"grout/internal/artutil"
	"grout/internal/fileutil"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	RommAssetPrefix = "/assets/romm/resources"
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
	ID                    int            `json:"id,omitempty"`
	GameListID            any            `json:"gamelist_id,omitempty"`
	PlatformID            int            `json:"platform_id,omitempty"`
	PlatformSlug          string         `json:"platform_slug,omitempty"`
	PlatformFSSlug        string         `json:"platform_fs_slug,omitempty"`
	PlatformCustomName    string         `json:"platform_custom_name,omitempty"`
	PlatformDisplayName   string         `json:"platform_display_name,omitempty"`
	FsName                string         `json:"fs_name,omitempty"`
	FsNameNoTags          string         `json:"fs_name_no_tags,omitempty"`
	FsNameNoExt           string         `json:"fs_name_no_ext,omitempty"`
	FsExtension           string         `json:"fs_extension,omitempty"`
	FsPath                string         `json:"fs_path,omitempty"`
	FsSizeBytes           int64          `json:"fs_size_bytes,omitempty"`
	Name                  string         `json:"name,omitempty"`
	DisplayName           string         `json:"-"`
	Slug                  string         `json:"slug,omitempty"`
	ScreenScraperID       int            `json:"ss_id,omitempty"`
	Summary               string         `json:"summary,omitempty"`
	AlternativeNames      []string       `json:"alternative_names,omitempty"`
	Metadatum             RomMetadata    `json:"metadatum,omitempty"`
	PathCoverSmall        string         `json:"path_cover_small,omitempty"`
	PathCoverLarge        string         `json:"path_cover_large,omitempty"`
	URLCover              string         `json:"url_cover,omitempty"`
	HasManual             bool           `json:"has_manual,omitempty"`
	PathManual            string         `json:"path_manual,omitempty"`
	URLManual             string         `json:"url_manual,omitempty"`
	UserScreenshots       []Screenshot   `json:"user_screenshots,omitempty"`
	UserSaves             []Save         `json:"user_saves,omitempty"`
	MergedScreenshots     []string       `json:"merged_screenshots,omitempty"`
	IsIdentifying         bool           `json:"is_identifying,omitempty"`
	IsUnidentified        bool           `json:"is_unidentified,omitempty"`
	IsIdentified          bool           `json:"is_identified,omitempty"`
	Revision              string         `json:"revision,omitempty"`
	Regions               []string       `json:"regions,omitempty"`
	Languages             []string       `json:"languages,omitempty"`
	Tags                  []any          `json:"tags,omitempty"`
	CrcHash               string         `json:"crc_hash,omitempty"`
	Md5Hash               string         `json:"md5_hash,omitempty"`
	Sha1Hash              string         `json:"sha1_hash,omitempty"`
	RetroAchievementsHash string         `json:"ra_hash,omitempty"`
	RetroAchievementsID   int            `json:"ra_id,omitempty"`
	HasSimpleSingleFile   bool           `json:"has_simple_single_file,omitempty"`
	HasNestedSingleFile   bool           `json:"has_nested_single_file,omitempty"`
	HasMultipleFiles      bool           `json:"has_multiple_files,omitempty"`
	Files                 []RomFile      `json:"files,omitempty"`
	FullPath              string         `json:"full_path,omitempty"`
	CreatedAt             time.Time      `json:"created_at,omitempty"`
	UpdatedAt             time.Time      `json:"updated_at,omitempty"`
	MissingFromFs         bool           `json:"missing_from_fs,omitempty"`
	Siblings              []any          `json:"siblings,omitempty"`
	PathVideo             string         `json:"path_video,omitempty"`
	ScreenScraperMetadata ScreenScrapper `json:"ss_metadata,omitempty"`
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
	FileSizeBytes int64     `json:"file_size_bytes,omitempty"`
	FullPath      string    `json:"full_path,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	CrcHash       string    `json:"crc_hash,omitempty"`
	Md5Hash       string    `json:"md5_hash,omitempty"`
	Sha1Hash      string    `json:"sha1_hash,omitempty"`
	RAHash        string    `json:"ra_hash,omitempty"`
	Category      any       `json:"category,omitempty"`
}

type GetRomsQuery struct {
	Offset              int    `qs:"offset,omitempty"`
	Limit               int    `qs:"limit,omitempty"`
	PlatformIDs         []int  `qs:"platform_ids,omitempty"`
	CollectionID        int    `qs:"collection_id,omitempty"`
	SmartCollectionID   int    `qs:"smart_collection_id,omitempty"`
	VirtualCollectionID string `qs:"virtual_collection_id,omitempty"`
	Search              string `qs:"search,omitempty"`
	OrderBy             string `qs:"order_by,omitempty"`
	OrderDir            string `qs:"order_dir,omitempty"`
	UpdatedAfter        string `qs:"updated_after,omitempty"` // ISO8601 timestamp with timezone
	WithFilterValues    bool   `qs:"with_filter_values"`
	WithCharIndex       bool   `qs:"with_char_index"`
	WithFiles           bool   `qs:"with_files"`
}

func (q GetRomsQuery) Valid() bool {
	return q.Limit > 0 || len(q.PlatformIDs) > 0 || q.CollectionID > 0 || q.SmartCollectionID > 0 || q.VirtualCollectionID != "" || q.Search != "" || q.OrderBy != "" || q.OrderDir != ""
}

type GetRomByHashQuery struct {
	CrcHash  string `qs:"crc_hash,omitempty"`
	Md5Hash  string `qs:"md5_hash,omitempty"`
	Sha1Hash string `qs:"sha1_hash,omitempty"`
}

func (q GetRomByHashQuery) Valid() bool {
	return q.CrcHash != "" || q.Md5Hash != "" || q.Sha1Hash != ""
}

func (c *Client) GetRoms(query GetRomsQuery) (PaginatedRoms, error) {
	var result PaginatedRoms
	err := c.doRequest("GET", endpointRoms, query, nil, &result)
	return result, err
}
func (c *Client) GetRomIdentifiers() ([]int, error) {
	var ids []int
	err := c.doRequest("GET", endpointRomIdentifiers, nil, nil, &ids)
	return ids, err
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
func joinPathWithQuery(base string, elem ...string) (string, error) {
	last := elem[len(elem)-1]
	// because url.JoinPath doesn't handle query parameters without encoding all the string
	// we need to check if the last element contains a "?" and split it before joining
	if idx := strings.Index(last, "?"); idx != -1 {
		joined, err := url.JoinPath(base, append(elem[:len(elem)-1], last[:idx])...)
		if err != nil {
			return "", err
		}
		return joined + last[idx:], nil
	}
	return url.JoinPath(base, elem...)
}

// resolveAssetURL resolves a RomM asset path or fallback URL into a full URL.
// If path is set, it joins it with the host URL (prepending the asset prefix if needed).
// If path is empty, it falls back to fallbackURL.
func resolveAssetURL(host Host, path, fallbackURL string) string {
	if path != "" {
		var result string
		var err error
		if !strings.Contains(path, RommAssetPrefix) {
			result, err = joinPathWithQuery(host.URL(), RommAssetPrefix, path)
		} else {
			result, err = joinPathWithQuery(host.URL(), path)
		}
		if err != nil {
			gaba.GetLogger().Error("Error resolving asset URL", "error", err, "hostURL", host.ToLoggable(), "path", path)
			return ""
		}
		return strings.ReplaceAll(result, " ", "%20")
	}
	if fallbackURL != "" {
		return strings.ReplaceAll(fallbackURL, " ", "%20")
	}
	return ""
}

func (r *Rom) GetGamePage(host Host) string {
	u, _ := url.JoinPath(host.URL(), "rom", strconv.Itoa(r.ID))
	return u
}

// CanonicalLocalBasename returns the extension-less filename this ROM occupies on
// disk once downloaded — the single identity used to resolve local ROM files and
// emulator save files back to this ROM. It mirrors the download path exactly:
//   - multi-file ROMs are written/loaded through an m3u named after FsNameNoExt;
//   - single-file ROMs (including RomM "nested single file" entries, where FsName
//     is the containing folder rather than the file) are written using the
//     individual file's name, so the folder-derived FsNameNoExt must NOT be used.
//
// Keying matching on this value (instead of fs_name_no_ext) fixes the case where a
// downloaded ROM and its saves never matched their own cache row (issue #242).
func (r *Rom) CanonicalLocalBasename() string {
	if !r.HasMultipleFiles && len(r.Files) > 0 {
		fileName := r.Files[0].FileName
		return strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	return r.FsNameNoExt
}

// LocalBasenames returns every extension-less basename this ROM can occupy on disk, so a
// downloaded ROM or emulator save file can be resolved back to it regardless of which file
// the user installed. Multi-disc ROMs are loaded through an m3u named after FsNameNoExt, so
// that single basename identifies them. Other ROMs may bundle several alternative files
// (regions/revisions) and the user downloads one of them (ui/download.go selectedFileID), so
// EACH file's basename is a valid on-disk identity — keying matching on only Files[0] left
// saves for any other version unmatched (issue #242). Falls back to FsNameNoExt when there
// is no file metadata. The result is de-duplicated, preserving first-seen order.
func (r *Rom) LocalBasenames() []string {
	if r.HasMultipleFiles {
		return []string{r.FsNameNoExt}
	}
	seen := make(map[string]bool, len(r.Files))
	out := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		base := strings.TrimSuffix(f.FileName, filepath.Ext(f.FileName))
		if base != "" && !seen[base] {
			seen[base] = true
			out = append(out, base)
		}
	}
	if len(out) == 0 {
		return []string{r.FsNameNoExt}
	}
	return out
}

func (r *Rom) GetLocalPath(resolver PlatformDirResolver) string {
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

func (r *Rom) IsDownloaded(resolver PlatformDirResolver) bool {
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

func (r *Rom) IsFileDownloaded(resolver PlatformDirResolver, fileName string) bool {
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

func (r *Rom) MaxPlayerCount() int {
	maxPlayers := 1
	if r.Metadatum.GameModes != nil && len(r.Metadatum.GameModes) > 0 {
		if slices.Contains(r.Metadatum.GameModes, "Multiplayer") {
			maxPlayers = 4
		}
		if slices.Contains(r.Metadatum.GameModes, "Co-operative") {
			maxPlayers = 2
		}
	}

	return maxPlayers
}

func (r *Rom) GetArtworkURL(kind artutil.ArtKind, host Host) string {
	var (
		coverURL string
		boxPath  string
	)
	var err error
	logger := gaba.GetLogger()
	logger.Debug("Getting artwork URL for ROM", "romID", r.ID, "romName", r.Name, "artKind", kind)

	if kind == artutil.ArtKindBox2D {
		if r.ScreenScraperMetadata.Box2DURL != "" {
			coverURL = r.ScreenScraperMetadata.Box2DURL
			boxPath = r.ScreenScraperMetadata.Box2DURL
		}
	} else if kind == artutil.ArtKindBox3D {
		if r.ScreenScraperMetadata.Box3DPath != "" {
			if !strings.Contains(r.ScreenScraperMetadata.Box3DPath, RommAssetPrefix) {
				coverURL, err = joinPathWithQuery(host.URL(), RommAssetPrefix, r.ScreenScraperMetadata.Box3DPath)
			} else {
				coverURL, err = joinPathWithQuery(host.URL(), r.ScreenScraperMetadata.Box3DPath)
			}
			boxPath = r.ScreenScraperMetadata.Box3DPath
		} else if r.ScreenScraperMetadata.Box3DURL != "" {
			coverURL = r.ScreenScraperMetadata.Box3DURL
			boxPath = r.ScreenScraperMetadata.Box3DURL
		}
	} else if kind == artutil.ArtKindMixImage {
		if r.ScreenScraperMetadata.MiximagePath != "" {
			if !strings.Contains(r.ScreenScraperMetadata.MiximagePath, RommAssetPrefix) {
				coverURL, err = joinPathWithQuery(host.URL(), RommAssetPrefix, r.ScreenScraperMetadata.MiximagePath)
			} else {
				coverURL, err = joinPathWithQuery(host.URL(), r.ScreenScraperMetadata.MiximagePath)
			}
			boxPath = r.ScreenScraperMetadata.MiximagePath
		} else if r.ScreenScraperMetadata.MiximageURL != "" {
			coverURL = r.ScreenScraperMetadata.MiximageURL
			boxPath = r.ScreenScraperMetadata.MiximageURL
		}
	}

	if kind == artutil.ArtKindDefault || coverURL == "" {
		if r.PathCoverSmall != "" {
			coverURL, err = joinPathWithQuery(host.URL(), r.PathCoverSmall)
			boxPath = r.PathCoverSmall
		} else if r.PathCoverLarge != "" {
			coverURL, err = joinPathWithQuery(host.URL(), r.PathCoverLarge)
			boxPath = r.PathCoverLarge
		} else if r.URLCover != "" {
			coverURL = r.URLCover
			boxPath = r.URLCover
		}
	}

	logger.Debug("Using cover URL", "url", coverURL)
	if coverURL == "" && err != nil {
		logger.Error("Error joining host URL with box path", "error", err, "hostURL", host.ToLoggable(), "boxPath", boxPath)
	}

	return strings.ReplaceAll(coverURL, " ", "%20")
}

func (r *Rom) GetScreenshotURL(host Host) string {
	var screenshotURL string
	var err error
	logger := gaba.GetLogger()
	if len(r.UserScreenshots) > 0 {
		screenshotURL, err = joinPathWithQuery(host.URL(), r.UserScreenshots[0].URLPath)
	} else if len(r.MergedScreenshots) > 0 {
		screenshotURL, err = joinPathWithQuery(host.URL(), r.MergedScreenshots[0])
	} else if r.ScreenScraperMetadata.ScreenshotURL != "" {
		screenshotURL = r.ScreenScraperMetadata.ScreenshotURL
	}

	if screenshotURL == "" || err != nil {
		logger.Error("No screenshot found in UserScreenshots, MergedScreenshots or ScreenScraper", "error", err, "hostURL", host.ToLoggable())
	}

	return strings.ReplaceAll(screenshotURL, " ", "%20")
}

func (r *Rom) GetSplashArtURL(kind artutil.ArtKind, host Host) string {
	switch kind {
	case artutil.ArtKindMarquee:
		return resolveAssetURL(host, r.ScreenScraperMetadata.MarqueePath, r.ScreenScraperMetadata.MarqueeURL)
	case artutil.ArtKindTitle:
		return resolveAssetURL(host, "", r.ScreenScraperMetadata.TitleScreenURL)
	default:
		return ""
	}
}

func (r *Rom) GetMarqueeURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.MarqueePath, r.ScreenScraperMetadata.MarqueeURL)
}

func (r *Rom) GetLogoURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.LogoPath, r.ScreenScraperMetadata.LogoURL)
}

func (r *Rom) GetVideoURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.VideoPath, r.ScreenScraperMetadata.VideoURL)
}

func (r *Rom) GetBezelURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.BezelPath, r.ScreenScraperMetadata.BezelURL)
}

func (r *Rom) GetManualURL(host Host) string {
	if r.HasManual {
		if result := resolveAssetURL(host, r.PathManual, r.URLManual); result != "" {
			return result
		}
	}
	return resolveAssetURL(host, "", r.ScreenScraperMetadata.ManualURL)
}

func (r *Rom) GetBoxbackURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.Box2DBackPath, r.ScreenScraperMetadata.Box2DBackURL)
}

func (r *Rom) GetFanartURL(host Host) string {
	return resolveAssetURL(host, r.ScreenScraperMetadata.FanartPath, r.ScreenScraperMetadata.FanartURL)
}
