package internal

import (
	"encoding/json"
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal/artutil"
	"grout/romm"
	"os"
	"sync/atomic"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

var kidModeEnabled atomic.Bool

type AdditionalDownloads struct {
	Marquee   artutil.ArtKind `json:"marquee,omitempty"`
	Video     bool            `json:"video,omitempty"`
	Thumbnail artutil.ArtKind `json:"thumbnail,omitempty"`
	Bezel     bool            `json:"bezel,omitempty"`
	Manual    bool            `json:"manual,omitempty"`
	BoxBack   bool            `json:"box_back,omitempty"`
	Fanart    bool            `json:"fanart,omitempty"`
}

// DurationSeconds is a time.Duration that marshals to/from JSON as whole seconds.
// Existing configs with nanosecond values are handled by detecting large values on unmarshal.
type DurationSeconds time.Duration

func (d DurationSeconds) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(time.Duration(d).Seconds()))
}

func (d *DurationSeconds) UnmarshalJSON(b []byte) error {
	var raw int64
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	// Values over 1,000,000 are nanoseconds from old configs (e.g. 1800000000000 = 30min).
	// Convert them to the equivalent duration directly.
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

type Config struct {
	Hosts                        []romm.Host                 `json:"hosts,omitempty"`
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
	ArtKind                      artutil.ArtKind             `json:"art_kind,omitempty"`
	DownloadArtScreenshotPreview bool                        `json:"download_art_screenshot_preview,omitempty"`
	DownloadSplashArt            artutil.ArtKind             `json:"download_splash_art,omitempty"`
	AdditionalDownloads          AdditionalDownloads         `json:"additional_downloads,omitempty"`

	SwapFaceButtons       bool              `json:"swap_face_buttons,omitempty"`
	PlatformOrder         []string          `json:"platform_order,omitempty"`
	SaveDirectoryMappings map[string]string `json:"save_directory_mappings,omitempty"`
	SlotPreferences       map[string]string `json:"-"`                           // Stored in save_slots.json, not config.json
	SaveBackupLimit       int               `json:"save_backup_limit,omitempty"` // 0 = no limit, 5/10/15 = keep N most recent per game

	PlatformsBinding map[string]string `json:"-"`
}

type DirectoryMapping struct {
	RomMSlug     string `json:"slug"`
	RelativePath string `json:"relative_path"`
}

func (c Config) ToLoggable() any {
	safeHosts := make([]map[string]any, len(c.Hosts))
	for i, host := range c.Hosts {
		safeHosts[i] = host.ToLoggable()
	}

	return map[string]any{
		"hosts":                   safeHosts,
		"directory_mappings":      c.DirectoryMappings,
		"api_timeout":             c.ApiTimeout,
		"download_timeout":        c.DownloadTimeout,
		"unzip_downloads":         c.UnzipDownloads,
		"download_art":            c.DownloadArt,
		"art_kind":                c.ArtKind,
		"show_box_art":            c.ShowBoxArt,
		"collections":             c.ShowRegularCollections,
		"smart_collections":       c.ShowSmartCollections,
		"virtual_collections":     c.ShowVirtualCollections,
		"downloaded_games_action": c.DownloadedGames,
		"log_level":               c.LogLevel,
	}
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("reading config.json: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config.json: %w", err)
	}

	if config.ApiTimeout == 0 {
		config.ApiTimeout = DurationSeconds(30 * time.Second)
	}

	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = DurationSeconds(60 * time.Minute)
	}

	// Clamp API timeout to valid picker range (15s–300s)
	if config.ApiTimeout.Duration() > 300*time.Second {
		config.ApiTimeout = DurationSeconds(30 * time.Second)
	}

	if config.Language == "" {
		config.Language = "en"
	}

	if config.DownloadedGames == "" {
		config.DownloadedGames = DownloadedGamesModeDoNothing
	}

	if config.CollectionView == "" {
		config.CollectionView = CollectionViewPlatform
	}

	if config.ArtKind == "" {
		config.ArtKind = artutil.ArtKindDefault
	}

	if config.AdditionalDownloads.Thumbnail == "" {
		config.AdditionalDownloads.Thumbnail = artutil.ArtKindNone
	}

	if config.AdditionalDownloads.Marquee == "" {
		config.AdditionalDownloads.Marquee = artutil.ArtKindNone
	}

	// Load slot preferences from dedicated file
	config.SlotPreferences = LoadSlotPreferences()

	return &config, nil
}

