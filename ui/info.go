package ui

import (
	"errors"
	"grout/cfw"
	"grout/imaging"
	"grout/settings"
	"grout/version"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type InfoInput struct {
	Host        settings.Host
	CFW         cfw.CFW
	RommVersion string
}

type InfoOutput struct {
	Action          InfoAction
	LogoutRequested bool
}

type InfoScreen struct{}

func NewInfoScreen() *InfoScreen {
	return &InfoScreen{}
}

func (s *InfoScreen) Draw(input InfoInput) (InfoOutput, error) {
	output := InfoOutput{Action: InfoActionBack}

	sections := s.buildSections(input)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ActionButton = buttons.VirtualButtonX
	options.AllowAction = true
	options.ConfirmButton = buttons.VirtualButtonUnassigned

	result, err := gaba.DetailScreen("", options, []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: localize("button_back", "Back")},
		{ButtonName: "X", HelpText: localize("button_logout", "Logout")},
	})

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Info screen error", "error", err)
		return output, err
	}

	if result.Action == gaba.DetailActionTriggered {
		output.LogoutRequested = true
		output.Action = InfoActionLogout
		return output, nil
	}

	return output, nil
}

func (s *InfoScreen) buildSections(input InfoInput) []gaba.Section {
	sections := make([]gaba.Section, 0)

	versionInfo := version.Get()
	versionMetadata := []gaba.MetadataItem{
		{Label: localize("info_version", "Version"), Value: versionInfo.Version},
		{Label: localize("info_commit", "Commit"), Value: versionInfo.GitCommit},
		{Label: localize("info_build_date", "Build Date"), Value: versionInfo.BuildDate},
		{Label: localize("info_cfw", "CFW"), Value: string(input.CFW)},
	}
	sections = append(sections, gaba.NewInfoSection("Grout", versionMetadata))

	rommVersion := input.RommVersion
	if rommVersion == "" {
		rommVersion = localize("info_unknown", "Unknown")
	}

	metadata := []gaba.MetadataItem{
		{
			Label: localize("info_server", "Server"),
			Value: input.Host.RootURI,
		},
		{
			Label: localize("info_user", "User"),
			Value: input.Host.Username,
		},
	}

	if input.Host.HasTokenAuth() {
		if input.Host.TokenName != "" {
			metadata = append(metadata, gaba.MetadataItem{
				Label: localize("info_token_name", "Token"),
				Value: input.Host.TokenName,
			})
		}

		expiresValue := localize("info_token_never_expires", "Never")
		if input.Host.TokenExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, input.Host.TokenExpiresAt); err == nil {
				expiresValue = t.Local().Format("2006-01-02 15:04")
			} else {
				expiresValue = input.Host.TokenExpiresAt
			}
		}
		metadata = append(metadata, gaba.MetadataItem{
			Label: localize("info_token_expires", "Expires"),
			Value: expiresValue,
		})
	}

	metadata = append(metadata, gaba.MetadataItem{
		Label: localize("info_romm_version", "Version"),
		Value: rommVersion,
	})

	sections = append(sections, gaba.NewInfoSection("RomM", metadata))

	qrText := "https://github.com/rommapp/grout"
	qrcode, err := imaging.CreateTempQRCode(qrText, 256)
	if err == nil {
		sections = append(sections, gaba.NewImageSection(
			localize("info_repository", "GitHub Repository"),
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
