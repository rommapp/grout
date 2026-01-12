package ui

import (
	"errors"
	"fmt"
	"grout/cfw"
	"grout/internal"
	"grout/internal/stringutil"
	"grout/update"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
)

type UpdateInput struct {
	CFW            cfw.CFW
	ReleaseChannel internal.ReleaseChannel
}

type UpdateOutput struct {
	UpdatePerformed bool
}

type UpdateScreen struct{}

func NewUpdateScreen() *UpdateScreen {
	return &UpdateScreen{}
}

func (s *UpdateScreen) Draw(input UpdateInput) (ScreenResult[UpdateOutput], error) {
	logger := gaba.GetLogger()
	output := UpdateOutput{}

	var updateInfo *update.Info
	var checkErr error

	_, err := gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "update_checking", Other: "Checking for updates..."}, nil),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
		},
		func() (interface{}, error) {
			updateInfo, checkErr = update.CheckForUpdate(input.CFW, input.ReleaseChannel)
			return nil, checkErr
		},
	)

	if err != nil || checkErr != nil {
		actualErr := checkErr
		if err != nil {
			actualErr = err
		}
		logger.Error("Failed to check for updates", "error", actualErr)

		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "update_failed", Other: "Update failed: {{.Error}}"}, map[string]interface{}{"Error": actualErr.Error()}),
			[]gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
			},
			gaba.MessageOptions{},
		)
		return back(output), nil
	}

	if !updateInfo.UpdateAvailable {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "update_up_to_date", Other: "You have the latest version ({{.Version}})"}, map[string]interface{}{"Version": updateInfo.CurrentVersion}),
			[]gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
			},
			gaba.MessageOptions{},
		)
		return back(output), nil
	}

	updateMessage := fmt.Sprintf(
		"%s\n%s\n%s",
		i18n.Localize(&goi18n.Message{ID: "update_available", Other: "Update available: {{.Version}}"}, map[string]interface{}{"Version": updateInfo.LatestVersion}),
		i18n.Localize(&goi18n.Message{ID: "update_current_version", Other: "Current: {{.Version}}"}, map[string]interface{}{"Version": updateInfo.CurrentVersion}),
		i18n.Localize(&goi18n.Message{ID: "update_size", Other: "Size: {{.Size}}"}, map[string]interface{}{"Size": stringutil.FormatBytes(updateInfo.AssetSize)}),
	)

	_, err = gaba.ConfirmationMessage(
		updateMessage,
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil)},
			{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "update_download", Other: "Download & Update"}, nil)},
		},
		gaba.MessageOptions{
			ConfirmButton: buttons.VirtualButtonA,
		},
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		return withCode(output, gaba.ExitCodeError), err
	}

	progress := &atomic.Float64{}
	var updateErr error

	_, err = gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "update_downloading", Other: "Downloading update..."}, nil),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     true,
			Progress:            progress,
		},
		func() (interface{}, error) {
			updateErr = update.PerformUpdate(updateInfo.DownloadURL, progress)
			return nil, updateErr
		},
	)

	if err != nil || updateErr != nil {
		actualErr := updateErr
		if err != nil {
			actualErr = err
		}
		logger.Error("Failed to perform update", "error", actualErr)

		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "update_failed", Other: "Update failed: {{.Error}}"}, map[string]interface{}{"Error": actualErr.Error()}),
			[]gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
			},
			gaba.MessageOptions{},
		)
		return back(output), nil
	}

	gaba.ConfirmationMessage(
		i18n.Localize(&goi18n.Message{ID: "update_complete", Other: "Update complete! Grout will now exit."}, nil),
		[]gaba.FooterHelpItem{
			{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_exit", Other: "Exit"}, nil)},
		},
		gaba.MessageOptions{},
	)

	output.UpdatePerformed = true
	return withCode(output, gaba.ExitCodeSuccess), nil
}
