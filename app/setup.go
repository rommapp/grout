package main

import (
	"errors"
	"grout/cache"
	"grout/catalog"
	"grout/cfw"
	"grout/environment"
	"grout/files"
	"grout/resources"
	"grout/romm"
	"grout/saves"
	"grout/settings"
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
	Config    *settings.Config
	Platforms []romm.Platform
}

func setup() SetupResult {
	// The launch script for each firmware sets this. Resolving it once here,
	// before anything else runs, is what lets the rest of the codebase treat an
	// unknown firmware as "no such directory" instead of terminating from
	// wherever it happened to notice.
	currentCFW, err := cfw.Active()
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Cannot start: %v", err)
	}

	setupInputMapping(currentCFW)
	initFramework(currentCFW)

	logger := gaba.GetLogger()

	config, isFirstLaunch := loadOrCreateConfig(logger)
	config = handleFirstLaunch(config, isFirstLaunch, logger)
	config = applyConfig(config, isFirstLaunch, currentCFW, logger)

	if err := cache.InitCacheManager(config.Hosts[0], *config); err != nil {
		logger.Error("Failed to initialize cache manager", "error", err)
	}

	platforms := connectAndLoadPlatforms(config, logger)

	return SetupResult{
		Config:    config,
		Platforms: platforms,
	}
}

func setupInputMapping(currentCFW cfw.CFW) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	cwdMappingPath := filepath.Join(cwd, "input_mapping.json")
	if files.FileExists(cwdMappingPath) {
		os.Setenv("INPUT_MAPPING_PATH", cwdMappingPath)
		return
	}

	if environment.IsDevelopment() {
		return
	}

	mappingBytes, err := cfw.Lookup(currentCFW).InputMapping()
	if err != nil {
		gaba.GetLogger().Error("Unable to read input mapping file", "error", err)
		return
	}
	if mappingBytes != nil {
		gaba.SetInputMappingBytes(mappingBytes)
	}
}

func initFramework(currentCFW cfw.CFW) {
	gabaOptions := gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             currentCFW == cfw.NextUI,
		DisplayOrientation:   gaba.OrientationNormal,
	}
	if preConfig, err := settings.LoadConfig(); err == nil {
		gaba.SetFlipFaceButtons(preConfig.SwapFaceButtons)
	}
	display := cfw.Lookup(currentCFW).Display()
	switch display.RotationDegrees {
	case 90:
		gabaOptions.DisplayOrientation = gaba.OrientationRotate90
	case 180:
		gabaOptions.DisplayOrientation = gaba.OrientationRotate180
	case 270:
		gabaOptions.DisplayOrientation = gaba.OrientationRotate270
	}
	if display.DisableKeyboardAndJoystick {
		gabaOptions.DisabledInputSources = gaba.DisabledInputSources{
			Keyboard: true,
			Joystick: true,
		}
	}
	gaba.Init(gabaOptions)

	gaba.RegisterChord("unlock-kid-mode", []buttons.VirtualButton{
		buttons.VirtualButtonL1,
		buttons.VirtualButtonR1,
		buttons.VirtualButtonMenu,
	}, gaba.ChordOptions{
		Window: time.Millisecond * 1500,
		OnTrigger: func() {
			if settings.IsKidModeEnabled() {
				settings.SetKidMode(false)
				gaba.GetLogger().Info("Kid Mode unlocked for this session")
			}
		},
	})

	gaba.SetLogLevel(slog.LevelDebug)

	// Lower layers log through slog rather than importing the UI toolkit, so
	// point the standard logger at the same place gabagool writes.
	slog.SetDefault(gaba.GetLogger())

	localeFiles, err := resources.GetLocaleMessageFiles()
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Failed to load locale files: %v", err)
	}
	if err := i18n.InitI18NFromBytes(localeFiles); err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Failed to initialize i18n: %v", err)
	}

	cfw.AddGroutToGamelist(currentCFW)
}

func loadOrCreateConfig(logger *slog.Logger) (*settings.Config, bool) {
	config, err := settings.LoadConfig()
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
			config = &settings.Config{
				ShowRegularCollections: true,
				ApiTimeout:             settings.DurationSeconds(30 * time.Second),
				DownloadTimeout:        settings.DurationSeconds(60 * time.Minute),
			}
		}
		config.Language = selectedLanguage
	}

	return config, isFirstLaunch
}

func handleFirstLaunch(config *settings.Config, isFirstLaunch bool, logger *slog.Logger) *settings.Config {
	if len(config.Hosts) > 0 {
		return config
	}

	logger.Debug("No RomM Host Configured, starting login flow")
	loginConfig, loginErr := ui.LoginFlow(settings.Host{})
	if loginErr != nil {
		logger.Error("Login flow failed", "error", loginErr)
		gaba.Close()
		log.SetOutput(os.Stderr)
		log.Fatalf("Login failed: %v", loginErr)
	}
	logger.Debug("Login successful, saving configuration")
	config.Hosts = loginConfig.Hosts
	config.PlatformsBinding = loginConfig.PlatformsBinding
	settings.SaveConfig(config)
	ui.ApplyRuntimeSettings(config)

	return config
}

