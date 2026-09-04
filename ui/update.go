package ui

import (
	"errors"
	"fmt"

	"grout/cfw"
	"grout/settings"
	"grout/textmatch"
	"grout/update"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"go.uber.org/atomic"
)

type UpdateInput struct {
	CFW            cfw.CFW
	ReleaseChannel settings.ReleaseChannel
	Host           *settings.Host
}

type UpdateOutput struct {
	Action UpdateCheckAction
	// UpdatePerformed means the new version is staged and the caller should
	// end the app, since the launcher swaps it in on the next run.
	UpdatePerformed bool
}

type UpdateScreen struct{}

func NewUpdateScreen() *UpdateScreen {
	return &UpdateScreen{}
}

func (s *UpdateScreen) Draw(input UpdateInput) (UpdateOutput, error) {
	output := UpdateOutput{Action: UpdateCheckActionComplete}

	info, err := gaba.ProcessMessage(
		localize("update_checking", "Checking for updates..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (*update.Info, error) {
			return update.CheckForUpdate(input.CFW, input.ReleaseChannel, input.Host)
		},
	)
	if err != nil {
		gaba.GetLogger().Debug("Failed to check for updates", "error", err)
		s.tell(localizeWith("update_check_error", "{{.Error}}", map[string]any{"Error": err.Error()}))
		return output, nil
	}

	if !info.UpdateAvailable {
		s.tell(localizeWith("update_up_to_date", "You have the latest version ({{.Version}})",
			map[string]any{"Version": info.CurrentVersion}))
		return output, nil
	}

	wanted, err := s.offer(info)
	if err != nil {
		return output, err
	}
	if !wanted {
		return output, nil
	}

	if err := s.install(input.CFW, info); err != nil {
		gaba.GetLogger().Error("Failed to perform update", "error", err)
		s.tell(localizeWith("update_failed", "Update failed: {{.Error}}", map[string]any{"Error": err.Error()}))
		return output, nil
	}

	gaba.ConfirmationMessage(
		localize("update_complete", "Update complete! Grout will now exit."),
		[]gaba.FooterHelpItem{{ButtonName: "A", HelpText: localize("button_exit", "Exit")}},
		gaba.MessageOptions{},
	)

	output.UpdatePerformed = true
	return output, nil
}

// offer shows what is available and asks whether to take it.
func (s *UpdateScreen) offer(info *update.Info) (bool, error) {
	message := fmt.Sprintf("%s\n%s\n%s",
		localizeWith("update_available", "Update available: {{.Version}}", map[string]any{"Version": info.LatestVersion}),
		localizeWith("update_current_version", "Current: {{.Version}}", map[string]any{"Version": info.CurrentVersion}),
		localizeWith("update_size", "Size: {{.Size}}", map[string]any{"Size": textmatch.FormatBytes(info.AssetSize)}),
	)

	_, err := gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{
			FooterCancel(),
			{ButtonName: "A", HelpText: localize("update_download", "Download & Update")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonA},
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *UpdateScreen) install(activeCFW cfw.CFW, info *update.Info) error {
	progress := &atomic.Float64{}

	_, err := gaba.ProcessMessage(
		localize("update_downloading", "Downloading update..."),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     true,
			Progress:            progress,
		},
		func() (any, error) {
			return nil, update.PerformUpdate(activeCFW, info.DownloadURL, info.AssetSize, info.AssetSHA256, progress)
		},
	)
	return err
}

// tell shows a message the user acknowledges and returns from.
func (s *UpdateScreen) tell(message string) {
	gaba.ConfirmationMessage(message, []gaba.FooterHelpItem{FooterBack()}, gaba.MessageOptions{})
}
