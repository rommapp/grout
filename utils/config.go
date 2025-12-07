package utils

import (
	"encoding/json"
	"fmt"
	"grout/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func LoadConfig() (*models.Config, error) {
	configFiles := []string{"config.json", "config.yml"}

	var data []byte
	var err error
	var foundFile string

	for _, filename := range configFiles {
		data, err = os.ReadFile(filename)
		if err == nil {
			foundFile = filename
			break
		}
	}

	if foundFile == "" {
		return nil, fmt.Errorf("no config file found (tried: %s)", strings.Join(configFiles, ", "))
	}

	var config models.Config

	ext := strings.ToLower(filepath.Ext(foundFile))

	switch ext {
	case ".json":
		err = json.Unmarshal(data, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &config)
	default:
		return nil, fmt.Errorf("unknown config file type: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", foundFile, err)
	}

	if ext == ".yaml" || ext == ".yml" {
		gaba.GetLogger().Info("Migrating config to JSON")
		_ = SaveConfig(&config)
	}

	if config.ApiTimeout == 0 {
		config.ApiTimeout = 30 * time.Minute
	}

	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = 60 * time.Minute
	}

	return &config, nil
}

func SaveConfig(config *models.Config) error {
	configFiles := []string{"config.json", "config.yml"}

	var existingFile string
	var configType string

	for _, filename := range configFiles {
		if _, err := os.Stat(filename); err == nil {
			existingFile = filename
			ext := strings.ToLower(filepath.Ext(filename))
			switch ext {
			case ".json":
				configType = "json"
			case ".yml":
				configType = "yml"
			}
			break
		}
	}

	if existingFile == "" {
		existingFile = "config.json"
		configType = "json"
	}

	viper.SetConfigName(strings.TrimSuffix(filepath.Base(existingFile), filepath.Ext(existingFile)))
	viper.SetConfigType(configType)
	viper.AddConfigPath(".")

	if config.LogLevel == "" {
		config.LogLevel = "ERROR"
	}

	viper.Set("hosts", config.Hosts)
	viper.Set("directory_mappings", config.DirectoryMappings)
	viper.Set("download_art", config.DownloadArt)
	viper.Set("show_game_details", config.ShowGameDetails)
	viper.Set("api_timeout", config.ApiTimeout)
	viper.Set("download_timeout", config.DownloadTimeout)
	viper.Set("log_level", config.LogLevel)

	gaba.SetRawLogLevel(config.LogLevel)

	newConfig := viper.AllSettings()

	pretty, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		gaba.GetLogger().Error("Failed to marshal config to JSON", "error", err)
		return err
	}

	err = os.WriteFile("config.json", pretty, 0644)
	if err != nil {
		gaba.GetLogger().Error("Failed to write config file", "error", err)
		return err
	}

	_ = os.Remove("config.yml")

	return nil
}
