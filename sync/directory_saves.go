package sync

// DirectorySavePlatforms maps RomM fs_slug to platform slugs where saves are
// stored as directories (e.g., PPSSPP uses Game ID folders containing save files)
// rather than individual files alongside other saves.
//
// These require special handling during sync: the entire directory must be zipped
// for upload and unzipped on download. Matching saves to ROMs requires Game ID
// resolution rather than filename matching.
var DirectorySavePlatforms = map[string]bool{
	"psp": true,
}

// IsDirectorySavePlatform returns true if the platform stores saves as directories.
func IsDirectorySavePlatform(fsSlug string) bool {
	return DirectorySavePlatforms[fsSlug]
}
