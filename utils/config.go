package utils

import (
	"encoding/json"
	"fmt"
	"grout/romm"
	"os"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

type Config struct {
	Hosts                  []romm.Host                 `json:"hosts,omitempty"`
	DirectoryMappings      map[string]DirectoryMapping `json:"directory_mappings,omitempty"`
	SaveSyncMode           string                      `json:"save_sync_mode"`
	SaveDirectoryMappings  map[string]string           `json:"save_directory_mappings,omitempty"`
	GameSaveOverrides      map[int]string              `json:"game_save_overrides,omitempty"`
	DownloadArt            bool                        `json:"download_art,omitempty"`
	ShowBoxArt             bool                        `json:"show_box_art,omitempty"`
	UnzipDownloads         bool                        `json:"unzip_downloads,omitempty"`
	ShowCollections        bool                        `json:"show_collections"`
	ShowSmartCollections   bool                        `json:"show_smart_collections"`
	ShowVirtualCollections bool                        `json:"show_virtual_collections"`
	DownloadedGames        string                      `json:"downloaded_games,omitempty"`
	ApiTimeout             time.Duration               `json:"api_timeout"`
	DownloadTimeout        time.Duration               `json:"download_timeout"`
	LogLevel               string                      `json:"log_level,omitempty"`
	Language               string                      `json:"language,omitempty"`
	CollectionView         string                      `json:"collection_view,omitempty"`
	KidMode                bool                        `json:"kid_mode,omitempty"`

	PlatformOrder []string `json:"platform_order,omitempty"`
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
		"show_box_art":            c.ShowBoxArt,
		"save_directory_mappings": c.SaveDirectoryMappings,
		"game_save_overrides":     c.GameSaveOverrides,
		"collections":             c.ShowCollections,
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
		config.ApiTimeout = 30 * time.Minute
	}

	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = 60 * time.Minute
	}

	if config.Language == "" {
		config.Language = "en"
	}

	if config.DownloadedGames == "" {
		config.DownloadedGames = "do_nothing"
	}

	if config.CollectionView == "" {
		config.CollectionView = "platform"
	}

	if config.SaveSyncMode == "" {
		config.SaveSyncMode = "off"
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	if config.LogLevel == "" {
		config.LogLevel = "ERROR"
	}

	if config.Language == "" {
		config.Language = "en"
	}

	if config.DownloadedGames == "" {
		config.DownloadedGames = "do_nothing"
	}

	if config.CollectionView == "" {
		config.CollectionView = "platform"
	}

	if config.SaveSyncMode == "" {
		config.SaveSyncMode = "off"
	}

	gaba.SetRawLogLevel(config.LogLevel)

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

// SortPlatformsByOrder sorts platforms based on the saved order in config.
// If no order is saved, platforms are sorted alphabetically.
func SortPlatformsByOrder(platforms []romm.Platform, order []string) []romm.Platform {
	if len(order) == 0 {
		// No saved order, return alphabetically sorted
		return SortPlatformsAlphabetically(platforms)
	}

	// Create a map of slug to platform for quick lookup
	platformMap := make(map[string]romm.Platform)
	for _, p := range platforms {
		platformMap[p.Slug] = p
	}

	// Create result slice with platforms in saved order
	var result []romm.Platform
	usedSlugs := make(map[string]bool)

	// Add platforms in saved order
	for _, slug := range order {
		if platform, exists := platformMap[slug]; exists {
			result = append(result, platform)
			usedSlugs[slug] = true
		}
	}

	// Add any new platforms that aren't in the saved order (alphabetically)
	var newPlatforms []romm.Platform
	for _, p := range platforms {
		if !usedSlugs[p.Slug] {
			newPlatforms = append(newPlatforms, p)
		}
	}
	newPlatforms = SortPlatformsAlphabetically(newPlatforms)
	result = append(result, newPlatforms...)

	return result
}

// SortPlatformsAlphabetically sorts platforms by name
func SortPlatformsAlphabetically(platforms []romm.Platform) []romm.Platform {
	sorted := make([]romm.Platform, len(platforms))
	copy(sorted, platforms)

	// Simple bubble sort by name
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Name > sorted[j].Name {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// PrunePlatformOrder removes platform slugs from the order that are no longer in the directory mappings.
// This ensures the platform order stays synchronized with available platforms.
func PrunePlatformOrder(order []string, mappings map[string]DirectoryMapping) []string {
	if len(order) == 0 {
		return order
	}

	pruned := make([]string, 0, len(order))
	for _, slug := range order {
		if _, exists := mappings[slug]; exists {
			pruned = append(pruned, slug)
		}
	}

	return pruned
}
