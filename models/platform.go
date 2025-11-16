package models

type Platform struct {
	Name           string `yaml:"-" json:"-"`
	LocalDirectory string `yaml:"-" json:"-"`
	RomMPlatformID string `yaml:"-" json:"-"`

	Host Host `yaml:"-" json:"-"`
}

type Platforms []Platform
