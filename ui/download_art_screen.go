package ui

import (
	"fmt"
	"grout/models"
	"grout/utils"
	"time"

	"github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"qlova.tech/sum"
)

type DownloadArtScreen struct {
	Platform     models.Platform
	Games        shared.Items
	DownloadType sum.Int[shared.ArtDownloadType]
	SearchFilter string
}

func InitDownloadArtScreen(platform models.Platform, games shared.Items, downloadType sum.Int[shared.ArtDownloadType], searchFilter string) models.Screen {
	return DownloadArtScreen{
		Platform:     platform,
		Games:        games,
		DownloadType: downloadType,
		SearchFilter: searchFilter,
	}
}

func (a DownloadArtScreen) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.DownloadArt
}

func (a DownloadArtScreen) Draw() (value interface{}, exitCode int, e error) {
	var artPaths []string

	gabagool.ProcessMessage("Downloading art...",
		gabagool.ProcessMessageOptions{ShowThemeBackground: true}, func() (interface{}, error) {
			for _, game := range a.Games {
				artPath := utils.FindArt(a.Platform, game, a.DownloadType)

				if artPath != "" {
					artPaths = append(artPaths, artPath)
				}
			}
			return nil, nil
		})

	if len(artPaths) == 0 {
		gabagool.ProcessMessage("No art found!",
			gabagool.ProcessMessageOptions{ShowThemeBackground: true}, func() (interface{}, error) {
				time.Sleep(time.Millisecond * 1500)
				return nil, nil
			})

		return nil, 404, nil
	} else if len(a.Games) > 1 {
		gabagool.ProcessMessage(fmt.Sprintf("Art found for %d/%d games!", len(artPaths), len(a.Games)),
			gabagool.ProcessMessageOptions{ShowThemeBackground: true}, func() (interface{}, error) {
				time.Sleep(time.Millisecond * 1500)
				return nil, nil
			})
	}

	for _, artPath := range artPaths {
		result, err := gabagool.ConfirmationMessage("Found This Art!",
			[]gabagool.FooterHelpItem{
				{ButtonName: "B", HelpText: "I'll Find My Own"},
				{ButtonName: "A", HelpText: "Use It!"},
			},
			gabagool.MessageOptions{
				ImagePath: artPath,
			})

		if err != nil || result.IsNone() {
			common.DeleteFile(artPath)
		}
	}

	time.Sleep(time.Millisecond * 100)

	return nil, 0, nil
}
