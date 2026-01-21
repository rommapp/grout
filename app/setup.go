package main

import (
	"errors"
	"grout/cache"
	"grout/cfw"
	"grout/cfw/knulli"
	"grout/cfw/muos"
	"grout/internal"
	"grout/internal/environment"
	"grout/internal/fileutil"
	"grout/resources"
	"grout/romm"
	"grout/ui"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SetupResult struct {
	Config    *internal.Config
	Platforms []romm.Platform
}

func setup() SetupResult {
	currentCFW := cfw.GetCFW()
	gaba.SetLogFilename("grout.log")

	if currentCFW == cfw.MuOS && !environment.IsDevelopment() {
		if cwd, err := os.Getwd(); err == nil {
			cwdMappingPath := filepath.Join(cwd, "input_mapping.json")
			if fileutil.FileExists(cwdMappingPath) {
				os.Setenv("INPUT_MAPPING_PATH", cwdMappingPath)
			} else {
				mappingBytes, err := muos.GetInputMappingBytes()
				if err == nil {
					gaba.SetInputMappingBytes(mappingBytes)
				} else {
					gaba.GetLogger().Error("Unable to read input mapping file", "error", err)
				}
			}
		}
	}

	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             currentCFW == cfw.NextUI,
	})

	gaba.RegisterChord("unlock-kid-mode", []buttons.VirtualButton{
		buttons.VirtualButtonL1,
		buttons.VirtualButtonR1,
		buttons.VirtualButtonMenu,
	}, gaba.ChordOptions{
		Window: time.Millisecond * 1500,
		OnTrigger: func() {
			if internal.IsKidModeEnabled() {
				internal.SetKidMode(false)
				gaba.GetLogger().Info("Kid Mode unlocked for this session")
			}
		},
	})

	gaba.SetLogLevel(slog.LevelDebug)
	logger := gaba.GetLogger()

	localeFiles, err := resources.GetLocaleMessageFiles()
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Failed to load locale files: %v", err)
	}
	if err := i18n.InitI18NFromBytes(localeFiles); err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Failed to initialize i18n: %v", err)
	}

	if cfw.GetCFW() == cfw.Knulli {
		knulli.AddToToolsGameList(cfw.GetRomDirectory())
	}

	config, err := internal.LoadConfig()
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

		if err := i18n.SetWithCode(selectedLanguage); err != nil {
			logger.Error("Failed to set language", "error", err, "language", selectedLanguage)
		}

		if config == nil {
			config = &internal.Config{
				ShowRegularCollections: true,
				ApiTimeout:             30 * time.Minute,
				DownloadTimeout:        60 * time.Minute,
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
			log.SetOutput(os.Stderr)
			log.Fatalf("Login failed: %v", loginErr)
		}
		logger.Debug("Login successful, saving configuration")
		config.Hosts = loginConfig.Hosts
		config.PlatformsBinding = loginConfig.PlatformsBinding
		internal.SaveConfig(config)
	}

	if config.LogLevel != "" {
		gaba.SetRawLogLevel(string(config.LogLevel))
	}

	if config.Language != "" && !isFirstLaunch {
		if err := i18n.SetWithCode(config.Language); err != nil {
			logger.Error("Failed to set language", "error", err, "language", config.Language)
		}
	}

	internal.InitKidMode(config)

	if internal.IsKidModeEnabled() {
		splashBytes, _ := resources.GetSplashImageBytes()
		gaba.ProcessMessage("", gaba.ProcessMessageOptions{
			ImageBytes:   splashBytes,
			ImageWidth:   768,
			ImageHeight:  540,
			ProcessInput: true,
		}, func() (interface{}, error) {
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				if !internal.IsKidModeEnabled() {
					break
				}
			}
			return nil, nil
		})
	}

	gaba.UnregisterCombo("unlock-kid-mode")

	if len(config.DirectoryMappings) == 0 {
		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:             config.Hosts[0],
			ApiTimeout:       config.ApiTimeout,
			CFW:              currentCFW,
			RomDirectory:     cfw.GetRomDirectory(),
			AutoSelect:       false,
			HideBackButton:   true,
			PlatformsBinding: config.PlatformsBinding,
		})

		if err == nil && result.Action == ui.PlatformMappingActionSaved {
			config.DirectoryMappings = result.Mappings
			internal.SaveConfig(config)
		}
	}

	logger.Debug("Configuration Loaded!", "config", config.ToLoggable())

	// Initialize cache manager early so platforms can be loaded from cache
	if err := cache.InitCacheManager(config.Hosts[0], config); err != nil {
		logger.Error("Failed to initialize cache manager", "error", err)
	}

	var platforms []romm.Platform
	var loadErr error

	splashBytes, _ := resources.GetSplashImageBytes()

	for {
		gaba.ProcessMessage("", gaba.ProcessMessageOptions{
			ImageBytes:  splashBytes,
			ImageWidth:  768,
			ImageHeight: 540,
		}, func() (interface{}, error) {
			// Load platform bindings from RomM server (non-fatal if it fails)
			if err := config.LoadPlatformsBinding(config.Hosts[0], config.ApiTimeout); err != nil {
				logger.Debug("Failed to load platform bindings", "error", err)
			}

			var err error
			platforms, err = internal.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings, config.ApiTimeout)
			if err != nil {
				loadErr = err
				return nil, err
			}
			loadErr = nil
			platforms = internal.SortPlatformsByOrder(platforms, config.PlatformOrder)
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

	return SetupResult{
		Config:    config,
		Platforms: platforms,
	}
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
