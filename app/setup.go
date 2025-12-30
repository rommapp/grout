package main

import (
	"errors"
	"fmt"
	"grout/constants"
	"grout/constants/cfw/muos"
	"grout/resources"
	"grout/romm"
	"grout/ui"
	"grout/utils"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SetupResult struct {
	Config    *utils.Config
	Platforms []romm.Platform
}

func classifyStartupError(err error) *goi18n.Message {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, romm.ErrInvalidHostname):
		return &goi18n.Message{ID: "startup_error_invalid_hostname", Other: "Could not resolve hostname!\nPlease check your server configuration."}
	case errors.Is(err, romm.ErrConnectionRefused):
		return &goi18n.Message{ID: "startup_error_connection_refused", Other: "Could not connect to RomM!\nPlease check the server is running."}
	case errors.Is(err, romm.ErrTimeout):
		return &goi18n.Message{ID: "startup_error_timeout", Other: "Connection timed out!\nPlease check your network connection."}
	case errors.Is(err, romm.ErrWrongProtocol):
		return &goi18n.Message{ID: "startup_error_wrong_protocol", Other: "Protocol mismatch!\nCheck your server configuration."}
	case errors.Is(err, romm.ErrUnauthorized):
		return &goi18n.Message{ID: "startup_error_credentials", Other: "Invalid credentials!\nPlease check your username and password."}
	case errors.Is(err, romm.ErrForbidden):
		return &goi18n.Message{ID: "startup_error_forbidden", Other: "Access forbidden!\nCheck your server permissions."}
	case errors.Is(err, romm.ErrServerError):
		return &goi18n.Message{ID: "startup_error_server", Other: "RomM server error!\nPlease check the RomM server logs."}
	default:
		return &goi18n.Message{ID: "error_loading_platforms", Other: "Error loading platforms!\nPlease check the logs for more info."}
	}
}

func showStartupError(errorMsg string) bool {
	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "startup_error_action_exit", Other: "Exit"}, nil)},
		{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "startup_error_action_retry", Other: "Retry Connection"}, nil)},
	}

	result, err := gaba.ConfirmationMessage(errorMsg, footerItems, gaba.MessageOptions{})

	return err == nil && result != nil && result.Confirmed
}

func setup() SetupResult {
	setupStart := time.Now()

	cfw := utils.GetCFW()

	// Set up input mapping for muOS with auto-detection
	if cfw == constants.MuOS && !utils.IsDevelopment() {
		if cwd, err := os.Getwd(); err == nil {
			cwdMappingPath := filepath.Join(cwd, "input_mapping.json")
			if _, err := os.Stat(cwdMappingPath); err == nil {
				// User-provided mapping takes priority
				os.Setenv("INPUT_MAPPING_PATH", cwdMappingPath)
			} else {
				// Use embedded mapping with auto-detection
				if mappingBytes, err := muos.GetInputMappingBytes(); err == nil {
					gaba.SetInputMappingBytes(mappingBytes)
				}
			}
		}
	}

	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             cfw == constants.NextUI,
		LogFilename:          "grout.log",
	})

	gaba.SetLogLevel(slog.LevelDebug)
	logger := gaba.GetLogger()

	localeFiles, err := resources.GetLocaleMessageFiles()
	if err != nil {
		utils.LogStandardFatal("Failed to load locale files", err)
	}
	if err := i18n.InitI18NFromBytes(localeFiles); err != nil {
		utils.LogStandardFatal("Failed to initialize i18n", err)
	}

	config, err := utils.LoadConfig()
	isFirstLaunch := err != nil || (len(config.Hosts) == 0 && config.Language == "")

	if isFirstLaunch {
		logger.Debug("First launch detected, showing language selection")
		languageScreen := ui.NewLanguageSelectionScreen()
		selectedLanguage, langErr := languageScreen.Draw()
		if langErr != nil {
			logger.Error("Language selection failed", "error", langErr)
			selectedLanguage = "en"
		}
		logger.Debug("Language selected", "language", selectedLanguage)

		// Set the language immediately
		if err := i18n.SetWithCode(selectedLanguage); err != nil {
			logger.Error("Failed to set language", "error", err, "language", selectedLanguage)
		}

		if config == nil {
			config = &utils.Config{
				ShowCollections: true,
			}
		}
		config.Language = selectedLanguage
	}

	if err != nil || len(config.Hosts) == 0 {
		logger.Debug("No RomM Host Configured", "error", err)
		logger.Debug("Starting login flow for initial setup")
		loginConfig, loginErr := ui.LoginFlow(romm.Host{})
		if loginErr != nil {
			logger.Error("Login flow failed", "error", loginErr)
			utils.LogStandardFatal("Login failed", loginErr)
		}
		logger.Debug("Login successful, saving configuration")
		config.Hosts = loginConfig.Hosts
		utils.SaveConfig(config)
	}

	if config.LogLevel != "" {
		gaba.SetRawLogLevel(config.LogLevel)
	}

	if config.Language != "" && !isFirstLaunch {
		if err := i18n.SetWithCode(config.Language); err != nil {
			logger.Error("Failed to set language", "error", err, "language", config.Language)
		}
	}

	if config.DirectoryMappings == nil || len(config.DirectoryMappings) == 0 {
		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:           config.Hosts[0],
			ApiTimeout:     config.ApiTimeout,
			CFW:            cfw,
			RomDirectory:   utils.GetRomDirectory(),
			AutoSelect:     false,
			HideBackButton: true,
		})

		if err == nil && result.ExitCode == gaba.ExitCodeSuccess {
			config.DirectoryMappings = result.Value.Mappings
			utils.SaveConfig(config)
		}
	}

	logger.Debug("Configuration Loaded!", "config", config.ToLoggable())

	var platforms []romm.Platform
	var loadErr error

	splashBytes, _ := resources.GetSplashImageBytes()

	for {
		gaba.ProcessMessage("", gaba.ProcessMessageOptions{
			ImageBytes:  splashBytes,
			ImageWidth:  768,
			ImageHeight: 540,
		}, func() (interface{}, error) {
			var err error
			platforms, err = utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings)
			if err != nil {
				loadErr = err
				return nil, err
			}
			loadErr = nil
			platforms = utils.SortPlatformsByOrder(platforms, config.PlatformOrder)
			return nil, nil
		})

		if loadErr == nil {
			break
		}

		logger.Error("Failed to load platforms", "error", loadErr)
		errorMessage := classifyStartupError(loadErr)
		errorMsg := i18n.Localize(errorMessage, nil)

		retry := showStartupError(errorMsg)
		if !retry {
			logger.Info("User chose to quit after startup error")
			gaba.Close()
			os.Exit(1)
		}
		logger.Info("User chose to retry connection")
	}

	logger.Info("Setup complete", "totalSeconds", fmt.Sprintf("%.2f", time.Since(setupStart).Seconds()))

	return SetupResult{
		Config:    config,
		Platforms: platforms,
	}
}
