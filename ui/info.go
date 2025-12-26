package ui

import (
	"errors"
	"grout/constants"
	"grout/romm"
	"grout/utils"
	"grout/version"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

type InfoInput struct {
	Host         romm.Host
	FromAdvanced bool
}

type InfoOutput struct {
	LogoutRequested bool
}

type InfoScreen struct{}

func NewInfoScreen() *InfoScreen {
	return &InfoScreen{}
}

func (s *InfoScreen) Draw(input InfoInput) (ScreenResult[InfoOutput], error) {
	output := InfoOutput{}

	sections := s.buildSections(input)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ActionButton = buttons.VirtualButtonX
	options.EnableAction = true

	result, err := gaba.DetailScreen(i18n.GetString("info_title"), options, []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.GetString("button_back")},
		{ButtonName: "X", HelpText: i18n.GetString("button_logout")},
	})

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			if input.FromAdvanced {
				return withCode(output, constants.ExitCodeBackToAdvanced), nil
			}
			return back(output), nil
		}
		gaba.GetLogger().Error("Info screen error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.DetailActionTriggered {
		output.LogoutRequested = true
		return withCode(output, constants.ExitCodeLogoutConfirm), nil
	}

	if input.FromAdvanced {
		return withCode(output, constants.ExitCodeBackToAdvanced), nil
	}
	return back(output), nil
}

func (s *InfoScreen) buildSections(input InfoInput) []gaba.Section {
	sections := make([]gaba.Section, 0)

	versionInfo := version.Get()
	versionMetadata := []gaba.MetadataItem{
		{Label: i18n.GetString("info_version"), Value: versionInfo.Version},
		{Label: i18n.GetString("info_commit"), Value: versionInfo.GitCommit},
		{Label: i18n.GetString("info_build_date"), Value: versionInfo.BuildDate},
	}
	sections = append(sections, gaba.NewInfoSection("Grout", versionMetadata))

	// RomM server metadata
	metadata := []gaba.MetadataItem{
		{
			Label: i18n.GetString("info_server"),
			Value: input.Host.RootURI,
		},
		{
			Label: i18n.GetString("info_user"),
			Value: input.Host.Username,
		},
	}

	sections = append(sections, gaba.NewInfoSection("RomM", metadata))

	qrText := "https://github.com/rommapp/grout"
	qrcode, err := utils.CreateTempQRCode(qrText, 256)
	if err == nil {
		sections = append(sections, gaba.NewImageSection(
			i18n.GetString("info_repository"),
			qrcode,
			int32(256),
			int32(256),
			buttons.TextAlignCenter,
		))
	} else {
		gaba.GetLogger().Error("Unable to generate QR code for repository", "error", err)
	}

	return sections
}
