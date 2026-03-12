package ui

import (
	"errors"
	"fmt"
	"grout/internal"
	"grout/romm"
	"grout/sync"
	"os"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveSyncSettingsInput struct {
	Config *internal.Config
	Host   romm.Host
}

type SaveSyncSettingsOutput struct {
	Action SaveSyncSettingsAction
	Config *internal.Config
	Host   romm.Host
}

type SaveSyncSettingsScreen struct{}

func NewSaveSyncSettingsScreen() *SaveSyncSettingsScreen {
	return &SaveSyncSettingsScreen{}
}

func (s *SaveSyncSettingsScreen) Draw(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	if input.Host.DeviceID == "" {
		return s.drawUnregistered(input)
	}
	return s.drawRegistered(input)
}

func (s *SaveSyncSettingsScreen) drawUnregistered(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	output := SaveSyncSettingsOutput{Config: input.Config, Host: input.Host}

	items := []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "save_sync_register_device", Other: "Register Device"}, nil),
			},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_save_sync", Other: "Save Sync"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{FooterBack(), FooterSelect()},
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		items,
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
	logger := gaba.GetLogger()

	const (
		menuDeviceName = iota
		menuBackupLimit
		menuSaveMapping
	)

	items := []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "save_sync_device_name", Other: "Device Name"}, nil),
			},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable, DisplayName: input.Host.DeviceName}},
		},
		{
			Item: gaba.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "save_sync_backup_limit", Other: "Save Backups"}, nil),
			},
			Options: []gaba.Option{
				{DisplayName: "5", Value: 5},
				{DisplayName: "10", Value: 10},
				{DisplayName: "15", Value: 15},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "save_sync_backup_no_limit", Other: "No Limit"}, nil), Value: 0},
			},
			SelectedOption: backupLimitToIndex(input.Config.SaveBackupLimit),
		},
		{
			Item: gaba.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "sync_menu_save_mapping", Other: "Save Mapping"}, nil),
			},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_save_sync", Other: "Save Sync"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	// Apply backup limit setting
	if val, ok := result.Items[menuBackupLimit].Options[result.Items[menuBackupLimit].SelectedOption].Value.(int); ok {
		output.Config.SaveBackupLimit = val
	}

	if result.Action != gaba.ListActionSelected {
		return output, nil
	}

	switch result.Selected {
	case menuSaveMapping:
		output.Action = SaveSyncSettingsActionSaveMapping
		return output, nil
	case menuDeviceName:
		// fall through to device name editing below
	default:
		return output, nil
	}

	defaultName := input.Host.DeviceName
	if defaultName == "" {
		if hostname, err := os.Hostname(); err == nil {
			defaultName = hostname
		}
	}

	res, err := gaba.Keyboard(defaultName, i18n.Localize(&goi18n.Message{ID: "device_registration_prompt", Other: "Enter a name for this device"}, nil))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	deviceName := res.Text
	if deviceName == "" {
		return output, nil
	}

	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout)

	var updateErr error
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "device_registration_updating", Other: "Updating device..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			_, updateErr = client.UpdateDevice(input.Host.DeviceID, romm.UpdateDeviceRequest{Name: deviceName})
			return nil, nil
		},
	)
	if updateErr != nil {
		logger.Error("Failed to update device", "error", updateErr)
		gaba.ConfirmationMessage(
			fmt.Sprintf("Failed to update device: %v", updateErr),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	} else {
		output.Host.DeviceName = deviceName
	}
	return output, nil
}

func (s *SaveSyncSettingsScreen) registerDevice(output SaveSyncSettingsOutput) (SaveSyncSettingsOutput, error) {
	defaultName := ""
	if hostname, err := os.Hostname(); err == nil {
		defaultName = hostname
	}

	res, err := gaba.Keyboard(defaultName, i18n.Localize(&goi18n.Message{ID: "device_registration_prompt", Other: "Enter a name for this device"}, nil))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	deviceName := res.Text
	if deviceName == "" {
		return output, nil
	}

	client := romm.NewClientFromHost(output.Host, output.Config.ApiTimeout)

	var device romm.Device
	var regErr error
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "device_registration_registering", Other: "Registering device..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			device, regErr = sync.RegisterDevice(client, deviceName)
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
	output.Host.DeviceName = deviceName
	return output, nil
}
