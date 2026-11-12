package cfw

import (
	"grout/archive"
	"grout/files"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// playlistExt names the playlist that represents a multi-disc game.
const playlistExt = ".m3u"

// RomLayout describes how one rom is stored on disk. It holds no server
// identifiers: whether a rom has been downloaded is a filesystem question.
type RomLayout struct {
	// BaseName excludes the extension. A multi-disc game is "<BaseName>.m3u".
	BaseName string
	// MultiDisc means the discs are listed in a playlist.
	MultiDisc bool
	// FileNames are the rom's files, as the server names them.
	FileNames []string
	// ArchiveMembers maps an archive in FileNames to the files inside it. An
	// archive unpacked on the device is deleted, so what came out of it stands
	// in for it.
	ArchiveMembers map[string][]string
	// ExtractedDir is the folder a multi-file game is unpacked into, when the
	// firmware names it other than BaseName.
	ExtractedDir string
}

// DownloadPath returns where the rom's file lives under romDir, or "" if it
// has none.
func (l RomLayout) DownloadPath(romDir string) string {
	if romDir == "" {
		return ""
	}
	if l.MultiDisc {
		return filepath.Join(romDir, l.BaseName+playlistExt)
	}
	if len(l.FileNames) > 0 {
		return filepath.Join(romDir, l.FileNames[0])
	}
	return ""
}

// IsDownloaded reports whether the rom is present under romDir. A multi-disc
// game needs its playlist or the folder it was unpacked into; a game with
// several versions needs any one of them.
func (l RomLayout) IsDownloaded(romDir string) bool {
	return l.LocalPath(romDir) != ""
}

// LocalPath returns the file that makes the rom present under romDir: the
// playlist of a multi-disc game, or the first of its versions found. An
// archive unpacked on the device is replaced by what came out of it. It is ""
// when the rom is not on the device.
func (l RomLayout) LocalPath(romDir string) string {
	if romDir == "" {
		return ""
	}

	if l.MultiDisc {
		if playlist := filepath.Join(romDir, l.BaseName+playlistExt); files.FileExists(playlist) {
			return playlist
		}
		if l.BaseName == "" {
			return ""
		}
		return unpackedFolderPath(filepath.Join(romDir, l.extractedDir()))
	}

	for _, name := range l.FileNames {
		if path := filePath(romDir, name, l.ArchiveMembers[name]); path != "" {
			return path
		}
	}
	return ""
}

func (l RomLayout) extractedDir() string {
	if l.ExtractedDir != "" {
		return l.ExtractedDir
	}
	return l.BaseName
}

// IsFileDownloaded reports whether one named file of a rom is present.
// members are the files inside it when it is an archive.
func IsFileDownloaded(romDir, fileName string, members []string) bool {
	if romDir == "" || fileName == "" {
		return false
	}
	return filePath(romDir, fileName, members) != ""
}

// filePath finds a file under romDir, or what came out of it when it is an
// archive grout unpacks, and returns the path to launch. It is "" when neither
// is there.
//
// The archive's contents are known exactly when the server listed them. When it
// did not, a file sharing the archive's name is taken as its contents, which is
// how No-Intro and Redump sets are packed.
func filePath(romDir, fileName string, members []string) string {
	if path := filepath.Join(romDir, fileName); files.FileExists(path) {
		return path
	}
	if !archive.IsArchive(fileName) {
		return ""
	}
	if len(members) > 0 {
		return unpackedMembersPath(romDir, members)
	}
	return unpackedSiblingPath(romDir, fileName)
}

// unpackedMembersPath checks that every file listed inside an archive is under
// romDir, and returns the one to launch: the playlist if there is one, since it
// ties several discs together, and otherwise the first file.
//
// Member names use forward slashes and may sit in a folder; a folder's own
// entry ends in one and says nothing about its files.
func unpackedMembersPath(romDir string, members []string) string {
	launch := ""
	for _, member := range members {
		if strings.HasSuffix(member, "/") {
			continue
		}
		member = filepath.FromSlash(member)
		if member == "" || !filepath.IsLocal(member) {
			continue
		}
		path := filepath.Join(romDir, member)
		if !files.FileExists(path) {
			return ""
		}
		if launch == "" || isPlaylist(member) && !isPlaylist(launch) {
			launch = path
		}
	}
	return launch
}

func isPlaylist(name string) bool {
	return strings.EqualFold(filepath.Ext(name), playlistExt)
}

// notGameExtensions are files that sit beside a rom under its name without
// being it: other archives, saves, patches, artwork and notes.
var notGameExtensions = map[string]bool{
	".zip": true, ".7z": true,
	".srm": true, ".sav": true,
	".ips": true, ".bps": true, ".ups": true, ".cht": true,
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".bmp": true,
	".txt": true, ".nfo": true, ".xml": true, ".json": true, ".cfg": true,
}

func isGameExtension(ext string) bool {
	ext = strings.ToLower(ext)
	// RetroArch numbers its save states: .state, .state1, .state2...
	return !notGameExtensions[ext] && !strings.HasPrefix(ext, ".state")
}

// unpackedSiblingPath finds a file named like the archive, or a folder named
// exactly like it, that could have come out of it. A folder resolves to what
// to launch inside it, as unpacking chose it.
func unpackedSiblingPath(romDir, archiveName string) string {
	stem := strings.TrimSuffix(archiveName, filepath.Ext(archiveName))
	if stem == "" {
		return ""
	}
	name := romDirListings.stems(romDir)[strings.ToLower(stem)]
	if name == "" {
		return ""
	}
	path := filepath.Join(romDir, name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return unpackedFolderPath(path)
	}
	return path
}

// unpackedFolderPath returns what to launch from the folder a multi-file game
// was unpacked into: its playlist, else its first file, else the folder itself
// when it holds only folders. It is "" when the folder is missing or empty.
func unpackedFolderPath(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	firstFile, hasFolders := "", false
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			hasFolders = true
			continue
		}
		if isPlaylist(name) {
			return filepath.Join(dir, name)
		}
		if firstFile == "" {
			firstFile = filepath.Join(dir, name)
		}
	}

	switch {
	case firstFile != "":
		return firstFile
	case hasFolders:
		return dir
	}
	return ""
}

