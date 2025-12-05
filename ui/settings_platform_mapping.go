package ui

import (
	"fmt"
	"grout/client"
	"grout/models"
	"grout/state"
	"grout/utils"
	"path/filepath"
	"slices"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"qlova.tech/sum"
)

type PlatformMappingScreen struct {
	Host           models.Host
	AutoSelect     bool
	HideBackButton bool
}

func InitPlatformMappingScreen(host models.Host, autoSelect bool, hideBackButton bool) PlatformMappingScreen {
	return PlatformMappingScreen{
		Host:           host,
		AutoSelect:     autoSelect,
		HideBackButton: hideBackButton,
	}
}

func (p PlatformMappingScreen) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.SettingsPlatformMapping
}

func (p PlatformMappingScreen) Draw() (interface{}, int, error) {
	logger := gaba.GetLogger()
	config := state.GetAppState().Config

	c := client.NewRomMClient(p.Host, config.ApiTimeout)

	rommPlatforms, err := c.GetPlatforms()
	if err != nil {
		logger.Error("Error loading fetching RomM Platforms", "error", err)
		return nil, 0, err
	}

	fb := utils.NewFileBrowser(logger)
	err = fb.CWD(utils.GetRomDirectory(), false)
	if err != nil {
		logger.Error("Error loading fetching ROM directories", "error", err)
		return nil, 1, err
	}

	unmapped := gaba.Option{
		DisplayName: "Skip",
		Value:       "",
	}

	var mappingOptions []gaba.ItemWithOptions

	for _, platform := range rommPlatforms {
		options := []gaba.Option{unmapped}

		rdi := slices.IndexFunc(fb.Items, func(item models.Item) bool {
			switch utils.GetCFW() {
			case models.NEXTUI:
				return utils.ParseTag(utils.RomMSlugToCFW(platform.Slug)) == utils.RomFolderBase(item)
			case models.MUOS:
				return utils.RomMSlugToCFW(platform.Slug) == utils.RomFolderBase(item)
			default:
				return utils.RomMSlugToCFW(platform.Slug) == utils.RomFolderBase(item)
			}
		})

		canCreate := false

		if rdi == -1 {
			dn := utils.RomMSlugToCFW(platform.Slug)

			if utils.GetCFW() == models.NEXTUI {
				dn = utils.ParseTag(dn)
			}

			if dn != "" {
				options = append(options, gaba.Option{
					DisplayName: fmt.Sprintf("Create '%s'", dn),
					Value:       utils.RomMSlugToCFW(platform.Slug),
				})

				canCreate = true
			}
		}

		selectedIndex := 0

		for _, romDirectory := range fb.Items {
			dn := filepath.Base(romDirectory.Path)

			if utils.GetCFW() == models.NEXTUI {
				dn = utils.ParseTag(dn)
			}

			options = append(options, gaba.Option{
				DisplayName: fmt.Sprintf("/%s", dn),
				Value:       filepath.Base(romDirectory.Path),
			})

			switch utils.GetCFW() {
			case models.NEXTUI:
				if utils.ParseTag(utils.RomMSlugToCFW(platform.Slug)) == utils.RomFolderBase(romDirectory) {
					selectedIndex = len(options) - 1
				}
			case models.MUOS:
				if utils.RomMSlugToCFW(platform.Slug) == utils.RomFolderBase(romDirectory) {
					selectedIndex = len(options) - 1
				}
			}

		}

		if selectedIndex == 0 && len(options) > 1 && (len(fb.Items) == 0 || (canCreate && p.AutoSelect)) {
			selectedIndex = 1
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
		// TODO might need logging
		return nil, 1, nil
	}

	mappings := make(map[string]models.DirectoryMapping)

	for _, m := range result.Items {
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
