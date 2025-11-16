package utils

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"grout/client"
	"grout/models"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func init() {}

func IsDev() bool {
	return os.Getenv("ENVIRONMENT") == "DEV"
}

func GetRomDirectory() string {
	if IsDev() || os.Getenv("ROM_DIRECTORY") != "" {
		return os.Getenv("ROM_DIRECTORY")
	}

	return common.RomDirectory
}

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
		gaba.GetLoggerInstance().Info("Migrating config to JSON")
		_ = SaveConfig(&config)
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

	viper.Set("hosts", config.Hosts)
	viper.Set("download_art", config.DownloadArt)
	viper.Set("unzip_downloads", config.UnzipDownloads)
	viper.Set("group_bin_cue", config.GroupBinCue)
	viper.Set("group_multi_disc", config.GroupMultiDisc)
	viper.Set("log_level", config.LogLevel)

	gaba.SetRawLogLevel(config.LogLevel)

	newConfig := viper.AllSettings()

	pretty, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		gaba.GetLoggerInstance().Error("Failed to marshal config to JSON", "error", err)
		return err
	}

	err = os.WriteFile("config.json", pretty, 0644)
	if err != nil {
		gaba.GetLoggerInstance().Error("Failed to write config file", "error", err)
		return err
	}

	_ = os.Remove("config.yml")

	return nil
}

func MapPlatforms(host models.Host, directories shared.Items) []models.Platform {
	cfw := os.Getenv("CFW")
	var mapping map[string][]string

	switch strings.ToLower(cfw) {
	case "muos":
		mapping = muOSPlatforms
	case "nextui":
		mapping = NextUIPlatforms
	default:
		common.LogStandardFatal(fmt.Sprintf("Unsupported CFW: %s", cfw), nil)
	}

	c := client.NewRomMClient(host)

	rommPlatforms, err := c.GetPlatforms()
	if err != nil {
		common.LogStandardFatal(fmt.Sprintf("Failed to get platforms from RomM: %s", err), nil)
	}

	slugToRomMPlatform := make(map[string]client.RomMPlatform)
	for _, platform := range rommPlatforms {
		slugToRomMPlatform[platform.Slug] = platform
	}

	var platforms models.Platforms

	for _, directory := range directories {
		var key string

		switch strings.ToLower(cfw) {
		case "muos":
			key = directory.Filename
		case "nextui":
			key = directory.Tag
		}

		slugs, ok := mapping[key]
		if ok {
			for _, slug := range slugs {
				rommPlatform, ok := slugToRomMPlatform[slug]
				if ok {
					platforms = append(platforms, models.Platform{
						Name:           rommPlatform.DisplayName,
						LocalDirectory: directory.Path,
						RomMPlatformID: strconv.Itoa(rommPlatform.ID),
						Host:           host,
					})
				}
			}
		}
	}

	return platforms
}

func UnzipGame(platform models.Platform, game shared.Item) ([]string, error) {
	logger := gaba.GetLoggerInstance()

	zipPath := filepath.Join(platform.LocalDirectory, game.Filename)
	romDirectory := platform.LocalDirectory

	if IsDev() {
		romDirectory = strings.ReplaceAll(platform.LocalDirectory, common.RomDirectory, GetRomDirectory())
		zipPath = filepath.Join(romDirectory, game.Filename)
	}

	extractedFiles, err := gaba.ProcessMessage(fmt.Sprintf("%s %s...", "Unzipping", game.DisplayName), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		extractedFiles, err := Unzip(zipPath, romDirectory)
		if err != nil {
			return nil, err
		}

		logger.Debug("Extracted files", "files", extractedFiles)

		return extractedFiles, nil
	})

	if err != nil {
		gaba.ProcessMessage(fmt.Sprintf("Unable to unzip %s", game.DisplayName), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
			time.Sleep(3 * time.Second)
			return nil, nil
		})
		logger.Error("Unable to unzip pak", "error", err)
		return nil, err
	} else {
		deleted := common.DeleteFile(zipPath)
		if !deleted {
			return nil, errors.New("unable to delete zip file")
		}
	}

	return extractedFiles.Result.([]string), nil
}

func Unzip(src, dest string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	err = os.MkdirAll(dest, 0755)
	if err != nil {
		return nil, err
	}

	extractedFiles := []string{}

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(path, f.Mode())
			if err != nil {
				return err
			}
		} else {
			err := os.MkdirAll(filepath.Dir(path), f.Mode())
			if err != nil {
				return err
			}

			tempPath := path + ".tmp"
			tempFile, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			_, err = io.Copy(tempFile, rc)
			tempFile.Close() // Close the file before attempting to rename it

			if err != nil {
				common.DeleteFile(tempPath)
				return err
			}

			// Now rename the temporary file to the target path
			err = os.Rename(tempPath, path)
			if err != nil {
				common.DeleteFile(tempPath)
				return err
			}

			extractedFiles = append(extractedFiles, path)
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return extractedFiles, err
		}
	}

	return extractedFiles, nil
}

