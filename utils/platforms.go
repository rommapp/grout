package utils

import (
	"fmt"
	"grout/models"

	"github.com/brandonkowalski/go-romm"
)

func GetMappedPlatforms(host models.Host, mappings map[string]models.DirectoryMapping) []romm.Platform {
	c := romm.NewClient(host.URL(), romm.WithBasicAuth(host.Username, host.Password))

	rommPlatforms, err := c.GetPlatforms()
	if err != nil {
		LogStandardFatal(fmt.Sprintf("Failed to get platforms from RomM: %s", err), nil)
	}

	var platforms []romm.Platform

	for _, platform := range rommPlatforms {
		_, exists := mappings[platform.Slug]
		if exists {
			platforms = append(platforms, romm.Platform{
				Name: platform.Name,
				ID:   platform.ID,
				Slug: platform.Slug,
			})
		}
	}

	return platforms
}
