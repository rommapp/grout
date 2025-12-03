package models

type Platform struct {
	Name             string `yaml:"-" json:"-"`
	RomMPlatformID   string `yaml:"-" json:"-"`
	RomMPlatformSlug string `yaml:"-" json:"-"`

	Host Host `json:"-"`
}

type Platforms []Platform
