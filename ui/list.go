package ui

import (
	"encoding/json"
	"grout/clients"
	"grout/models"
	"grout/utils"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

func FetchListStateless(platform models.Platform) (shared.Items, error) {
	logger := gaba.GetLoggerInstance()

	logger.Debug("Fetching Item List",
		"host", platform.Host)

	client, err := clients.BuildClient(platform.Host)
	if err != nil {
		return nil, err
	}

	defer func(client shared.Client) {
		err := client.Close()
		if err != nil {
			logger.Error("Unable to close client", "error", err)
		}
	}(client)

	subdirectory := ""

	switch platform.Host.HostType {
	case shared.HostTypes.ROMM:
		subdirectory = platform.RomMPlatformID
	default:
		subdirectory = platform.HostSubdirectory
	}

	items, err := client.ListDirectory(subdirectory)
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		items[i].DisplayName = strings.ReplaceAll(item.Filename, filepath.Ext(item.Filename), "")
	}

	filtered := make([]shared.Item, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.Filename, ".") {
			filtered = append(filtered, item)
		}
	}
	items = filtered

	if platform.Host.HostType == shared.HostTypes.MEGATHREAD {
		jsonData, err := json.Marshal(items)
		if err != nil {
			logger.Debug("Unable to get marshal JSON for Megathread", "error", err)

			cwd, err := os.Getwd()
			if err != nil {
				logger.Debug("Unable to get current working directory for caching Megathread", "error", err)
			}

			filePath := path.Join(cwd, ".cache", utils.CachedMegaThreadJsonFilename("", ""))
			err = os.WriteFile(filePath, jsonData, 0644)
			if err != nil {
				logger.Debug("Unable to write JSON to file for Megathread", "error", err)
			}
		}
	}

	return items, nil
}

func filterList(itemList []shared.Item, filters models.Filters) []shared.Item {
	result := itemList

	if len(filters.InclusiveFilters) > 0 {
		result = nil
		for _, item := range itemList {
			for _, filter := range filters.InclusiveFilters {
				if strings.Contains(strings.ToLower(item.DisplayName), strings.ToLower(filter)) {
					result = append(result, item)
					break
				}
			}
		}
	}

	if len(filters.ExclusiveFilters) > 0 {
		filtered := make([]shared.Item, 0, len(result))
		for _, item := range result {
			excluded := false
			for _, filter := range filters.ExclusiveFilters {
				if strings.Contains(strings.ToLower(item.DisplayName), strings.ToLower(filter)) {
					excluded = true
					break
				}
			}
			if !excluded {
				filtered = append(filtered, item)
			}
		}
		result = filtered
	}

	slices.SortFunc(result, func(a, b shared.Item) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	return result
}
