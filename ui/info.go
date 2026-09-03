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
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
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
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
		{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_logout", Other: "Logout"}, nil)},
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
		{Label: i18n.Localize(&goi18n.Message{ID: "info_version", Other: "Version"}, nil), Value: versionInfo.Version},
		{Label: i18n.Localize(&goi18n.Message{ID: "info_commit", Other: "Commit"}, nil), Value: versionInfo.GitCommit},
		{Label: i18n.Localize(&goi18n.Message{ID: "info_build_date", Other: "Build Date"}, nil), Value: versionInfo.BuildDate},
		{Label: i18n.Localize(&goi18n.Message{ID: "info_cfw", Other: "CFW"}, nil), Value: string(input.CFW)},
	}
	sections = append(sections, gaba.NewInfoSection("Grout", versionMetadata))

	rommVersion := input.RommVersion
	if rommVersion == "" {
		rommVersion = i18n.Localize(&goi18n.Message{ID: "info_unknown", Other: "Unknown"}, nil)
	}

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

	if input.Host.HasTokenAuth() {
		if input.Host.TokenName != "" {
			metadata = append(metadata, gaba.MetadataItem{
				Label: i18n.Localize(&goi18n.Message{ID: "info_token_name", Other: "Token"}, nil),
				Value: input.Host.TokenName,
			})
		}

		expiresValue := i18n.Localize(&goi18n.Message{ID: "info_token_never_expires", Other: "Never"}, nil)
		if input.Host.TokenExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, input.Host.TokenExpiresAt); err == nil {
				expiresValue = t.Local().Format("2006-01-02 15:04")
			} else {
				expiresValue = input.Host.TokenExpiresAt
			}
		}
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "info_token_expires", Other: "Expires"}, nil),
			Value: expiresValue,
		})
	}

	metadata = append(metadata, gaba.MetadataItem{
		Label: i18n.Localize(&goi18n.Message{ID: "info_romm_version", Other: "Version"}, nil),
		Value: rommVersion,
	})

	sections = append(sections, gaba.NewInfoSection("RomM", metadata))

	qrText := "https://github.com/rommapp/grout"
	qrcode, err := imaging.CreateTempQRCode(qrText, 256)
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
