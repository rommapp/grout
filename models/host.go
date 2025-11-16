package models

type Host struct {
	DisplayName string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	RootURI     string `yaml:"root_uri,omitempty" json:"root_uri,omitempty"`
	Port        int    `yaml:"port,omitempty" json:"port,omitempty"`

	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	Platforms Platforms `yaml:"-" json:"-"`
}
type Hosts []Host
