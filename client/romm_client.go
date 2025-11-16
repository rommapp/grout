package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"grout/models"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

type RomMClient struct {
	Hostname   string
	Port       int
	Username   string
	Password   string
	HttpClient *http.Client
}

type RomMPlatform struct {
	ID          int           `json:"id"`
	Slug        string        `json:"slug"`
	FsSlug      string        `json:"fs_slug"`
	RomCount    int           `json:"rom_count"`
	Name        string        `json:"name"`
	CustomName  string        `json:"custom_name"`
	IgdbID      int           `json:"igdb_id"`
	SgdbID      interface{}   `json:"sgdb_id"`
	MobyID      int           `json:"moby_id"`
	SsID        int           `json:"ss_id"`
	Category    string        `json:"category"`
	Generation  int           `json:"generation"`
	FamilyName  string        `json:"family_name"`
	FamilySlug  string        `json:"family_slug"`
	URL         string        `json:"url"`
	URLLogo     string        `json:"url_logo"`
	LogoPath    string        `json:"logo_path"`
	Firmware    []interface{} `json:"firmware"`
	AspectRatio string        `json:"aspect_ratio"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	DisplayName string        `json:"display_name"`
}

type RomMList struct {
	CharIndex map[string]int `json:"char_index"`
	Items     []RomMRom      `json:"items"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
	Total     int            `json:"total"`
}

