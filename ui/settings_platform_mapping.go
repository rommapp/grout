package ui

import (
	"fmt"
	"grout/client"
	"grout/models"
	"grout/utils"
	"path/filepath"
	"slices"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/filebrowser"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"qlova.tech/sum"
)

type PlatformMappingScreen struct {
	Host           models.Host
	HideBackButton bool
}

func InitPlatformMappingScreen(host models.Host, hideBackButton bool) PlatformMappingScreen {
	return PlatformMappingScreen{
		Host:           host,
		HideBackButton: hideBackButton,
	}
}

func (p PlatformMappingScreen) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.SettingsPlatformMapping
}

func (p PlatformMappingScreen) Draw() (interface{}, int, error) {
	logger := gaba.GetLogger()
	//appState := state.GetAppState()

	c := client.NewRomMClient(p.Host)

	rommPlatforms, err := c.GetPlatforms()
	if err != nil {
		logger.Error("Error loading fetching RomM Platforms", "error", err)
		return nil, 0, err
	}

	fb := filebrowser.NewFileBrowser(logger)
	err = fb.CWD(utils.GetRomDirectory(), false)
	if err != nil {
		logger.Error("Error loading fetching ROM directories", "error", err)
		return nil, 1, err
	}

	unmapped := gaba.Option{
		DisplayName: "Unmapped",
		Value:       "",
	}

	var mappingOptions []gaba.ItemWithOptions

	for _, platform := range rommPlatforms {
		options := []gaba.Option{unmapped}

		rdi := slices.IndexFunc(fb.Items, func(item shared.Item) bool {
			return platform.Slug == filepath.Base(item.Path)
		})

		for _, romDirectory := range fb.Items {
			options = append(options, gaba.Option{
				DisplayName: fmt.Sprintf("/Roms/%s", filepath.Base(romDirectory.Path)),
				Value:       filepath.Base(romDirectory.Path),
			})
		}

		if rdi == -1 {
			options = append(options, gaba.Option{
				DisplayName: fmt.Sprintf("Create '%s'", utils.RomMSlugToMuOS(platform.Slug)),
				Value:       utils.RomMSlugToMuOS(platform.Slug),
			})
		}

		selectedIndex := rdi

		if selectedIndex == -1 {
			selectedIndex = len(options) - 1
		} else {
			selectedIndex = selectedIndex + 1
		}

		mappingOptions = append(mappingOptions, gaba.ItemWithOptions{
			Item: gaba.MenuItem{
				Text:     platform.DisplayName,
				Metadata: platform.Slug,
			},
			Options:        options,
			SelectedOption: selectedIndex,
		})

	}

	fhi := []gaba.FooterHelpItem{
		{ButtonName: "←→", HelpText: "Cycle"},
		{ButtonName: "Start", HelpText: "Save"},
	}

	if !p.HideBackButton {
		fhi = slices.Insert(fhi, 0, gaba.FooterHelpItem{ButtonName: "B", HelpText: "Cancel"})
	}

	result, err := gaba.OptionsList(
		"Rom Directory Mapping",
		gaba.OptionListSettings{
			FooterHelpItems:   fhi,
			DisableBackButton: p.HideBackButton},
		mappingOptions,
	)

	if err != nil {
		// TODO fill me
	}

	if result == nil || result.IsNone() {
		return nil, 1, nil
	}

	mappings := make(map[string]models.DirectoryMapping)

	for _, m := range result.Unwrap().Items {
		rp := m.Item.Metadata.(string)
		rfd := m.Options[m.SelectedOption].Value.(string)

		if rfd == "" {
			continue
		}

		mappings[rp] = models.DirectoryMapping{
			RomMSlug:     rp,
			RelativePath: rfd,
		}
	}

	return mappings, 0, nil
}
