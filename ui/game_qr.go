package ui

import (
	"errors"
	"grout/imaging"
	"grout/romm"
	"grout/settings"
	"os"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type GameQRInput struct {
	Host settings.Host
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
	qrcode, err := imaging.CreateTempQRCode(gameURL, 256)
	if err != nil {
		logger.Error("Unable to generate QR code", "error", err)
		return output, err
	}
	// It only has to last as long as the screen is drawn.
	defer os.Remove(qrcode)

	sections := []gaba.Section{
		gaba.NewImageSection(
			localize("game_qr_title", "RomM Game Page"),
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
		{ButtonName: "B", HelpText: localize("button_back", "Back")},
	}

	_, err = gaba.DetailScreen(input.Game.Name, options, footerItems)
	if err != nil && !errors.Is(err, gaba.ErrCancelled) {
		logger.Error("QR screen error", "error", err)
		return output, err
	}

	return output, nil
}
