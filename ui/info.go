package ui

import (
	"errors"
	"os"
	"time"

	"grout/cfw"
	"grout/imaging"
	"grout/settings"
	"grout/version"

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

const repositoryURL = "https://github.com/rommapp/grout"

func (s *InfoScreen) Draw(input InfoInput) (InfoOutput, error) {
	output := InfoOutput{Action: InfoActionBack}

	// The QR is a file on disk that only has to outlive this screen, so it is
	// made and removed here rather than by whatever builds the sections.
	qrcode, err := imaging.CreateTempQRCode(repositoryURL, qrSize)
	if err != nil {
		gaba.GetLogger().Error("Unable to generate QR code for repository", "error", err)
		qrcode = ""
	} else {
		defer os.Remove(qrcode)
	}

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = infoSections(input, qrcode)
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ActionButton = buttons.VirtualButtonX
	options.AllowAction = true
	options.ConfirmButton = buttons.VirtualButtonUnassigned

	result, err := gaba.DetailScreen("", options, []gaba.FooterHelpItem{
		FooterBack(),
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
	}

	return output, nil
}

const qrSize = 256

// infoSections describes what grout is running against. An empty qrcode means
// the repository link could not be drawn, which is not worth an empty box.
func infoSections(input InfoInput, qrcode string) []gaba.Section {
	sections := []gaba.Section{
		gaba.NewInfoSection("Grout", groutFacts(input.CFW)),
		gaba.NewInfoSection("RomM", serverFacts(input.Host, input.RommVersion)),
	}

	if qrcode != "" {
		sections = append(sections, gaba.NewImageSection(
			localize("info_repository", "GitHub Repository"),
			qrcode, qrSize, qrSize, buttons.TextAlignCenter,
		))
	}

	return sections
}

func groutFacts(activeCFW cfw.CFW) []gaba.MetadataItem {
	build := version.Get()

	return []gaba.MetadataItem{
		{Label: localize("info_version", "Version"), Value: build.Version},
		{Label: localize("info_commit", "Commit"), Value: build.GitCommit},
		{Label: localize("info_build_date", "Build Date"), Value: build.BuildDate},
		{Label: localize("info_cfw", "CFW"), Value: string(activeCFW)},
	}
}

func serverFacts(host settings.Host, rommVersion string) []gaba.MetadataItem {
	facts := []gaba.MetadataItem{
		{Label: localize("info_server", "Server"), Value: host.RootURI},
		{Label: localize("info_user", "User"), Value: host.Username},
	}

	if host.HasTokenAuth() {
		if host.TokenName != "" {
			facts = append(facts, gaba.MetadataItem{
				Label: localize("info_token_name", "Token"), Value: host.TokenName,
			})
		}
		facts = append(facts, gaba.MetadataItem{
			Label: localize("info_token_expires", "Expires"), Value: tokenExpiry(host.TokenExpiresAt),
		})
	}

	if rommVersion == "" {
		rommVersion = localize("info_unknown", "Unknown")
	}
	return append(facts, gaba.MetadataItem{
		Label: localize("info_romm_version", "Version"), Value: rommVersion,
	})
}

// tokenExpiry reads when a token runs out.
//
// The server sends RFC 3339; anything else is shown as it arrived rather than
// swallowed, since a token with an unreadable expiry is worth noticing.
func tokenExpiry(expiresAt string) string {
	if expiresAt == "" {
		return localize("info_token_never_expires", "Never")
	}
	if parsed, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		return parsed.Local().Format("2006-01-02 15:04")
	}
	return expiresAt
}
