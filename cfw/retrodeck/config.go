package retrodeck

import (
	"encoding/json"
	"fmt"
	"os"
)

const configPathEnv = "RETRODECK_CFG"

// Paths holds the subset of RetroDECK path configuration relevant to Grout.
type Paths struct {
	RDHomePath          string `json:"rd_home_path"`
	RomsPath            string `json:"roms_path"`
	SavesPath           string `json:"saves_path"`
	BiosPath            string `json:"bios_path"`
	DownloadedMediaPath string `json:"downloaded_media_path"`
	VideosPath          string `json:"videos_path"`
}

type retrodeckConfig struct {
	Paths Paths `json:"paths"`
}

// LoadConfig reads the RetroDECK config file from the path set in RETRODECK_CFG.
func LoadConfig() (*Paths, error) {
	cfgPath := os.Getenv(configPathEnv)
	if cfgPath == "" {
		return nil, fmt.Errorf("%s environment variable not set", configPathEnv)
	}
	return ParseConfig(cfgPath)
}

// ParseConfig reads and parses the RetroDECK config file at the given path.
func ParseConfig(path string) (*Paths, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading retrodeck config: %w", err)
	}

	var cfg retrodeckConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing retrodeck config: %w", err)
	}

	return &cfg.Paths, nil
}
