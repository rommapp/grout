package ui

import (
	"errors"
	"grout/internal/imageutil"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GameQRInput struct {
	Host romm.Host
	Game romm.Rom
}

type GameQROutput struct{}

type GameQRScreen struct{}

func NewGameQRScreen() *GameQRScreen {
	return &GameQRScreen{}
}

func (s *GameQRScreen) Draw(input GameQRInput) (GameQROutput, error) {
	output := GameQROutput{}
	logger := gaba.GetLogger()

	gameURL := input.Game.GetGamePage(input.Host)
	qrcode, err := imageutil.CreateTempQRCode(gameURL, 256)
	if err != nil {
		logger.Error("Unable to generate QR code", "error", err)
		return output, err
	}

	sections := []gaba.Section{
		gaba.NewImageSection(
			i18n.Localize(&goi18n.Message{ID: "game_qr_title", Other: "RomM Game Page"}, nil),
			qrcode,
			int32(256),
			int32(256),
			constants.TextAlignCenter,
		),
	}

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ConfirmButton = constants.VirtualButtonUnassigned

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
	}

	_, err = gaba.DetailScreen(input.Game.Name, options, footerItems)
	if err != nil && !errors.Is(err, gaba.ErrCancelled) {
		logger.Error("QR screen error", "error", err)
		return output, err
	}

	return output, nil
}