// romDirListings remembers which names a rom directory holds, so a list of a
// thousand zipped games reads the directory once rather than once per game.
var romDirListings = &dirListings{entries: map[string]dirListing{}}

// mtimeGranularity is the coarsest a card's filesystem records a change: FAT
// keeps modification times to two seconds.
const mtimeGranularity = 2 * time.Second

type dirListing struct {
	modTime  time.Time
	listedAt time.Time
	stems    map[string]string
}

type dirListings struct {
	mu      sync.Mutex
	entries map[string]dirListing
}

// stems maps the lowercased name, without extension, of each game in dir to
// its file name. A playlist wins over other files sharing its name, and a file
// over a folder.
//
// A listing is reused while the directory's modification time is unchanged.
// One taken within mtimeGranularity of that time is not: a change landing in
// the same tick would leave the time as it was.
func (d *dirListings) stems(dir string) map[string]string {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	d.mu.Lock()
	cached, ok := d.entries[dir]
	d.mu.Unlock()
	if ok && cached.modTime.Equal(info.ModTime()) && !cached.listedAt.Before(cached.modTime.Add(mtimeGranularity)) {
		return cached.stems
	}

	listedAt := time.Now()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	stems := make(map[string]string, len(entries))
	var folders []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			folders = append(folders, name)
			continue
		}
		ext := filepath.Ext(name)
		if !isGameExtension(ext) {
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(name, ext))
		if stems[key] == "" || isPlaylist(name) {
			stems[key] = name
		}
	}
	for _, name := range folders {
		if key := strings.ToLower(name); stems[key] == "" {
			stems[key] = name
		}
	}

	d.mu.Lock()
	d.entries[dir] = dirListing{modTime: info.ModTime(), listedAt: listedAt, stems: stems}
	d.mu.Unlock()
	return stems
}
