package romm

import "strings"

// notPlayable are the RomM file categories that ride along with a game but are
// not a version of it. RomM sets a file's category from the folder it sits in,
// so a manuals folder beside the roms files its PDFs as "manual".
var notPlayable = map[string]bool{
	"manual": true, "walkthrough": true, "soundtrack": true, "screenshot": true,
	"cheat": true, "patch": true, "update": true, "dlc": true,
}

// Playable reports whether a file is something to launch rather than an extra.
// Files RomM leaves uncategorised, like those in a versions folder, count.
func (f RomFile) Playable() bool {
	category, _ := f.Category.(string)
	return !notPlayable[strings.ToLower(category)]
}

// PlayableFiles are the versions of a game a player can pick between, in
// RomM's order. A game whose files all look like extras keeps them all, so it
// always has something to download.
func (r Rom) PlayableFiles() []RomFile {
	var playable []RomFile
	for _, file := range r.Files {
		if file.Playable() {
			playable = append(playable, file)
		}
	}
	if len(playable) == 0 {
		return r.Files
	}
	return playable
}

// ArchiveMemberNames lists the files RomM found inside the file, when it is an
// archive the server looked into.
func (f RomFile) ArchiveMemberNames() []string {
	if len(f.ArchiveMembers) == 0 {
		return nil
	}
	names := make([]string, 0, len(f.ArchiveMembers))
	for _, member := range f.ArchiveMembers {
		names = append(names, member.Name)
	}
	return names
}
