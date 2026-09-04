package ui

import (
	"errors"
	"fmt"
	"os"

	"grout/romm"
	"grout/saves"
	"grout/settings"
	"grout/version"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveSyncSettingsInput struct {
	Config *settings.Config
	Host   settings.Host
}

type SaveSyncSettingsOutput struct {
	Action SaveSyncSettingsAction
	Config *settings.Config
	Host   settings.Host
}

type SaveSyncSettingsScreen struct{}

func NewSaveSyncSettingsScreen() *SaveSyncSettingsScreen {
	return &SaveSyncSettingsScreen{}
}

// saveSyncDestinations is where each row that navigates goes. Rows that only
// hold a value are absent.
var saveSyncDestinations = map[string]SaveSyncSettingsAction{
	"save_mapping": SaveSyncSettingsActionSaveMapping,
}

func (s *SaveSyncSettingsScreen) Draw(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	if input.Host.DeviceID == "" {
		return s.drawUnregistered(input)
	}
	return s.drawRegistered(input)
}

// drawUnregistered offers the one thing a device with no registration can do.
func (s *SaveSyncSettingsScreen) drawUnregistered(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	output := SaveSyncSettingsOutput{Config: input.Config, Host: input.Host}

	result, err := gaba.OptionsList(
		localize("settings_save_sync", "Save Sync"),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{FooterBack(), FooterSelect()},
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		[]gaba.ItemWithOptions{{
			Item:    gaba.MenuItem{Text: localize("save_sync_register_device", "Register Device")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}},
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}
	if result.Action != gaba.ListActionSelected {
		return output, nil
	}

	return s.registerDevice(output)
}

func (s *SaveSyncSettingsScreen) drawRegistered(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	output := SaveSyncSettingsOutput{Config: input.Config, Host: input.Host}
	rows := saveSyncRows(input.Host)

	result, err := gaba.OptionsList(
		localize("settings_save_sync", "Save Sync"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		settingItems(rows, *input.Config),
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	applySettingRows(rows, input.Config, result.Items)

	// This screen writes its own config, the way the other settings screens
	// do. Leaving it to the caller does not work: the caller handed over the
	// very config being edited, so it has nothing to compare against.
	if err := settings.SaveConfig(input.Config); err != nil {
		gaba.GetLogger().Error("Error saving save sync settings", "error", err)
	}

	if result.Action != gaba.ListActionSelected || result.Selected >= len(rows) {
		return output, nil
	}

	switch key := rows[result.Selected].key; key {
	case "device_name":
		return s.renameDevice(input, output)
	default:
		if action, leads := saveSyncDestinations[key]; leads {
			output.Action = action
		}
		return output, nil
	}
}

func saveSyncRows(host settings.Host) []settingRow {
	// The name sits on the row rather than behind it, since it is the thing
	// the row changes.
	deviceName := clickableRow("device_name", "save_sync_device_name", "Device Name")
	deviceName.display = host.DeviceName

	return []settingRow{
		deviceName,
		{
			key: "backup_limit", label: localize("save_sync_backup_limit", "Save Backups"),
			options: backupLimitOptions(),
			get:     func(c settings.Config) any { return c.SaveBackupLimit },
			set:     assign(func(c *settings.Config, v int) { c.SaveBackupLimit = v }),
			def:     0,
		},
		clickableRow("save_mapping", "sync_menu_save_mapping", "Save Mapping"),
	}
}

// renameDevice changes what the server calls this device.
func (s *SaveSyncSettingsScreen) renameDevice(input SaveSyncSettingsInput, output SaveSyncSettingsOutput) (SaveSyncSettingsOutput, error) {
	name, err := askDeviceName(input.Host.DeviceName)
	if err != nil {
		return output, err
	}
	if name == "" {
		return output, nil
	}

	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout.Duration())

	var updateErr error
	gaba.ProcessMessage(
		localize("device_registration_updating", "Updating device..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			_, updateErr = client.UpdateDevice(input.Host.DeviceID, romm.UpdateDeviceRequest{Name: name})
			return nil, nil
		},
	)
	if updateErr != nil {
		gaba.GetLogger().Error("Failed to update device", "error", updateErr)
		gaba.ConfirmationMessage(
			fmt.Sprintf("Failed to update device: %v", updateErr),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	output.Host.DeviceName = name
	return output, nil
}

// registerDevice tells the server about this device so saves can be synced to
// it.
func (s *SaveSyncSettingsScreen) registerDevice(output SaveSyncSettingsOutput) (SaveSyncSettingsOutput, error) {
	name, err := askDeviceName("")
	if err != nil {
		return output, err
	}
	if name == "" {
		return output, nil
	}

	client := romm.NewClientFromHost(output.Host, output.Config.ApiTimeout.Duration())

	var device romm.Device
	var regErr error
	gaba.ProcessMessage(
		localize("device_registration_registering", "Registering device..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			device, regErr = saves.RegisterDevice(client, name)
			return nil, nil
		},
	)
	if regErr != nil {
		gaba.ConfirmationMessage(
			fmt.Sprintf("Failed to register device: %v", regErr),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	output.Host.DeviceID = device.ID
	output.Host.DeviceName = name
	// RegisterDevice already pushed the current client_version; record it so the startup
	// refresh only fires on a later upgrade.
	output.Host.DeviceClientVersion = version.Get().Version
	return output, nil
}

// askDeviceName prompts for a name, offering the one already set or failing
// that the machine's own. An empty answer means the user backed out.
func askDeviceName(current string) (string, error) {
	suggestion := current
	if suggestion == "" {
		if hostname, err := os.Hostname(); err == nil {
			suggestion = hostname
		}
	}

	result, err := gaba.Keyboard(suggestion, localize("device_registration_prompt", "Enter a name for this device"))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return "", nil
		}
		return "", err
	}
	return result.Text, nil
}

// backupLimitOptions offers how many old copies of a save to keep. Zero is no
// limit, which is what an unset config means.
func backupLimitOptions() []gaba.Option {
	return []gaba.Option{
		{DisplayName: "5", Value: 5},
		{DisplayName: "10", Value: 10},
		{DisplayName: "15", Value: 15},
		{DisplayName: localize("save_sync_backup_no_limit", "No Limit"), Value: 0},
	}
}
