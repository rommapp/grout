// Package download decides what a download run will fetch and where each file
// goes. It computes paths and URLs; it does no I/O and knows nothing about the
// screen that shows progress.
package download

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"

	"grout/cfw"
	"grout/files"
	"grout/gamelist"
	"grout/library"
	"grout/romm"
	"grout/settings"
	"grout/textmatch"
)

// Item is one file to fetch.
type Item struct {
	URL      string
	Location string
	GameName string
	// IsImage marks files that are normalised after download. Videos and
	// manuals are written as they arrive.
	IsImage bool
}

// Plan is everything a download run will fetch and record.
type Plan struct {
	// Roms are the game files themselves.
	Roms []Item
	// Art is cover art and its companions, fetched only after the roms land.
	Art []Item
	// Entries record what to write into the frontend's metadata once the roms
	// are in place.
	Entries []gamelist.RomGameEntry
}

// artSpec describes one kind of artwork: whether the user wants it, where it
// comes from, what it is called, and which field of the entry records it.
//
// These were eight near-identical blocks. The shape is the same for all of
// them, so adding a kind is a row rather than another copied block.
type artSpec struct {
	slot cfw.ArtSlot
	// wanted reports whether the user asked for this kind.
	wanted func(settings.Config) bool
	// sourceURL is where RomM serves it, or "" when this game has none.
	sourceURL func(romm.Rom, settings.Config, settings.Host) string
	// fixedExt names files that are not images, which take the game's base
	// name and this extension rather than a slot-derived name.
	fixedExt string
	// record stores the chosen path on the entry, when the firmware will read
	// it back.
	record func(paths *library.ArtPaths, path string, isESBased bool)
}

var artSpecs = []artSpec{
	{
		slot:      cfw.ArtCover,
		wanted:    func(settings.Config) bool { return true },
		sourceURL: func(g romm.Rom, c settings.Config, h settings.Host) string { return g.GetArtworkURL(c.ArtKind, h) },
		record:    func(p *library.ArtPaths, path string, _ bool) { p.Cover = path },
	},
	{
		slot:      cfw.ArtScreenshotPreview,
		wanted:    func(c settings.Config) bool { return c.DownloadArtScreenshotPreview },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetScreenshotURL(h) },
		record:    func(*library.ArtPaths, string, bool) {},
	},
	{
		slot: cfw.ArtThumbnail,
		wanted: func(c settings.Config) bool {
			return c.DownloadSplashArt != library.ArtKindNone || c.AdditionalDownloads.Thumbnail != library.ArtKindNone
		},
		sourceURL: func(g romm.Rom, c settings.Config, h settings.Host) string {
			kind := c.DownloadSplashArt
			if c.AdditionalDownloads.Thumbnail != library.ArtKindNone {
				kind = c.AdditionalDownloads.Thumbnail
			}
			return g.GetSplashArtURL(kind, h)
		},
		// Only the EmulationStation family reads a thumbnail back out of its
		// gamelist; elsewhere the file is written but not referenced.
		record: func(p *library.ArtPaths, path string, isESBased bool) {
			if isESBased {
				p.Thumbnail = path
			}
		},
	},
	{
		slot:   cfw.ArtMarquee,
		wanted: func(c settings.Config) bool { return c.AdditionalDownloads.Marquee != library.ArtKindNone },
		sourceURL: func(g romm.Rom, c settings.Config, h settings.Host) string {
			switch c.AdditionalDownloads.Marquee {
			case library.ArtKindMarquee:
				return g.GetMarqueeURL(h)
			case library.ArtKindLogo:
				return g.GetLogoURL(h)
			default:
				return ""
			}
		},
		record: func(p *library.ArtPaths, path string, _ bool) { p.Marquee = path },
	},
	{
		slot:      cfw.ArtVideo,
		wanted:    func(c settings.Config) bool { return c.AdditionalDownloads.Video },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetVideoURL(h) },
		fixedExt:  ".mp4",
		record:    func(p *library.ArtPaths, path string, _ bool) { p.Video = path },
	},
	{
		slot:      cfw.ArtBezel,
		wanted:    func(c settings.Config) bool { return c.AdditionalDownloads.Bezel },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetBezelURL(h) },
		record:    func(p *library.ArtPaths, path string, _ bool) { p.Bezel = path },
	},
	{
		slot:      cfw.ArtManual,
		wanted:    func(c settings.Config) bool { return c.AdditionalDownloads.Manual },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetManualURL(h) },
		fixedExt:  ".pdf",
		record:    func(p *library.ArtPaths, path string, _ bool) { p.Manual = path },
	},
	{
		slot:      cfw.ArtBoxback,
		wanted:    func(c settings.Config) bool { return c.AdditionalDownloads.BoxBack },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetBoxbackURL(h) },
		record:    func(p *library.ArtPaths, path string, _ bool) { p.BoxBack = path },
	},
	{
		slot:      cfw.ArtFanart,
		wanted:    func(c settings.Config) bool { return c.AdditionalDownloads.Fanart },
		sourceURL: func(g romm.Rom, _ settings.Config, h settings.Host) string { return g.GetFanartURL(h) },
		record:    func(p *library.ArtPaths, path string, _ bool) { p.Fanart = path },
	},
}

// MultiFileArchivePath is where a game that ships as several files is written
// while it downloads.
//
// It lands in a temp directory rather than the rom directory so a run that
// fails part way leaves no archive where the frontend would try to launch it.
// Planning and unpacking both need this path, so it is derived in one place.
func MultiFileArchivePath(game romm.Rom) string {
	return filepath.Join(files.TempDir(), fmt.Sprintf("grout_multirom_%d.zip", game.ID))
}

