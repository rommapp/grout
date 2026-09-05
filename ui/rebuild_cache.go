package ui

import (
	"fmt"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	uatomic "go.uber.org/atomic"

	"grout/cache"
	"grout/catalog"
	"grout/romm"
	"grout/settings"
)

type RebuildCacheInput struct {
	Host      settings.Host
	Config    *settings.Config
	CacheSync *cache.BackgroundSync
}

type RebuildCacheOutput struct {
	Action           RebuildCacheAction
	UpdatedPlatforms []romm.Platform
}

type RebuildCacheAction int

const (
	RebuildCacheActionComplete RebuildCacheAction = iota
	RebuildCacheActionError
)

type RebuildCacheScreen struct{}

func NewRebuildCacheScreen() *RebuildCacheScreen {
	return &RebuildCacheScreen{}
}

func (s *RebuildCacheScreen) Draw(input RebuildCacheInput) (RebuildCacheOutput, error) {
	output := RebuildCacheOutput{Action: RebuildCacheActionComplete}

	scope, chosen := askCacheScope()
	if !chosen {
		return output, nil
	}

	// The background sync writes to the same tables, so it has to stand down
	// for the duration and be told the cache is current afterwards.
	if input.CacheSync != nil {
		input.CacheSync.Stop()
		defer input.CacheSync.SetSynced()
	}

	if err := catalog.ClearCache(input.Host, *input.Config, scope); err != nil {
		return reportRebuildFailure(err)
	}

	if !scope.ClearsMetadata() {
		return output, nil
	}

	platforms, err := rebuildMetadataWithProgress(input)
	if err != nil {
		return reportRebuildFailure(err)
	}

	output.UpdatedPlatforms = platforms
	return output, nil
}

// askCacheScope asks what to clear, reporting false when the user backs out.
//
// The choice is carried as a catalog.CacheScope rather than the selected
// label, so translating the menu cannot change what the screen does.
func askCacheScope() (catalog.CacheScope, bool) {
	result, err := gaba.SelectionMessage(
		localize("cache_clear_prompt", "What would you like to clear?"),
		[]gaba.SelectionOption{
			{DisplayName: localize("cache_clear_metadata", "Metadata"), Value: catalog.ScopeMetadata},
			{DisplayName: localize("cache_clear_artwork", "Artwork"), Value: catalog.ScopeArtwork},
			{DisplayName: localize("cache_clear_both", "All"), Value: catalog.ScopeAll},
		},
		[]gaba.FooterHelpItem{FooterContinue(), FooterCancel()},
		gaba.SelectionMessageSettings{},
	)
	if err != nil {
		return 0, false
	}

	scope, ok := result.SelectedValue.(catalog.CacheScope)
	return scope, ok
}

func rebuildMetadataWithProgress(input RebuildCacheInput) ([]romm.Platform, error) {
	progress := uatomic.NewFloat64(0)

	var platforms []romm.Platform
	_, err := gaba.ProcessMessage(
		localize("cache_building", "Building cache..."),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     true,
			Progress:            progress,
		},
		func() (any, error) {
			var err error
			platforms, err = catalog.RebuildMetadata(input.Host, *input.Config, progress)
			return nil, err
		},
	)
	if err != nil {
		return nil, err
	}
	return platforms, nil
}

// reportRebuildFailure tells the user, rather than returning to a menu that
// looks as though the rebuild worked.
func reportRebuildFailure(err error) (RebuildCacheOutput, error) {
	gaba.GetLogger().Error("Cache rebuild failed", "error", err)
	gaba.ConfirmationMessage(
		fmt.Sprintf("%s: %v",
			localize("cache_rebuild_failed", "Cache rebuild failed"), err),
		ContinueFooter(),
		gaba.MessageOptions{},
	)
	return RebuildCacheOutput{Action: RebuildCacheActionError}, err
}