func ListZipContents(platform models.Platform, game shared.Item) ([]string, error) {
	zipPath := filepath.Join(platform.LocalDirectory, game.Filename)
	romDirectory := platform.LocalDirectory

	if IsDev() {
		romDirectory = strings.ReplaceAll(platform.LocalDirectory, common.RomDirectory, GetRomDirectory())
		zipPath = filepath.Join(romDirectory, game.Filename)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	filenames := make([]string, 0, len(reader.File))

	for _, file := range reader.File {
		filenames = append(filenames, file.Name)
	}

	return filenames, nil
}

func HasBinCue(platform models.Platform, game shared.Item) bool {
	filenames, err := ListZipContents(platform, game)
	if err != nil {
		return false
	}
	for _, filename := range filenames {
		if strings.HasSuffix(filename, ".bin") || strings.HasSuffix(filename, ".cue") {
			return true
		}
	}

	return false
}

func IsMultiDisc(platform models.Platform, game shared.Item) bool {
	if filepath.Ext(game.Filename) == ".zip" {
		filenames, err := ListZipContents(platform, game)
		if err != nil {
			return false
		}

		for _, filename := range filenames {
			if strings.Contains(filename, "(Disc") || strings.Contains(filename, "(Disk") {
				return true
			}
		}

		return false
	}

	return strings.Contains(game.Filename, "(Disc") || strings.Contains(game.Filename, "(Disk")
}

func GroupBinCue(platform models.Platform, game shared.Item) {
	logger := gaba.GetLoggerInstance()

	unzipped, err := UnzipGame(platform, game)

	if err == nil && len(unzipped) > 0 {

		gaba.ProcessMessage(fmt.Sprintf("Grouping BIN/CUE for %s", game.DisplayName), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
			time.Sleep(1500 * time.Millisecond)
			logger.Debug("Grouping BIN / CUE ROMs")

			// Find all CUE files in the unzipped files
			cueFiles := []string{}
			for _, file := range unzipped {
				if strings.HasSuffix(file, ".cue") {
					cueFiles = append(cueFiles, file)
				}
			}

			// For each CUE file, create a directory and move related files
			for _, cueFile := range cueFiles {
				baseName := filepath.Base(cueFile)
				dirName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
				dirPath := filepath.Join(filepath.Dir(cueFile), dirName)

				// Create directory with the same name as the CUE file
				err := os.MkdirAll(dirPath, 0755)
				if err != nil {
					logger.Error("Failed to create directory for BIN/CUE grouping",
						"directory", dirPath,
						"error", err)
					continue
				}

				// Move all related files (both BIN and CUE) to the new directory
				for _, file := range unzipped {
					// Check if file is in the same directory as the CUE file
					if filepath.Dir(file) == filepath.Dir(cueFile) {
						fileBaseName := filepath.Base(file)
						// Check if it's a BIN file or the CUE file itself
						if strings.HasSuffix(file, ".bin") || file == cueFile {
							newPath := filepath.Join(dirPath, fileBaseName)
							err := os.Rename(file, newPath)
							if err != nil {
								logger.Error("Failed to move file to BIN/CUE group directory",
									"file", file,
									"destination", newPath,
									"error", err)
							}
						}
					}
				}

				logger.Debug("Successfully grouped BIN/CUE files",
					"cueFile", baseName,
					"directory", dirPath)
			}

			return nil, nil
		})
	}
}