// RomLocation is where a game's rom file was written, or "" when the game was
// not part of the plan.
//
// A game shipping several versions is downloaded under the name of whichever
// one was chosen, so this is the only reliable way to find it afterwards.
func (p Plan) RomLocation(gameName string) string {
	for _, item := range p.Roms {
		if item.GameName == gameName {
			return item.Location
		}
	}
	return ""
}

// SetGamePath points a game's metadata entry at where its file actually ended
// up. Unpacking an archive moves it, and the entry is written afterwards.
func (p Plan) SetGamePath(fileName, path string) {
	for i := range p.Entries {
		if p.Entries[i].Game.FileName == fileName {
			p.Entries[i].Game.Path = path
			return
		}
	}
}

// SkipReason says why a game was left out of a plan.
type SkipReason struct {
	Game   romm.Rom
	Reason string
}

// BuildPlan works out what to fetch for the chosen games.
//
// selectedFileID picks one version of a game that ships several; zero takes the
// first. Games are skipped rather than failing the run, and each skip is
// reported so the caller can say why.
func BuildPlan(config settings.Config, host settings.Host, platform romm.Platform, games []romm.Rom, selectedFileID int) (Plan, []SkipReason) {
	// Resolved once: GetCFW re-reads the environment on every call.
	activeCFW := cfw.GetCFW()
	isESBased := activeCFW.IsBasedOnEmulationStation()

	plan := Plan{
		Roms:    make([]Item, 0, len(games)),
		Art:     make([]Item, 0, len(games)),
		Entries: make([]gamelist.RomGameEntry, 0, len(games)),
	}
	var skipped []SkipReason

	for _, game := range games {
		gamePlatform := platformFor(platform, game)
		romDirectory := cfw.PlatformRomDirectory(config, gamePlatform.FSSlug)

		rom, err := romItem(config, host, game, romDirectory, selectedFileID)
		if err != nil {
			skipped = append(skipped, SkipReason{Game: game, Reason: err.Error()})
			continue
		}
		plan.Roms = append(plan.Roms, rom)

		var artPaths library.ArtPaths
		if config.DownloadArt && hasCoverArt(game) {
			art := artItems(config, host, game, gamePlatform, activeCFW, isESBased, &artPaths)
			plan.Art = append(plan.Art, art...)
		}

		regions := game.Regions
		if config.GamelistOmitsRegion {
			regions = nil
		}

		plan.Entries = append(plan.Entries, gamelist.RomGameEntry{
			Game:         game.ToGame(textmatch.PrepareRomName(game.Name, regions), rom.Location, artPaths),
			Platform:     gamePlatform.ToPlatform(),
			RomDirectory: romDirectory,
		})
	}

	return plan, skipped
}

// platformFor prefers the game's own platform, which differs from the screen's
// when browsing a collection that spans several.
func platformFor(screen romm.Platform, game romm.Rom) romm.Platform {
	if screen.ID != 0 || game.PlatformID == 0 {
		return screen
	}
	return romm.Platform{
		ID:     game.PlatformID,
		FSSlug: game.PlatformFSSlug,
		Name:   game.PlatformDisplayName,
	}
}

func hasCoverArt(g romm.Rom) bool {
	return g.PathCoverLarge != "" || g.PathCoverSmall != "" || g.URLCover != ""
}

// romItem resolves where a game's file comes from and where it lands.
//
// A multi-disc game arrives as one archive in a temp directory and is expanded
// afterwards; everything else is written straight into the rom directory.
func romItem(config settings.Config, host settings.Host, game romm.Rom, romDirectory string, selectedFileID int) (Item, error) {
	if game.HasMultipleFiles {
		source, _ := url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(game.ID), "content", game.FsName)
		return Item{URL: source, Location: MultiFileArchivePath(game), GameName: game.Name}, nil
	}

	// A cached row written without a files array would panic on Files[0].
	// Refreshing the library repopulates it.
	if len(game.Files) == 0 {
		return Item{}, fmt.Errorf("no file metadata; refresh the library to repopulate it")
	}

	file := game.Files[0]
	if selectedFileID > 0 {
		for _, f := range game.Files {
			if f.ID == selectedFileID {
				file = f
				break
			}
		}
	}

	source, _ := url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(game.ID), "content", file.FileName)
	source += "?" + url.Values{"file_ids": {strconv.Itoa(file.ID)}}.Encode()

	return Item{
		URL:      source,
		Location: filepath.Join(romDirectory, file.FileName),
		GameName: game.Name,
	}, nil
}

func artItems(config settings.Config, host settings.Host, game romm.Rom, platform romm.Platform,
	activeCFW cfw.CFW, isESBased bool, paths *library.ArtPaths) []Item {

	items := make([]Item, 0, len(artSpecs))

	for _, spec := range artSpecs {
		if !spec.wanted(config) {
			continue
		}
		dir := cfw.PlatformArtDirectory(config, spec.slot, platform.FSSlug, platform.Name)
		if dir == "" {
			continue
		}
		source := spec.sourceURL(game, config, host)
		if source == "" {
			continue
		}

		name := cfw.ArtFileName(activeCFW, spec.slot, romArtFileName(game), game.FsNameNoExt)
		if spec.fixedExt != "" {
			name = game.FsNameNoExt + spec.fixedExt
		}

		location := filepath.Join(dir, name)
		spec.record(paths, location, isESBased)
		items = append(items, Item{
			URL:      source,
			Location: location,
			GameName: game.Name,
			IsImage:  spec.fixedExt == "",
		})
	}

	return items
}

// romArtFileName is the rom's file name for artwork naming, or "" when the game
// has no file list. Only MinUI uses it; see cfw.ArtFileName.
func romArtFileName(g romm.Rom) string {
	if len(g.Files) > 0 {
		return g.Files[0].FileName
	}
	return ""
}
