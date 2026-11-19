package config

import (
	"encoding/json"
	"time"
)

type DurationSeconds time.Duration

func (d DurationSeconds) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(time.Duration(d).Seconds()))
}

func (d *DurationSeconds) UnmarshalJSON(b []byte) error {
	var raw int64
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if raw > 1_000_000 {
		*d = DurationSeconds(time.Duration(raw))
	} else {
		*d = DurationSeconds(time.Duration(raw) * time.Second)
	}
	return nil
}

func (d DurationSeconds) Duration() time.Duration {
	return time.Duration(d)
}

type DirectoryMapping struct {
	RomMSlug     string `json:"slug"`
	RelativePath string `json:"relative_path"`
}

type AdditionalDownloads struct {
	Marquee   ArtKind `json:"marquee,omitempty"`
	Video     bool    `json:"video,omitempty"`
	Thumbnail ArtKind `json:"thumbnail,omitempty"`
	Bezel     bool    `json:"bezel,omitempty"`
	Manual    bool    `json:"manual,omitempty"`
	BoxBack   bool    `json:"box_back,omitempty"`
	Fanart    bool    `json:"fanart,omitempty"`
}

type Config struct {
	Hosts                        []Host                      `json:"hosts,omitempty"`
	DirectoryMappings            map[string]DirectoryMapping `json:"directory_mappings,omitempty"`
	DownloadArt                  bool                        `json:"download_art,omitempty"`
	ShowBoxArt                   bool                        `json:"show_box_art,omitempty"`
	UnzipDownloads               bool                        `json:"unzip_downloads,omitempty"`
	ShowRegularCollections       bool                        `json:"show_collections"`
	ShowSmartCollections         bool                        `json:"show_smart_collections"`
	ShowVirtualCollections       bool                        `json:"show_virtual_collections"`
	DownloadedGames              DownloadedGamesMode         `json:"downloaded_games,omitempty"`
	ApiTimeout                   DurationSeconds             `json:"api_timeout"`
	DownloadTimeout              DurationSeconds             `json:"download_timeout"`
	LogLevel                     LogLevel                    `json:"log_level,omitempty"`
	Language                     string                      `json:"language,omitempty"`
	CollectionView               CollectionView              `json:"collection_view,omitempty"`
	KidMode                      bool                        `json:"kid_mode,omitempty"`
	ReleaseChannel               ReleaseChannel              `json:"release_channel,omitempty"`
	ArtKind                      ArtKind                     `json:"art_kind,omitempty"`
	DownloadArtScreenshotPreview bool                        `json:"download_art_screenshot_preview,omitempty"`
	DownloadSplashArt            ArtKind                     `json:"download_splash_art,omitempty"`
	AdditionalDownloads          AdditionalDownloads         `json:"additional_downloads,omitempty"`
	GamelistOmitsRegion          bool                        `json:"gamelist_omits_region,omitempty"`
	MuOSRomStorage               RomStorage                  `json:"muos_rom_storage,omitempty"`
	SwapFaceButtons              bool                        `json:"swap_face_buttons,omitempty"`
	PlatformOrder                []string                    `json:"platform_order,omitempty"`
	SaveDirectoryMappings        map[string]string           `json:"save_directory_mappings,omitempty"`
	SaveBackupLimit              int                         `json:"save_backup_limit,omitempty"`
	ChannelPreferences           map[int]string              `json:"channel_preferences,omitempty"`
}

func applyDefaults(c *Config) {
	if c.ApiTimeout == 0 || c.ApiTimeout.Duration() > 300*time.Second {
		c.ApiTimeout = DurationSeconds(30 * time.Second)
	}
	if c.DownloadTimeout == 0 {
		c.DownloadTimeout = DurationSeconds(60 * time.Minute)
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if c.LogLevel == "" {
		c.LogLevel = LogLevelError
	}
	if c.ReleaseChannel == "" {
		c.ReleaseChannel = ReleaseChannelMatchRomM
	}
	if c.DownloadedGames == "" {
		c.DownloadedGames = DownloadedGamesModeDoNothing
	}
	if c.CollectionView == "" {
		c.CollectionView = CollectionViewPlatform
	}
	if c.ArtKind == "" {
		c.ArtKind = ArtKindDefault
	}
}
