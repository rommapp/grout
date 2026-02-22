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
	Config *internal.Config
	Host   romm.Host
}

type SaveSyncSettingsScreen struct{}

func NewSaveSyncSettingsScreen() *SaveSyncSettingsScreen {
	return &SaveSyncSettingsScreen{}
}

func (s *SaveSyncSettingsScreen) Draw(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	output := SaveSyncSettingsOutput{Config: input.Config, Host: input.Host}
	logger := gaba.GetLogger()

	currentName := ""
	if input.Host.DeviceID != "" {
		client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout)
		device, err := client.GetDevice(input.Host.DeviceID)
		if err == nil {
			currentName = device.Name
		} else {
			logger.Warn("Failed to fetch device info", "error", err)
		}
	}

	items := []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "save_sync_device_name", Other: "Device Name"}, nil),
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

	if result.Action != gaba.ListActionSelected {
		return output, nil
	}

	defaultName := currentName
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

	if input.Host.DeviceID != "" {
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
		}
		return output, nil
	}

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
	return output, nil
}