func SaveConfig(config *Config) error {
	if config.LogLevel == "" {
		config.LogLevel = LogLevelError
	}

	if config.Language == "" {
		config.Language = "en"
	}

	if config.DownloadedGames == "" {
		config.DownloadedGames = DownloadedGamesModeDoNothing
	}

	if config.CollectionView == "" {
		config.CollectionView = CollectionViewPlatform
	}

	if config.ReleaseChannel == "" {
		config.ReleaseChannel = ReleaseChannelMatchRomM
	}

	if config.ArtKind == "" {
		config.ArtKind = artutil.ArtKindDefault
	}

	if config.AdditionalDownloads.Thumbnail == "" {
		config.AdditionalDownloads.Thumbnail = artutil.ArtKindNone
	}

	if config.AdditionalDownloads.Marquee == "" {
		config.AdditionalDownloads.Marquee = artutil.ArtKindNone
	}

	gaba.SetRawLogLevel(string(config.LogLevel))

	if err := i18n.SetWithCode(config.Language); err != nil {
		gaba.GetLogger().Error("Failed to set language", "error", err, "language", config.Language)
	}

	pretty, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		gaba.GetLogger().Error("Failed to marshal config to JSON", "error", err)
		return err
	}

	if err := os.WriteFile("config.json", pretty, 0644); err != nil {
		gaba.GetLogger().Error("Failed to write config file", "error", err)
		return err
	}

	return nil
}

func InitKidMode(config *Config) {
	kidModeEnabled.Store(config.KidMode)
}

func IsKidModeEnabled() bool {
	return kidModeEnabled.Load()
}

func SetKidMode(enabled bool) {
	kidModeEnabled.Store(enabled)
}

// LoadPlatformsBinding fetches the PLATFORMS_BINDING from the RomM server
// and stores it in the config for use in CFW lookups.
// This requires the pointer receiver!
//
//goland:noinspection ALL
func (c *Config) LoadPlatformsBinding(host romm.Host, timeout ...time.Duration) error {
	client := romm.NewClientFromHost(host, timeout...)

	rommConfig, err := client.GetConfig()
	if err != nil {
		// Non-fatal - older RomM versions may not have this endpoint
		return err
	}

	c.PlatformsBinding = rommConfig.PlatformsBinding
	return nil
}

func (c Config) GetDirectoryMapping(fsSlug string) (string, bool) {
	if mapping, ok := c.DirectoryMappings[fsSlug]; ok {
		return mapping.RelativePath, true
	}
	return "", false
}

func LoadSlotPreferences() map[string]string {
	data, err := os.ReadFile("save_slots.json")
	if err != nil {
		return nil
	}
	var prefs map[string]string
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil
	}
	return prefs
}

