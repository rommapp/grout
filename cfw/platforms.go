package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/knulli"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/rocknix"
	"grout/cfw/spruce"
	"grout/cfw/trimui"
	"strings"
)

// platformAliasMap is computed from platform mappings - slugs that map to
// overlapping local folders are considered aliases (e.g., sfam/snes both map to "snes")
var platformAliasMap = buildPlatformAliasMap()

func buildPlatformAliasMap() map[string][]string {
	// Combine all platform maps to find aliases across all CFWs
	allMaps := []map[string][]string{
		knulli.Platforms,
		muos.Platforms,
		nextui.Platforms,
		rocknix.Platforms,
		spruce.Platforms,
		trimui.Platforms,
		allium.Platforms,
	}

	// Build reverse map: primary folder -> list of RomM slugs that use it as primary
	// Only use the FIRST folder in each list (the primary/default folder)
	// This avoids false aliases like arcade/neogeoaes which share "neogeo" as a secondary folder
	primaryFolderToSlugs := make(map[string]map[string]bool)
	for _, platformMap := range allMaps {
		for slug, folders := range platformMap {
			if len(folders) == 0 {
				continue
			}
			// Use only the primary (first) folder
			primary := strings.ToLower(folders[0])
			if primaryFolderToSlugs[primary] == nil {
				primaryFolderToSlugs[primary] = make(map[string]bool)
			}
			primaryFolderToSlugs[primary][slug] = true
		}
	}

	// Find slug groups that share the same primary folder using union-find
	parent := make(map[string]string)
	var find func(s string) string
	find = func(s string) string {
		if parent[s] == "" {
			parent[s] = s
		}
		if parent[s] != s {
			parent[s] = find(parent[s])
		}
		return parent[s]
	}
	union := func(a, b string) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}

	// Union slugs that share the same primary folder
	for _, slugs := range primaryFolderToSlugs {
		var slugList []string
		for slug := range slugs {
			slugList = append(slugList, slug)
		}
		for i := 1; i < len(slugList); i++ {
			union(slugList[0], slugList[i])
		}
	}

	// Group slugs by their root parent
	groups := make(map[string][]string)
	for slug := range parent {
		root := find(slug)
		groups[root] = append(groups[root], slug)
	}

	// Build final alias map (only for groups with more than one slug)
	result := make(map[string][]string)
	for _, group := range groups {
		if len(group) > 1 {
			for _, slug := range group {
				result[slug] = group
			}
		}
	}

	return result
}

// GetPlatformAliases returns all equivalent platform slugs for the given slug.
// Aliases are RomM slugs that map to overlapping local folders across CFWs.
// Returns a slice containing at least the input slug itself.
func GetPlatformAliases(fsSlug string) []string {
	if aliases, ok := platformAliasMap[fsSlug]; ok {
		return aliases
	}
	return []string{fsSlug}
}

// GetPlatformMap returns the platform mapping for the given CFW.
func GetPlatformMap(c CFW) map[string][]string {
	switch c {
	case MuOS:
		return muos.Platforms
	case NextUI:
		return nextui.Platforms
	case Knulli:
		return knulli.Platforms
	case Spruce:
		return spruce.Platforms
	case ROCKNIX:
		return rocknix.Platforms
	case Trimui:
		return trimui.Platforms
	case Allium:
		return allium.Platforms
	default:
		return nil
	}
}

// RomMFSSlugToCFW converts a RomM filesystem slug to the CFW-specific folder name.
func RomMFSSlugToCFW(fsSlug string) string {
	cfwPlatformMap := GetPlatformMap(GetCFW())
	if cfwPlatformMap == nil {
		return strings.ToLower(fsSlug)
	}

	if value, ok := cfwPlatformMap[fsSlug]; ok {
		if len(value) > 0 {
			return value[0]
		}
		return ""
	}

	return strings.ToLower(fsSlug)
}
