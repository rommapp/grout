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
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type InfoInput struct {
	Host romm.Host
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

	result, err := gaba.DetailScreen("", options, []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
		{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_logout", Other: "Logout"}, nil)},
	})

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		gaba.GetLogger().Error("Info screen error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.DetailActionTriggered {
		output.LogoutRequested = true
		return withCode(output, constants.ExitCodeLogoutConfirm), nil
	}

	return back(output), nil
}

func (s *InfoScreen) buildSections(input InfoInput) []gaba.Section {
	sections := make([]gaba.Section, 0)

	versionInfo := version.Get()
	versionMetadata := []gaba.MetadataItem{
		{Label: i18n.Localize(&goi18n.Message{ID: "info_version", Other: "Version"}, nil), Value: versionInfo.Version},
		{Label: i18n.Localize(&goi18n.Message{ID: "info_commit", Other: "Commit"}, nil), Value: versionInfo.GitCommit},
		{Label: i18n.Localize(&goi18n.Message{ID: "info_build_date", Other: "Build Date"}, nil), Value: versionInfo.BuildDate},
	}
	sections = append(sections, gaba.NewInfoSection("Grout", versionMetadata))

	metadata := []gaba.MetadataItem{
		{
			Label: i18n.Localize(&goi18n.Message{ID: "info_server", Other: "Server"}, nil),
			Value: input.Host.RootURI,
		},
		{
			Label: i18n.Localize(&goi18n.Message{ID: "info_user", Other: "User"}, nil),
			Value: input.Host.Username,
		},
	}

	sections = append(sections, gaba.NewInfoSection("RomM", metadata))

	qrText := "https://github.com/rommapp/grout"
	qrcode, err := utils.CreateTempQRCode(qrText, 256)
	if err == nil {
		sections = append(sections, gaba.NewImageSection(
			i18n.Localize(&goi18n.Message{ID: "info_repository", Other: "GitHub Repository"}, nil),
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
