package models

type Platform struct {
	Name           string `yaml:"platform_name,omitempty" json:"platform_name,omitempty"`
	SystemTag      string `yaml:"system_tag,omitempty" json:"system_tag,omitempty"`
	LocalDirectory string `yaml:"local_directory,omitempty" json:"local_directory,omitempty"`
	RomMPlatformID string `yaml:"romm_platform_id,omitempty" json:"romm_platform_id,omitempty"`

	Host Host `yaml:"-" json:"-"`
}

type Platforms []Platform
