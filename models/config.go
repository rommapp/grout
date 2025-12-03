package models

import (
	"strings"
	"time"
)

type Config struct {
	Hosts              Hosts                       `json:"hosts,omitempty"`
	DirectoryMappings  map[string]DirectoryMapping `json:"directory_mappings,omitempty"`
	ApiTimeout         time.Duration               `json:"api_timeout"`
	DownloadTimeout    time.Duration               `json:"download_timeout"`
	UseTitleAsFilename bool                        `json:"use_title_as_filename"`
	UnzipDownloads     bool                        `json:"unzip_downloads,omitempty"`
	DownloadArt        bool                        `json:"download_art,omitempty"`
	GroupBinCue        bool                        `json:"group_bin_cue,omitempty"`
	GroupMultiDisc     bool                        `json:"group_multi_disc,omitempty"`
	LogLevel           string                      `json:"log_level,omitempty"`
}

func (c Config) ToLoggable() any {
	safeHosts := make([]map[string]any, len(c.Hosts))
	for i, host := range c.Hosts {
		safeHosts[i] = map[string]any{
			"display_name": host.DisplayName,
			"root_uri":     host.RootURI,
			"port":         host.Port,
			"username":     host.Username,
			"password":     strings.Repeat("*", len(host.Password)),
			"platforms":    host.Platforms,
		}
	}

	return map[string]any{
		"hosts":                 safeHosts,
		"directory_mappings":    c.DirectoryMappings,
		"api_timeout":           c.ApiTimeout,
		"download_timeout":      c.DownloadTimeout,
		"use_title_as_filename": c.UseTitleAsFilename,
		"unzip_downloads":       c.UnzipDownloads,
		"download_art":          c.DownloadArt,
		"group_bin_cue":         c.GroupBinCue,
		"group_multi_disc":      c.GroupMultiDisc,
		"log_level":             c.LogLevel,
	}
}