func applyConfig(config *settings.Config, isFirstLaunch bool, currentCFW cfw.CFW, logger *slog.Logger) *settings.Config {
	if config.LogLevel != "" {
		gaba.SetRawLogLevel(string(config.LogLevel))
	}

	if config.Language != "" && !isFirstLaunch {
		if err := i18n.SetWithCode(config.Language); err != nil {
			logger.Error("Failed to set language", "error", err, "language", config.Language)
		}
	}

	settings.InitKidMode(config)
	gaba.SetFlipFaceButtons(config.SwapFaceButtons)

	if settings.IsKidModeEnabled() {
		splashBytes, _ := resources.GetSplashImageBytes()
		gaba.ProcessMessage("", gaba.ProcessMessageOptions{
			ImageBytes:   splashBytes,
			ImageWidth:   768,
			ImageHeight:  540,
			ProcessInput: true,
		}, func() (interface{}, error) {
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				if !settings.IsKidModeEnabled() {
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
			ApiTimeout:       config.ApiTimeout.Duration(),
			CFW:              currentCFW,
			RomDirectory:     cfw.GetRomDirectory(),
			AutoSelect:       false,
			HideBackButton:   true,
			PlatformsBinding: config.PlatformsBinding,
		})

		if err == nil && result.Action == ui.PlatformMappingActionSaved {
			config.DirectoryMappings = result.Mappings
			settings.SaveConfig(config)
			ui.ApplyRuntimeSettings(config)
		}
	}

	logger.Debug("Configuration Loaded!", "config", config.ToLoggable())

	return config
}

func connectAndLoadPlatforms(config *settings.Config, logger *slog.Logger) []romm.Platform {
	var platforms []romm.Platform
	splashBytes, _ := resources.GetSplashImageBytes()

	for {
		var connErr error
		var authErr error
		var loadErr error

		gaba.ProcessMessage("", gaba.ProcessMessageOptions{
			ImageBytes:  splashBytes,
			ImageWidth:  768,
			ImageHeight: 540,
		}, func() (interface{}, error) {
			host := config.Hosts[0]

			// Validate server connectivity
			client := romm.NewClient(host.URL(), romm.WithInsecureSkipVerify(host.InsecureSkipVerify), romm.WithTimeout(settings.ValidationTimeout))
			if err := client.ValidateConnection(); err != nil {
				connErr = err
				return nil, nil
			}

			// Validate token; configs from before the RomM 5.0 cutover have no
			// token and must re-pair.
			authClient := romm.NewClientFromHost(host, settings.LoginTimeout)
			if !host.HasTokenAuth() {
				authErr = errors.New("stored basic-auth credentials require re-pairing")
				return nil, nil
			}
			if err := authClient.ValidateToken(); err != nil {
				authErr = err
				return nil, nil
			}
			if host.Username == "" {
				if user, err := authClient.GetCurrentUser(); err == nil {
					host.Username = user.Username
					config.Hosts[0] = host
					settings.SaveConfig(config)
					ui.ApplyRuntimeSettings(config)
				}
			}

			// If grout was upgraded since this device last reported in, refresh the
			// client_version the server has on record (diagnostic/display only).
			if v, changed := saves.RefreshDeviceVersion(authClient, host.DeviceID, host.DeviceClientVersion); changed {
				host.DeviceClientVersion = v
				config.Hosts[0] = host
				settings.SaveConfig(config)
				ui.ApplyRuntimeSettings(config)
			}

			// Load platforms
			if err := catalog.LoadPlatformsBinding(config, config.Hosts[0], config.ApiTimeout.Duration()); err != nil {
				logger.Debug("Failed to load platform bindings", "error", err)
			}

			var err error
			platforms, err = catalog.MappedPlatforms(config.Hosts[0], config.DirectoryMappings, config.ApiTimeout.Duration())
			if err != nil {
				loadErr = err
				return nil, nil
			}
			platforms = catalog.SortByOrder(platforms, config.PlatformOrder)
			return nil, nil
		})

		if connErr != nil {
			logger.Warn("Server connectivity failed", "error", connErr)
			errorMessage := classifyStartupError(connErr)
			if !showStartupError(i18n.Localize(errorMessage, nil)) {
				gaba.Close()
				os.Exit(1)
			}
			continue
		}

		if authErr != nil {
			logger.Warn("Auth validation failed", "error", authErr)
			config = handleAuthFailure(config, logger)
			continue
		}

		if loadErr == nil {
			break
		}

		logger.Error("Failed to load platforms", "error", loadErr)
		if !showStartupError(i18n.Localize(classifyStartupError(loadErr), nil)) {
			logger.Info("User chose to quit after startup error")
			gaba.Close()
			os.Exit(1)
		}
		logger.Info("User chose to retry connection")
	}

	return platforms
}

func handleAuthFailure(config *settings.Config, logger *slog.Logger) *settings.Config {
	var msg string
	if config.Hosts[0].HasTokenAuth() {
		msg = i18n.Localize(&goi18n.Message{ID: "startup_error_token_invalid", Other: "Your API token is invalid or expired.\nPlease set up a new one."}, nil)
	} else {
		msg = i18n.Localize(&goi18n.Message{ID: "startup_error_repair_needed", Other: "Your RomM login needs to be re-paired.\nPlease log in again."}, nil)
	}

	gaba.ConfirmationMessage(msg, []gaba.FooterHelpItem{
		{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_continue", Other: "Continue"}, nil)},
	}, gaba.MessageOptions{})

	loginConfig, loginErr := ui.LoginFlow(config.Hosts[0])
	if loginErr != nil {
		logger.Error("Re-login failed", "error", loginErr)
		gaba.Close()
		log.SetOutput(os.Stderr)
		log.Fatalf("Login failed: %v", loginErr)
	}
	config.Hosts = loginConfig.Hosts
	config.PlatformsBinding = loginConfig.PlatformsBinding
	settings.SaveConfig(config)
	ui.ApplyRuntimeSettings(config)

	if err := cache.InitCacheManager(config.Hosts[0], *config); err != nil {
		logger.Error("Failed to re-initialize cache manager", "error", err)
	}

	return config
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
	case errors.Is(err, romm.ErrUnauthorized):
		return &goi18n.Message{ID: "startup_error_credentials", Other: "Authentication failed!\nYour RomM login may need to be re-paired."}
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