type RomMRom struct {
	ID                  int         `json:"id"`
	IgdbID              int         `json:"igdb_id"`
	SgdbID              interface{} `json:"sgdb_id"`
	MobyID              interface{} `json:"moby_id"`
	SsID                interface{} `json:"ss_id"`
	PlatformID          int         `json:"platform_id"`
	PlatformSlug        string      `json:"platform_slug"`
	PlatformFsSlug      string      `json:"platform_fs_slug"`
	PlatformName        string      `json:"platform_name"`
	PlatformCustomName  string      `json:"platform_custom_name"`
	PlatformDisplayName string      `json:"platform_display_name"`
	FsName              string      `json:"fs_name"`
	FsNameNoTags        string      `json:"fs_name_no_tags"`
	FsNameNoExt         string      `json:"fs_name_no_ext"`
	FsExtension         string      `json:"fs_extension"`
	FsPath              string      `json:"fs_path"`
	FsSizeBytes         int         `json:"fs_size_bytes"`
	Name                string      `json:"name"`
	Slug                string      `json:"slug"`
	Summary             string      `json:"summary"`
	FirstReleaseDate    int64       `json:"first_release_date"`
	YoutubeVideoID      string      `json:"youtube_video_id"`
	AverageRating       float64     `json:"average_rating"`
	AlternativeNames    []string    `json:"alternative_names"`
	Genres              []string    `json:"genres"`
	Franchises          []string    `json:"franchises"`
	MetaCollections     []string    `json:"meta_collections"`
	Companies           []string    `json:"companies"`
	GameModes           []string    `json:"game_modes"`
	AgeRatings          []string    `json:"age_ratings"`
	IgdbMetadata        struct {
		TotalRating      string   `json:"total_rating"`
		AggregatedRating string   `json:"aggregated_rating"`
		FirstReleaseDate int      `json:"first_release_date"`
		YoutubeVideoID   string   `json:"youtube_video_id"`
		Genres           []string `json:"genres"`
		Franchises       []string `json:"franchises"`
		AlternativeNames []string `json:"alternative_names"`
		Collections      []string `json:"collections"`
		Companies        []string `json:"companies"`
		GameModes        []string `json:"game_modes"`
		AgeRatings       []struct {
			Rating         string `json:"rating"`
			Category       string `json:"category"`
			RatingCoverURL string `json:"rating_cover_url"`
		} `json:"age_ratings"`
		Platforms []struct {
			IgdbID int    `json:"igdb_id"`
			Name   string `json:"name"`
		} `json:"platforms"`
		Expansions []interface{} `json:"expansions"`
		Dlcs       []interface{} `json:"dlcs"`
		Remasters  []interface{} `json:"remasters"`
		Remakes    []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Type     string `json:"type"`
			CoverURL string `json:"cover_url"`
		} `json:"remakes"`
		ExpandedGames []interface{} `json:"expanded_games"`
		Ports         []interface{} `json:"ports"`
		SimilarGames  []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Type     string `json:"type"`
			CoverURL string `json:"cover_url"`
		} `json:"similar_games"`
	} `json:"igdb_metadata"`
	MobyMetadata struct {
	} `json:"moby_metadata"`
	SsMetadata     interface{}   `json:"ss_metadata"`
	PathCoverSmall string        `json:"path_cover_small"`
	PathCoverLarge string        `json:"path_cover_large"`
	URLCover       string        `json:"url_cover"`
	HasManual      bool          `json:"has_manual"`
	PathManual     interface{}   `json:"path_manual"`
	URLManual      interface{}   `json:"url_manual"`
	IsUnidentified bool          `json:"is_unidentified"`
	Revision       string        `json:"revision"`
	Regions        []interface{} `json:"regions"`
	Languages      []interface{} `json:"languages"`
	Tags           []interface{} `json:"tags"`
	CrcHash        string        `json:"crc_hash"`
	Md5Hash        string        `json:"md5_hash"`
	Sha1Hash       string        `json:"sha1_hash"`
	Multi          bool          `json:"multi"`
	Files          []struct {
		ID            int         `json:"id"`
		RomID         int         `json:"rom_id"`
		FileName      string      `json:"file_name"`
		FilePath      string      `json:"file_path"`
		FileSizeBytes int         `json:"file_size_bytes"`
		FullPath      string      `json:"full_path"`
		CreatedAt     time.Time   `json:"created_at"`
		UpdatedAt     time.Time   `json:"updated_at"`
		LastModified  time.Time   `json:"last_modified"`
		CrcHash       string      `json:"crc_hash"`
		Md5Hash       string      `json:"md5_hash"`
		Sha1Hash      string      `json:"sha1_hash"`
		Category      interface{} `json:"category"`
	} `json:"files"`
	FullPath    string        `json:"full_path"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	SiblingRoms []interface{} `json:"sibling_roms"`
	RomUser     struct {
		ID              int         `json:"id"`
		UserID          int         `json:"user_id"`
		RomID           int         `json:"rom_id"`
		CreatedAt       time.Time   `json:"created_at"`
		UpdatedAt       time.Time   `json:"updated_at"`
		LastPlayed      interface{} `json:"last_played"`
		NoteRawMarkdown string      `json:"note_raw_markdown"`
		NoteIsPublic    bool        `json:"note_is_public"`
		IsMainSibling   bool        `json:"is_main_sibling"`
		Backlogged      bool        `json:"backlogged"`
		NowPlaying      bool        `json:"now_playing"`
		Hidden          bool        `json:"hidden"`
		Rating          int         `json:"rating"`
		Difficulty      int         `json:"difficulty"`
		Completion      int         `json:"completion"`
		Status          interface{} `json:"status"`
		UserUsername    string      `json:"user__username"`
	} `json:"rom_user"`
	SortComparator string `json:"sort_comparator"`
}

const RomsEndpoint = "/api/roms/"

func NewRomMClient(host models.Host) *RomMClient {
	client := &http.Client{Timeout: 1750 * time.Millisecond}
	return &RomMClient{
		Hostname:   host.RootURI,
		Port:       host.Port,
		Username:   host.Username,
		Password:   host.Password,
		HttpClient: client,
	}
}

func (c *RomMClient) Close() error {
	return nil
}

func (c *RomMClient) newRequest(method, endpoint string, params url.Values, auth bool) (*http.Request, error) {
	u, err := url.Parse(c.buildRootURL())
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL: %v", err)
	}

	u = u.JoinPath(endpoint)
	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %v", err)
	}

	if auth {
		authStr := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password))
		req.Header.Set("Authorization", "Basic "+authStr)
	}

	return req, nil
}

func (c *RomMClient) fetch(req *http.Request, v interface{}) error {
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

func (c *RomMClient) buildRootURL() string {
	if c.Port != 0 {
		return fmt.Sprintf("%s:%d", c.Hostname, c.Port)
	}
	return c.Hostname
}

func (c *RomMClient) Heartbeat() bool {
	req, err := c.newRequest("GET", "/api/heartbeat", nil, true)
	if err != nil {
		return false
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func (c *RomMClient) Login() bool {
	req, err := c.newRequest("POST", "/api/login", nil, true)
	if err != nil {
		return false
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func (c *RomMClient) GetPlatforms() ([]RomMPlatform, error) {
	req, err := c.newRequest("GET", "/api/platforms/", nil, true)
	if err != nil {
		return nil, err
	}

	var platforms []RomMPlatform
	if err := c.fetch(req, &platforms); err != nil {
		return nil, err
	}

	return platforms, nil
}

func (c *RomMClient) ListDirectory(platformID string) (shared.Items, error) {
	params := url.Values{
		"platform_id": {platformID},
		"limit":       {"10000"},
	}

	req, err := c.newRequest("GET", RomsEndpoint, params, true)
	if err != nil {
		return nil, err
	}

	var rawItemsList RomMList
	if err := c.fetch(req, &rawItemsList); err != nil {
		return nil, err
	}

	items := make([]shared.Item, 0, len(rawItemsList.Items))
	for _, rawItem := range rawItemsList.Items {
		items = append(items, shared.Item{
			DisplayName:  rawItem.FsNameNoTags,
			Filename:     rawItem.FsName,
			FileSize:     strconv.Itoa(rawItem.FsSizeBytes),
			LastModified: rawItem.UpdatedAt.String(),
			RomID:        strconv.Itoa(rawItem.ID),
			ArtURL:       rawItem.PathCoverSmall,
		})
	}

	return items, nil
}

func (c *RomMClient) BuildDownloadURL(remotePath, filename string) (string, error) {
	return url.JoinPath(c.buildRootURL(), RomsEndpoint, remotePath, "content", filename)
}

func (c *RomMClient) BuildDownloadHeaders() map[string]string {
	return make(map[string]string)
}

func (c *RomMClient) DownloadArt(remotePath, localPath, filename, rename string) (string, error) {
	logger := gaba.GetLoggerInstance()
	logger.Debug("Downloading file...",
		"remotePath", remotePath,
		"localPath", localPath,
		"filename", filename,
		"rename", rename)

	sourceURL, err := url.JoinPath(c.buildRootURL(), remotePath, filename)
	if err != nil {
		return "", fmt.Errorf("unable to build download url: %w", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Determine final filename
	finalName := filename
	if rename != "" {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(rename, filepath.Ext(rename))
		finalName = base + ext
	}

	fullPath := filepath.Join(localPath, finalName)
	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return fullPath, nil
}
