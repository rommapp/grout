package ui

import (
	"grout/models"
	"grout/utils"
	"slices"
	"strings"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

func FetchListStateless(platform models.Platform) (shared.Items, error) {
	logger := gaba.GetLoggerInstance()

	logger.Debug("Fetching Item List",
		"host", platform.Host)

	client := utils.NewRomMClient(platform.Host)

	defer func(client shared.Client) {
		err := client.Close()
		if err != nil {
			logger.Error("Unable to close client", "error", err)
		}
	}(client)

	items, err := client.ListDirectory(platform.RomMPlatformID)
	if err != nil {
		return nil, err
	}

	filtered := make([]shared.Item, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.Filename, ".") {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

func filterList(itemList []shared.Item, filter string) []shared.Item {
	var result []shared.Item

	for _, item := range itemList {
		if strings.Contains(strings.ToLower(item.DisplayName), strings.ToLower(filter)) {
			result = append(result, item)
		}
	}

	slices.SortFunc(result, func(a, b shared.Item) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	return result
}
