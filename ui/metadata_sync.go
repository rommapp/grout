package ui

import (
	"fmt"

	"grout/catalog"
	"grout/download"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	uatomic "go.uber.org/atomic"
)

type MetadataSyncInput struct {
	Config settings.Config
	Host   settings.Host
}

type MetadataSyncOutput struct{}

// MetadataSyncScreen rewrites the metadata of games already on the device from
// what the server knows now, for when a download was cut short or the server's
// metadata changed since. What the firmware recorded about each game, such as
// play count or favourites, is kept.
type MetadataSyncScreen struct{}

func NewMetadataSyncScreen() *MetadataSyncScreen {
	return &MetadataSyncScreen{}
}

func (s *MetadataSyncScreen) Execute(input MetadataSyncInput) MetadataSyncOutput {
	s.draw(input)
	return MetadataSyncOutput{}
}

func (s *MetadataSyncScreen) draw(input MetadataSyncInput) {
	platforms, err := catalog.MappedPlatforms(input.Host, input.Config.DirectoryMappings, input.Config.ApiTimeout.Duration())
	if err != nil {
		gaba.GetLogger().Error("Failed to fetch platforms", "error", err)
		s.tell(fmt.Sprintf("Failed to fetch platforms: %v", err))
		return
	}
	if len(platforms) == 0 {
		s.tell(localize("artwork_sync_no_platforms", "No platforms with directory mappings found."))
		return
	}

	found, paths, failed := s.scan(input, platforms)
	if len(failed) > 0 {
		s.tell(fmt.Sprintf(localize("metadata_sync_refresh_failed", "Could not get the latest metadata for %d platforms from RomM.\nThey were skipped."), len(failed)))
	}
	if len(found) == 0 {
		// A run where every platform failed has already said so.
		if len(failed) == 0 {
			s.tell(localize("metadata_sync_no_games", "No downloaded games found."))
		}
		return
	}

	chosen, ok := choosePlatforms(found, localize("button_update", "Update"))
	if !ok {
		return
	}

	s.refresh(input, chosen, paths)
}

// scan finds the downloaded games of each platform, and the file holding each
// one keyed by game id, showing progress since it reads every platform's
// library and looks for each game on the card.
//
// Each platform is first brought up to date from the server: the cache only
// catches up at startup, and writing what it held then would undo the point of
// the run. A platform that cannot be refreshed is skipped rather than written
// from possibly stale data, and its name is returned in failed.
func (s *MetadataSyncScreen) scan(input MetadataSyncInput, platforms []romm.Platform) (found []platformRoms, paths map[int]string, failed []string) {
	logger := gaba.GetLogger()

	paths = make(map[int]string)
	for i, platform := range platforms {
		progress := uatomic.NewFloat64(0)
		// ProcessMessage runs the closure on the calling goroutine, so
		// appending from inside is safe
		gaba.ProcessMessage(
			fmt.Sprintf(localize("artwork_sync_scanning", "Scanning platform %d/%d: %s..."), i+1, len(platforms), platform.Name),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (any, error) {
				games, err := catalog.RefreshGames(catalog.GameSource{Platform: platform}, progress)
				if err != nil {
					logger.Error("Failed to refresh platform games, skipping it", "platform", platform.Name, "error", err)
					failed = append(failed, platform.Name)
					return nil, nil
				}

				downloaded := make([]romm.Rom, 0, len(games))
				for _, game := range games {
					if path := catalog.LocalRomPath(input.Config, game); path != "" {
						downloaded = append(downloaded, game)
						paths[game.ID] = path
					}
				}
				if len(downloaded) > 0 {
					found = append(found, platformRoms{platform: platform, roms: downloaded})
				}
				return nil, nil
			},
		)
	}
	return found, paths, failed
}

func (s *MetadataSyncScreen) refresh(input MetadataSyncInput, chosen []platformRoms, paths map[int]string) {
	batches := make([]download.PlatformGames, 0, len(chosen))
	for _, entry := range chosen {
		games := make([]download.LocalGame, 0, len(entry.roms))
		for _, rom := range entry.roms {
			games = append(games, download.LocalGame{Rom: rom, Path: paths[rom.ID]})
		}
		batches = append(batches, download.PlatformGames{Platform: entry.platform, Games: games})
	}

	result, err := gaba.ProcessMessage(
		localize("metadata_sync_updating", "Updating metadata..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (download.MetadataResult, error) {
			return download.RefreshMetadata(input.Config, batches)
		},
	)
	if err != nil {
		gaba.GetLogger().Error("Metadata update failed", "error", err)
	}
	gaba.GetLogger().Info("Metadata update complete", "updated", result.Updated, "failed", result.Failed)

	switch {
	case result.Updated == 0 && err != nil:
		s.tell(localize("metadata_sync_failed", "Could not update the metadata.\nCheck the logs for more info."))
	case result.Failed > 0:
		s.tell(fmt.Sprintf(localize("metadata_sync_partial", "Updated metadata for %d games, %d failed.\nCheck the logs for more info."), result.Updated, result.Failed))
	default:
		s.tell(fmt.Sprintf(localize("metadata_sync_complete", "Updated metadata for %d games."), result.Updated))
	}
}

func (s *MetadataSyncScreen) tell(message string) {
	gaba.ConfirmationMessage(message, ContinueFooter(), gaba.MessageOptions{})
}