func GroupMultiDisk(platform models.Platform, game shared.Item) error {
	logger := gaba.GetLoggerInstance()

	gameFolderName := game.DisplayName
	diskIndex := strings.Index(gameFolderName, "(Disk")
	discIndex := strings.Index(gameFolderName, "(Disc")

	trimIndex := -1
	if diskIndex != -1 && discIndex != -1 {
		trimIndex = min(diskIndex, discIndex)
	} else if diskIndex != -1 {
		trimIndex = diskIndex
	} else if discIndex != -1 {
		trimIndex = discIndex
	}

	if trimIndex != -1 {
		gameFolderName = gameFolderName[:trimIndex]
	}

	gameFolderName = strings.TrimSpace(gameFolderName)
	gameFolderPath := filepath.Join(platform.LocalDirectory, gameFolderName)

	if IsDev() {
		romDirectory := strings.ReplaceAll(platform.LocalDirectory, common.RomDirectory, GetRomDirectory())
		gameFolderPath = filepath.Join(romDirectory, gameFolderName)
	}

	if _, err := os.Stat(gameFolderPath); os.IsNotExist(err) {
		err := os.MkdirAll(gameFolderPath, 0755)
		if err != nil {
			logger.Error("Failed to create game directory", "error", err)
			return err
		}
		logger.Debug("Created new game directory", "path", gameFolderPath)
	} else {
		logger.Debug("Game directory already exists, skipping creation", "path", gameFolderPath)
	}

	var extractedFiles []string

	if filepath.Ext(game.Filename) == ".zip" {
		var err error
		extractedFiles, err = UnzipGame(platform, game)
		if err != nil {
			logger.Error("Failed to unzip game", "error", err)
			return err
		}
	} else {
		romDirectory := platform.LocalDirectory

		if IsDev() {
			romDirectory = strings.ReplaceAll(platform.LocalDirectory, common.RomDirectory, GetRomDirectory())
		}

		extractedFiles = append(extractedFiles, filepath.Join(romDirectory, game.Filename))
	}

	_, err := gaba.ProcessMessage(fmt.Sprintf("Wrangling multi-disk game %s", game.DisplayName), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		time.Sleep(1500 * time.Millisecond)
		for _, filePath := range extractedFiles {
			fileName := filepath.Base(filePath)

			destPath := filepath.Join(gameFolderPath, fileName)

			err := os.Rename(filePath, destPath)
			if err != nil {
				logger.Error("Failed to move file", "source", filePath, "destination", destPath, "error", err)
				return nil, err
			}
		}

		// Create or append to M3U file with the game's display name
		m3uFileName := fmt.Sprintf("%s.m3u", gameFolderName)
		m3uFilePath := filepath.Join(gameFolderPath, m3uFileName)

		// Find all .cue, .chd, and .pbp files in the new directory and add them to the M3U
		var discFiles []string
		for _, filePath := range extractedFiles {
			fileName := filepath.Base(filePath)
			fileNameLower := strings.ToLower(fileName)
			if strings.HasSuffix(fileNameLower, ".cue") ||
				strings.HasSuffix(fileNameLower, ".chd") ||
				strings.HasSuffix(fileNameLower, ".pbp") {
				discFiles = append(discFiles, fileName)
			}
		}

		// Check if there are any disc files to add
		if len(discFiles) > 0 {
			// Open the M3U file for appending (or create if it doesn't exist)
			m3uFile, err := os.OpenFile(m3uFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logger.Error("Failed to open M3U file", "error", err)
				return nil, err
			}
			defer m3uFile.Close()

			// Write each disc file to the M3U, one per line
			for _, discFile := range discFiles {
				_, err := m3uFile.WriteString(discFile + "\n")
				if err != nil {
					logger.Error("Failed to write to M3U file", "error", err)
					return nil, err
				}
			}

			logger.Debug("Successfully appended to M3U file",
				"m3u_path", m3uFilePath,
				"disc_files", discFiles)
		} else {
			logger.Debug("No .cue, .chd, or .pbp files found to add to M3U file")
		}

		logger.Debug("Successfully processed game",
			"folder", gameFolderPath,
			"m3u_path", m3uFilePath)

		return nil, nil
	})

	return err
}

func FindArt(platform models.Platform, game shared.Item) string {
	artDirectory := ""

	if IsDev() {
		romDirectory := strings.ReplaceAll(platform.LocalDirectory, common.RomDirectory, GetRomDirectory())
		artDirectory = filepath.Join(romDirectory, ".media")
	} else {
		artDirectory = filepath.Join(platform.LocalDirectory, ".media")
	}

	c := client.NewRomMClient(platform.Host)

	if game.ArtURL == "" {
		return ""
	}

	slashIdx := strings.LastIndex(game.ArtURL, "/")
	artSubdirectory, artFilename := game.ArtURL[:slashIdx], game.ArtURL[slashIdx+1:]

	artFilename = strings.Split(artFilename, "?")[0] // For the query string caching stuff

	LastSavedArtPath, err := c.DownloadArt(artSubdirectory,
		artDirectory, artFilename, game.Filename)

	if err != nil {
		return ""
	}

	return LastSavedArtPath
}

func IsConnectedToInternet() bool {
	timeout := 5 * time.Second
	_, err := net.DialTimeout("tcp", "8.8.8.8:53", timeout)
	return err == nil
}
