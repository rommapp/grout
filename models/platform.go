package models

type Platform struct {
	Name             string `yaml:"platform_name,omitempty" json:"platform_name,omitempty"`
	SystemTag        string `yaml:"system_tag,omitempty" json:"system_tag,omitempty"`
	LocalDirectory   string `yaml:"local_directory,omitempty" json:"local_directory,omitempty"`
	HostSubdirectory string `yaml:"host_subdirectory,omitempty" json:"host_subdirectory,omitempty"`
	RomMPlatformID   string `yaml:"romm_platform_id,omitempty" json:"romm_platform_id,omitempty"`

	SkipExclusiveFilters bool `yaml:"skip_exclusive_filters,omitempty" json:"skip_exclusive_filters,omitempty"`
	SkipInclusiveFilters bool `yaml:"skip_inclusive_filters,omitempty" json:"skip_inclusive_filters,omitempty"`

	Host Host `yaml:"-" json:"-"`
}

type Platforms []Platform
