package models

type Config struct {
	Hosts          Hosts  `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	UnzipDownloads bool   `yaml:"unzip_downloads,omitempty" json:"unzip_downloads,omitempty"`
	DownloadArt    bool   `yaml:"download_art,omitempty" json:"download_art,omitempty"`
	GroupBinCue    bool   `yaml:"group_bin_cue,omitempty" json:"group_bin_cue,omitempty"`
	GroupMultiDisc bool   `yaml:"group_multi_disc,omitempty" json:"group_multi_disc,omitempty"`
	LogLevel       string `yaml:"log_level,omitempty" json:"log_level,omitempty"`
}