func SaveSlotPreferences(config *Config) error {
	if len(config.SlotPreferences) == 0 {
		os.Remove("save_slots.json")
		return nil
	}
	pretty, err := json.MarshalIndent(config.SlotPreferences, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("save_slots.json", pretty, 0644)
}

func (c Config) GetSlotPreference(romID int) string {
	if c.SlotPreferences != nil {
		if slot, ok := c.SlotPreferences[fmt.Sprintf("%d", romID)]; ok {
			return slot
		}
	}
	return "default"
}

func (c *Config) SetSlotPreference(romID int, slot string) {
	if c.SlotPreferences == nil {
		c.SlotPreferences = make(map[string]string)
	}
	key := fmt.Sprintf("%d", romID)
	if slot == "default" {
		delete(c.SlotPreferences, key)
	} else {
		c.SlotPreferences[key] = slot
	}
}

func (c Config) GetApiTimeout() time.Duration    { return c.ApiTimeout.Duration() }
func (c Config) GetShowCollections() bool        { return c.ShowRegularCollections }
func (c Config) GetShowSmartCollections() bool   { return c.ShowSmartCollections }
func (c Config) GetShowVirtualCollections() bool { return c.ShowVirtualCollections }

// ResolveFSSlug returns the effective fs_slug for CFW lookups.
// If the fs_slug has a binding in PlatformsBinding, the bound value is returned.
// Otherwise, the original fs_slug is returned.
// Example: PlatformsBinding {"ms": "sms"} means RomM "ms" -> CFW "sms"
// So ResolveFSSlug("ms") returns "sms"
func (c Config) ResolveFSSlug(fsSlug string) string {
	if c.PlatformsBinding != nil {
		if bound, ok := c.PlatformsBinding[fsSlug]; ok {
			gaba.GetLogger().Debug("Using platform binding for CFW lookup",
				"fsSlug", fsSlug, "boundTo", bound)
			return bound
		}
	}
	return fsSlug
}

// ResolveRommFSSlug returns the RomM fs_slug for a given CFW platform key.
// This is the inverse of ResolveFSSlug - it finds which RomM fs_slug maps TO the given CFW key.
// Example: PlatformsBinding {"ms": "sms"} means RomM "ms" -> CFW "sms"
// So ResolveRommFSSlug("sms") returns "ms"
func (c Config) ResolveRommFSSlug(cfwKey string) string {
	if c.PlatformsBinding != nil {
		for rommSlug, cfwSlug := range c.PlatformsBinding {
			if cfwSlug == cfwKey {
				gaba.GetLogger().Debug("Using inverse platform binding",
					"cfwKey", cfwKey, "rommFSSlug", rommSlug)
				return rommSlug
			}
		}
	}
	return cfwKey
}

func (c Config) GetPlatformRomDirectory(platform romm.Platform) string {
	rp := platform.FSSlug
	if mapping, ok := c.DirectoryMappings[platform.FSSlug]; ok && mapping.RelativePath != "" {
		rp = mapping.RelativePath
	}
	effectiveFSSlug := c.ResolveFSSlug(platform.FSSlug)
	return cfw.GetPlatformRomDirectory(rp, effectiveFSSlug)
}

func (c Config) GetArtDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtPreviewDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtPreviewDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtSplashDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtSplashDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtMarqueeDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtMarqueeDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtVideoDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtVideoDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtThumbnailDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtThumbnailDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetArtBezelDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetArtBezelDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetManualDirectory(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetManualDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetFanartDirectoru(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetFanartDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) GetBoxbackDirectoru(platform romm.Platform) string {
	romDir := c.GetPlatformRomDirectory(platform)
	return cfw.GetBoxbackDirectory(romDir, platform.FSSlug, platform.Name)
}

func (c Config) ShowCollections(host romm.Host) bool {
	if !c.ShowRegularCollections && !c.ShowSmartCollections && !c.ShowVirtualCollections {
		return false
	}

	// Check cache first
	if cm := cache.GetCacheManager(); cm != nil && cm.HasCollections() {
		return true
	}

	// Fallback to network check
	rc := romm.NewClientFromHost(host, c.ApiTimeout.Duration())

	if c.ShowRegularCollections {
		col, err := rc.GetCollections()
		if err == nil && len(col) > 0 {
			return true
		}
	}

	if c.ShowSmartCollections {
		smartCol, err := rc.GetSmartCollections()
		if err == nil && len(smartCol) > 0 {
			return true
		}
	}

	if c.ShowVirtualCollections {
		virtualCol, err := rc.GetVirtualCollections()
		if err == nil && len(virtualCol) > 0 {
			return true
		}
	}

	return false
}
