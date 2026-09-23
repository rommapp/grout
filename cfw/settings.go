package cfw

import (
	"grout/settings"
)

// PlatformRomDirectory returns where a platform's roms live on this device,
// honouring the user's directory mapping and the server's platform binding.
func PlatformRomDirectory(config settings.Config, fsSlug string) string {
	relativePath := fsSlug
	if mapping, ok := config.DirectoryMappings[fsSlug]; ok && mapping.RelativePath != "" {
		relativePath = mapping.RelativePath
	}
	return GetPlatformRomDirectory(relativePath, config.ResolveFSSlug(fsSlug))
}

// PlatformArtDirectory returns where a kind of artwork goes for a platform, or
// "" when the firmware keeps none. Callers treat "" as "skip", not as an error.
func PlatformArtDirectory(config settings.Config, slot ArtSlot, fsSlug, platformName string) string {
	return ArtDirectory(GetCFW(), slot, PlatformRomDirectory(config, fsSlug), fsSlug, platformName)
}
