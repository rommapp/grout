package models

import (
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"qlova.tech/sum"
)

type Host struct {
	DisplayName string                   `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	RootURI     string                   `yaml:"root_uri,omitempty" json:"root_uri,omitempty"`
	Port        int                      `yaml:"port,omitempty" json:"port,omitempty"`

	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	Platforms Platforms `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Filters   Filters   `yaml:"filters,omitempty" json:"filters,omitempty"`

	TableColumns       shared.TableColumns `yaml:"-" json:"-"`
	SourceReplacements SourceReplacements  `yaml:"-" json:"-"`
}

func (h Host) Value() interface{} {
	return h
}

type Hosts []Host

type Filters struct {
	InclusiveFilters []string `yaml:"inclusive_filters,omitempty" json:"inclusive_filters,omitempty"`
	ExclusiveFilters []string `yaml:"exclusive_filters,omitempty" json:"exclusive_filters,omitempty"`
}

type SourceReplacements map[string]string
