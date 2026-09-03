package internal

import (
	"encoding/json"
	"fmt"
	"grout/cfw"
	"grout/domain/library"
	"grout/internal/fileutil"
	"grout/romm"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

// kidModeEnabled is the session's kid mode, which starts from Config.KidMode
// but can be unlocked for this run without persisting the change. The chord
// handler in app clears it; the stored setting is untouched, so the next launch
// is locked again.
var kidModeEnabled atomic.Bool

type AdditionalDownloads struct {
	Marquee   library.ArtKind `json:"marquee,omitempty"`
	Video     bool            `json:"video,omitempty"`
	Thumbnail library.ArtKind `json:"thumbnail,omitempty"`
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
	ArtKind                      library.ArtKind             `json:"art_kind,omitempty"`
	DownloadArtScreenshotPreview bool                        `json:"download_art_screenshot_preview,omitempty"`
	DownloadSplashArt            library.ArtKind             `json:"download_splash_art,omitempty"`
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

// ConfigFileName is where settings live, relative to the working directory the
// launch script starts grout in.
const ConfigFileName = "config.json"

// applyDefaults fills in every unset field.
//
// Load and save both call this. When they each had their own list they
// disagreed: log level and release channel were only defaulted on save, so a
// config file without them loaded with an empty log level and never applied
// one until something happened to write the file back.
func applyDefaults(config *Config) {
	if config.ApiTimeout == 0 {
		config.ApiTimeout = DurationSeconds(30 * time.Second)
	}
	// The settings picker only offers 15s to 300s, so a larger stored value
	// cannot be represented and is reset rather than shown wrong.
	if config.ApiTimeout.Duration() > 300*time.Second {
		config.ApiTimeout = DurationSeconds(30 * time.Second)
	}
	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = DurationSeconds(60 * time.Minute)
	}
	if config.Language == "" {
		config.Language = "en"
	}
	if config.LogLevel == "" {
		config.LogLevel = LogLevelError
	}
	if config.ReleaseChannel == "" {
		config.ReleaseChannel = ReleaseChannelMatchRomM
	}
	if config.DownloadedGames == "" {
		config.DownloadedGames = DownloadedGamesModeDoNothing
	}
	if config.CollectionView == "" {
		config.CollectionView = CollectionViewPlatform
	}
	if config.ArtKind == "" {
		config.ArtKind = library.ArtKindDefault
	}
	if config.AdditionalDownloads.Thumbnail == "" {
		config.AdditionalDownloads.Thumbnail = library.ArtKindNone
	}
	if config.AdditionalDownloads.Marquee == "" {
		config.AdditionalDownloads.Marquee = library.ArtKindNone
	}
}

func LoadConfig() (*Config, error) {
	return LoadConfigFrom(ConfigFileName)
}

// LoadConfigFrom reads settings from path, filling in defaults for anything
// absent.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	applyDefaults(&config)
	config.SlotPreferences = loadSlotPreferencesFrom(slotPreferencesPathFor(path))

	return &config, nil
}

// SaveConfig writes settings and applies the ones that take effect
// immediately.
func SaveConfig(config *Config) error {
	if err := SaveConfigTo(config, ConfigFileName); err != nil {
		return err
	}
	ApplyRuntimeSettings(config)
	return nil
}

// ApplyRuntimeSettings pushes the settings that change how the running app
// behaves into the UI toolkit.
//
// Separate from SaveConfigTo because it needs an initialised toolkit, and
// writing a settings file should not.
func ApplyRuntimeSettings(config *Config) {
	gaba.SetRawLogLevel(string(config.LogLevel))
	if err := i18n.SetWithCode(config.Language); err != nil {
		gaba.GetLogger().Error("Failed to set language", "error", err, "language", config.Language)
	}
}

// SaveConfigTo writes settings to path.
func SaveConfigTo(config *Config, path string) error {
	applyDefaults(config)

	pretty, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}

	// Written atomically: a truncated config.json fails to parse, and a failed
	// parse is treated as a first launch, which discards the user's hosts,
	// credentials and directory mappings.
	if err := fileutil.WriteFileAtomic(path, pretty, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// InitKidMode seeds the session from the stored setting.
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

func (c Config) GetDirectoryMapping(fsSlug string) (string, bool) {
	if mapping, ok := c.DirectoryMappings[fsSlug]; ok {
		return mapping.RelativePath, true
	}
	return "", false
}

// SlotPreferencesFileName holds the per-rom save slot choices, kept out of
// config.json so a settings write cannot lose them and vice versa.
const SlotPreferencesFileName = "save_slots.json"

// slotPreferencesPathFor puts the slot file beside the config it belongs to.
func slotPreferencesPathFor(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), SlotPreferencesFileName)
}

func LoadSlotPreferences() map[string]string {
	return loadSlotPreferencesFrom(SlotPreferencesFileName)
}

func loadSlotPreferencesFrom(path string) map[string]string {
	data, err := os.ReadFile(path)
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
	return SaveSlotPreferencesTo(config, SlotPreferencesFileName)
}

// SaveSlotPreferencesTo writes the slot choices to path, removing the file when
// there are none left to record.
func SaveSlotPreferencesTo(config *Config, path string) error {
	if len(config.SlotPreferences) == 0 {
		os.Remove(path)
		return nil
	}
	pretty, err := json.MarshalIndent(config.SlotPreferences, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, pretty, 0644)
}

func (c Config) GetSlotPreference(romID int) string {
	if c.SlotPreferences != nil {
		if slot, ok := c.SlotPreferences[fmt.Sprintf("%d", romID)]; ok {
			return slot
		}
	}
	return "autosave"
}

// SlotPreferenceExplicit returns the user-set slot preference for a ROM and whether
// one was explicitly set (vs. the implicit "autosave" default). Lets an explicit user
// choice win over a recorded last-synced slot.
func (c Config) SlotPreferenceExplicit(romID int) (string, bool) {
	if c.SlotPreferences != nil {
		if slot, ok := c.SlotPreferences[fmt.Sprintf("%d", romID)]; ok {
			return slot, true
		}
	}
	return "", false
}

func (c *Config) SetSlotPreference(romID int, slot string) {
	if c.SlotPreferences == nil {
		c.SlotPreferences = make(map[string]string)
	}
	key := fmt.Sprintf("%d", romID)
	if slot == "" {
		// An empty slot means "no explicit choice", so clear any stored preference so the
		// recorded/last-synced slot (or the autosave default) applies again.
		delete(c.SlotPreferences, key)
		return
	}
	// Persist every explicit choice, INCLUDING "autosave". This lets an explicit autosave
	// selection override a sticky recorded slot via SlotPreferenceExplicit (issue #250);
	// previously "autosave" was discarded, so a recorded "default" could never be undone.
	c.SlotPreferences[key] = slot
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

// ArtDirectory returns where this device keeps a given kind of artwork for a
// platform, or "" when the firmware keeps none. Callers must treat "" as
// "skip this kind" rather than as an error.
func (c Config) ArtDirectory(platform romm.Platform, slot cfw.ArtSlot) string {
	return cfw.ArtDirectory(cfw.GetCFW(), slot, c.GetPlatformRomDirectory(platform), platform.FSSlug, platform.Name)
}
