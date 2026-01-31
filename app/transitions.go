package main

import (
	"grout/cfw"
	"grout/internal"
	"grout/ui"
	"os"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/router"
)

type transitionContext struct {
	state           *AppState
	stack           *router.Stack
	quitOnBack      bool
	showCollections bool
}

func buildTransitionFunc(state *AppState, quitOnBack bool, showCollections bool) router.TransitionFunc {
	return func(from router.Screen, result any, stack *router.Stack) (router.Screen, any) {
		ctx := &transitionContext{
			state:           state,
			stack:           stack,
			quitOnBack:      quitOnBack,
			showCollections: showCollections,
		}

		switch from {
		case ScreenPlatformSelection:
			return transitionPlatformSelection(ctx, result)
		case ScreenGameList:
			return transitionGameList(ctx, result)
		case ScreenSearch:
			return transitionSearch(ctx, result)
		case ScreenGameDetails:
			return transitionGameDetails(ctx, result)
		case ScreenGameOptions:
			return transitionGameOptions(ctx, result)
		case ScreenGameQR:
			return popOrExit(stack)
		case ScreenCollectionList:
			return transitionCollectionList(ctx, result)
		case ScreenCollectionPlatformSelection:
			return transitionCollectionPlatformSelection(ctx, result)
		case ScreenSettings:
			return transitionSettings(ctx, result)
		case ScreenGeneralSettings:
			return transitionGeneralSettings(ctx, result)
		case ScreenCollectionsSettings:
			return transitionCollectionsSettings(ctx, result)
		case ScreenAdvancedSettings:
			return transitionAdvancedSettings(ctx, result)
		case ScreenPlatformMapping:
			return transitionPlatformMapping(ctx, result)
		case ScreenSaveSyncSettings:
			return transitionSaveSyncSettings(ctx, result)
		case ScreenInfo:
			return transitionInfo(ctx, result)
		case ScreenLogoutConfirmation:
			return transitionLogoutConfirmation(ctx, result)
		case ScreenRebuildCache:
			return transitionRebuildCache(ctx, result)
		case ScreenSaveSync:
			return popOrExit(stack)
		case ScreenBIOSDownload:
			return popOrExit(stack)
		case ScreenArtworkSync:
			return popOrExit(stack)
		case ScreenUpdateCheck:
			return transitionUpdateCheck(ctx, result)
		}

		return router.ScreenExit, nil
	}
}

func transitionPlatformSelection(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.PlatformSelectionOutput)

	if len(r.ReorderedPlatforms) > 0 {
		savePlatformOrder(ctx.state, r.ReorderedPlatforms)
	}

	pushInput := ui.PlatformSelectionInput{
		Platforms:       &ctx.state.Platforms,
		QuitOnBack:      ctx.quitOnBack,
		ShowCollections: ctx.showCollections,
		ShowSaveSync:    computeShowSaveSync(ctx.state),
	}

	switch r.Action {
	case ui.PlatformSelectionActionSelected:
		ctx.stack.Push(ScreenPlatformSelection, pushInput, r)
		return ScreenGameList, ui.GameListInput{
			Config:   ctx.state.Config,
			Host:     ctx.state.Host,
			Platform: r.SelectedPlatform,
		}

	case ui.PlatformSelectionActionCollections:
		ctx.stack.Push(ScreenPlatformSelection, pushInput, r)
		return ScreenCollectionList, ui.CollectionSelectionInput{
			Config: ctx.state.Config,
			Host:   ctx.state.Host,
		}

	case ui.PlatformSelectionActionSettings:
		ctx.stack.Push(ScreenPlatformSelection, pushInput, r)
		return ScreenSettings, ui.SettingsInput{
			Config: ctx.state.Config,
			CFW:    ctx.state.CFW,
			Host:   ctx.state.Host,
		}

	case ui.PlatformSelectionActionSaveSync:
		ctx.stack.Push(ScreenPlatformSelection, pushInput, r)
		return ScreenSaveSync, ui.SaveSyncInput{
			Config: ctx.state.Config,
			Host:   ctx.state.Host,
		}

	case ui.PlatformSelectionActionQuit:
		return router.ScreenExit, nil
	}

	return router.ScreenExit, nil
}

func transitionGameList(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.GameListOutput)

	pushInput := ui.GameListInput{
		Config:       ctx.state.Config,
		Host:         ctx.state.Host,
		Platform:     r.Platform,
		Collection:   r.Collection,
		Games:        r.AllGames,
		HasBIOS:      r.HasBIOS,
		SearchFilter: r.SearchFilter,
	}

	switch r.Action {
	case ui.GameListActionSelected:
		if len(r.SelectedGames) > 1 {
			executeMultiDownloadUI(ctx.state, r)
			triggerAutoSyncRouter(ctx.state)
			return ScreenGameList, ui.GameListInput{
				Config:               ctx.state.Config,
				Host:                 ctx.state.Host,
				Platform:             r.Platform,
				Collection:           r.Collection,
				Games:                r.AllGames,
				HasBIOS:              r.HasBIOS,
				SearchFilter:         r.SearchFilter,
				LastSelectedIndex:    r.LastSelectedIndex,
				LastSelectedPosition: r.LastSelectedPosition,
			}
		}

		ctx.stack.Push(ScreenGameList, pushInput, r)
		return ScreenGameDetails, ui.GameDetailsInput{
			Config:   ctx.state.Config,
			Host:     ctx.state.Host,
			Platform: r.Platform,
			Game:     r.SelectedGames[0],
		}

	case ui.GameListActionSearch:
		ctx.stack.Push(ScreenGameList, pushInput, r)
		return ScreenSearch, ui.SearchInput{
			InitialText: r.SearchFilter,
		}

	case ui.GameListActionClearSearch:
		return ScreenGameList, ui.GameListInput{
			Config:     ctx.state.Config,
			Host:       ctx.state.Host,
			Platform:   r.Platform,
			Collection: r.Collection,
			Games:      r.AllGames,
			HasBIOS:    r.HasBIOS,
		}

	case ui.GameListActionBIOS:
		ctx.stack.Push(ScreenGameList, pushInput, r)
		return ScreenBIOSDownload, ui.BIOSDownloadInput{
			Config:   *ctx.state.Config,
			Host:     ctx.state.Host,
			Platform: r.Platform,
		}

	case ui.GameListActionBack:
		return popOrExit(ctx.stack)
	}

	return router.ScreenExit, nil
}

func transitionSearch(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.SearchOutput)

	entry := ctx.stack.Pop()
	if entry == nil {
		return router.ScreenExit, nil
	}

	switch r.Action {
	case ui.SearchActionApply:
		if entry.Screen == ScreenGameList {
			prevInput := entry.Input.(ui.GameListInput)
			return ScreenGameList, ui.GameListInput{
				Config:       prevInput.Config,
				Host:         prevInput.Host,
				Platform:     prevInput.Platform,
				Collection:   prevInput.Collection,
				Games:        prevInput.Games,
				HasBIOS:      prevInput.HasBIOS,
				SearchFilter: r.Query,
			}
		}
		if entry.Screen == ScreenCollectionList {
			prevInput := entry.Input.(ui.CollectionSelectionInput)
			return ScreenCollectionList, ui.CollectionSelectionInput{
				Config:       prevInput.Config,
				Host:         prevInput.Host,
				SearchFilter: r.Query,
			}
		}

	case ui.SearchActionCancel:
		if entry.Screen == ScreenGameList {
			prevInput := entry.Input.(ui.GameListInput)
			prevResume := entry.Resume.(ui.GameListOutput)
			return ScreenGameList, ui.GameListInput{
				Config:               prevInput.Config,
				Host:                 prevInput.Host,
				Platform:             prevInput.Platform,
				Collection:           prevInput.Collection,
				Games:                prevInput.Games,
				HasBIOS:              prevInput.HasBIOS,
				SearchFilter:         prevInput.SearchFilter,
				LastSelectedIndex:    prevResume.LastSelectedIndex,
				LastSelectedPosition: prevResume.LastSelectedPosition,
			}
		}
		if entry.Screen == ScreenCollectionList {
			prevInput := entry.Input.(ui.CollectionSelectionInput)
			prevResume := entry.Resume.(ui.CollectionSelectionOutput)
			return ScreenCollectionList, ui.CollectionSelectionInput{
				Config:               prevInput.Config,
				Host:                 prevInput.Host,
				SearchFilter:         prevInput.SearchFilter,
				LastSelectedIndex:    prevResume.LastSelectedIndex,
				LastSelectedPosition: prevResume.LastSelectedPosition,
			}
		}
	}

	return router.ScreenExit, nil
}

func transitionGameDetails(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.GameDetailsOutput)

	switch r.Action {
	case ui.GameDetailsActionDownload:
		executeDownloadUI(ctx.state, r, ctx.stack)
		triggerAutoSyncRouter(ctx.state)
		return popOrExit(ctx.stack)

	case ui.GameDetailsActionOptions:
		ctx.stack.Push(ScreenGameDetails, ui.GameDetailsInput{
			Config:   ctx.state.Config,
			Host:     ctx.state.Host,
			Platform: r.Platform,
			Game:     r.Game,
		}, nil)
		return ScreenGameOptions, ui.GameOptionsInput{
			Config: ctx.state.Config,
			Host:   ctx.state.Host,
			Game:   r.Game,
		}

	case ui.GameDetailsActionBack:
		return popOrExit(ctx.stack)
	}

	return router.ScreenExit, nil
}

func transitionGameOptions(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.GameOptionsOutput)
	if r.Config != nil {
		ctx.state.Config = r.Config
	}

	if r.Action == ui.GameOptionsActionShowQR {
		ctx.stack.Push(ScreenGameOptions, ui.GameOptionsInput{
			Config: ctx.state.Config,
			Host:   r.Host,
			Game:   r.Game,
		}, nil)
		return ScreenGameQR, ui.GameQRInput{
			Host: r.Host,
			Game: r.Game,
		}
	}

	return popOrExit(ctx.stack)
}

func transitionCollectionList(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.CollectionSelectionOutput)

	pushInput := ui.CollectionSelectionInput{
		Config:       ctx.state.Config,
		Host:         ctx.state.Host,
		SearchFilter: r.SearchFilter,
	}

	switch r.Action {
	case ui.CollectionListActionSelected:
		ctx.stack.Push(ScreenCollectionList, pushInput, r)
		return ScreenCollectionPlatformSelection, ui.CollectionPlatformSelectionInput{
			Config:     ctx.state.Config,
			Host:       ctx.state.Host,
			Collection: r.SelectedCollection,
		}

	case ui.CollectionListActionSearch:
		ctx.stack.Push(ScreenCollectionList, pushInput, r)
		return ScreenSearch, ui.SearchInput{
			InitialText: r.SearchFilter,
		}

	case ui.CollectionListActionClearSearch:
		return ScreenCollectionList, ui.CollectionSelectionInput{
			Config: ctx.state.Config,
			Host:   ctx.state.Host,
		}

	case ui.CollectionListActionBack:
		return popOrExit(ctx.stack)
	}

	return router.ScreenExit, nil
}

func transitionCollectionPlatformSelection(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.CollectionPlatformSelectionOutput)

	switch r.Action {
	case ui.CollectionPlatformSelectionActionSelected:
		games := r.AllGames

		// In unified mode (ID=0), the platform selection screen was skipped,
		// so don't push it to the stack - back should go to collection list
		if r.SelectedPlatform.ID != 0 {
			ctx.stack.Push(ScreenCollectionPlatformSelection, ui.CollectionPlatformSelectionInput{
				Config:      ctx.state.Config,
				Host:        ctx.state.Host,
				Collection:  r.Collection,
				CachedGames: r.AllGames,
			}, r)
			games = filterGamesByPlatform(r.AllGames, r.SelectedPlatform.ID)
		}

		return ScreenGameList, ui.GameListInput{
			Config:     ctx.state.Config,
			Host:       ctx.state.Host,
			Platform:   r.SelectedPlatform,
			Collection: r.Collection,
			Games:      games,
		}

	case ui.CollectionPlatformSelectionActionBack:
		return popOrExit(ctx.stack)
	}

	return router.ScreenExit, nil
}

func transitionSettings(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.SettingsOutput)

	if r.Config != nil {
		ctx.state.Config = r.Config
		internal.SaveConfig(ctx.state.Config)
		ctx.showCollections = ctx.state.Config.ShowCollections(ctx.state.Host)
	}

	pushInput := ui.SettingsInput{Config: ctx.state.Config, CFW: ctx.state.CFW, Host: ctx.state.Host}

	switch r.Action {
	case ui.SettingsActionGeneral:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenGeneralSettings, ui.GeneralSettingsInput{Config: ctx.state.Config}

	case ui.SettingsActionCollections:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenCollectionsSettings, ui.CollectionsSettingsInput{Config: ctx.state.Config}

	case ui.SettingsActionAdvanced:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenAdvancedSettings, ui.AdvancedSettingsInput{Config: ctx.state.Config, Host: ctx.state.Host}

	case ui.SettingsActionPlatformMapping:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenPlatformMapping, ui.PlatformMappingInput{
			Host:             ctx.state.Host,
			ApiTimeout:       ctx.state.Config.ApiTimeout,
			CFW:              ctx.state.CFW,
			RomDirectory:     cfw.GetRomDirectory(),
			ExistingMappings: ctx.state.Config.DirectoryMappings,
			PlatformsBinding: ctx.state.Config.PlatformsBinding,
		}

	case ui.SettingsActionSaveSync:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenSaveSyncSettings, ui.SaveSyncSettingsInput{Config: ctx.state.Config, CFW: ctx.state.CFW}

	case ui.SettingsActionInfo:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenInfo, ui.InfoInput{Host: ctx.state.Host}

	case ui.SettingsActionCheckUpdate:
		ctx.stack.Push(ScreenSettings, pushInput, r)
		return ScreenUpdateCheck, ui.UpdateInput{
			CFW:            ctx.state.CFW,
			ReleaseChannel: ctx.state.Config.ReleaseChannel,
			Host:           &ctx.state.Host,
		}

	case ui.SettingsActionSaved, ui.SettingsActionBack:
		return popOrExit(ctx.stack)
	}

	return router.ScreenExit, nil
}

func transitionGeneralSettings(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.GeneralSettingsOutput)
	if r.Config != nil {
		ctx.state.Config = r.Config
	}
	return popOrExit(ctx.stack)
}

func transitionCollectionsSettings(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.CollectionsSettingsOutput)
	if r.SyncNeeded && ctx.state.CacheSync != nil {
		ctx.state.CacheSync.SyncCollections()
	}
	ctx.showCollections = ctx.state.Config.ShowCollections(ctx.state.Host)
	return popOrExit(ctx.stack)
}

func transitionAdvancedSettings(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.AdvancedSettingsOutput)

	pushInput := ui.AdvancedSettingsInput{Config: ctx.state.Config, Host: ctx.state.Host}

	switch r.Action {
	case ui.AdvancedSettingsActionRebuildCache:
		ctx.stack.Push(ScreenAdvancedSettings, pushInput, r)
		return ScreenRebuildCache, ui.RebuildCacheInput{
			Host:   ctx.state.Host,
			Config: ctx.state.Config,
		}

	case ui.AdvancedSettingsActionSyncArtwork:
		ctx.stack.Push(ScreenAdvancedSettings, pushInput, r)
		return ScreenArtworkSync, ui.ArtworkSyncInput{
			Config: *ctx.state.Config,
			Host:   ctx.state.Host,
		}

	default:
		if ctx.state.AutoUpdate != nil {
			ctx.state.AutoUpdate.Recheck(ctx.state.Config.ReleaseChannel)
		}
		return popOrExit(ctx.stack)
	}
}

func transitionPlatformMapping(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.PlatformMappingOutput)
	if r.Action == ui.PlatformMappingActionSaved {
		handlePlatformMappingUpdateUI(ctx.state, r)
	}
	return popOrExit(ctx.stack)
}

func transitionSaveSyncSettings(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.SaveSyncSettingsOutput)
	if r.Config != nil {
		ctx.state.Config = r.Config
	}
	triggerAutoSyncRouter(ctx.state)
	return popOrExit(ctx.stack)
}

func transitionInfo(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.InfoOutput)
	if r.Action == ui.InfoActionLogout {
		ctx.stack.Push(ScreenInfo, ui.InfoInput{Host: ctx.state.Host}, nil)
		return ScreenLogoutConfirmation, nil
	}
	return popOrExit(ctx.stack)
}

func transitionLogoutConfirmation(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.LogoutConfirmationOutput)
	if r.Action == ui.LogoutConfirmationActionConfirm {
		handleLogout(ctx.state)
		ctx.stack.Clear()
		return ScreenPlatformSelection, ui.PlatformSelectionInput{
			Platforms:       &ctx.state.Platforms,
			QuitOnBack:      ctx.quitOnBack,
			ShowCollections: ctx.state.Config.ShowCollections(ctx.state.Host),
			ShowSaveSync:    computeShowSaveSync(ctx.state),
		}
	}
	return popOrExit(ctx.stack)
}

func transitionRebuildCache(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.RebuildCacheOutput)
	if len(r.UpdatedPlatforms) > 0 {
		ctx.state.Platforms = r.UpdatedPlatforms
	}
	return popOrExit(ctx.stack)
}

func transitionUpdateCheck(ctx *transitionContext, result any) (router.Screen, any) {
	r := result.(ui.UpdateOutput)
	if r.UpdatePerformed {
		os.Exit(0)
	}
	return popOrExit(ctx.stack)
}

func popOrExit(stack *router.Stack) (router.Screen, any) {
	entry := stack.Pop()
	if entry == nil {
		return router.ScreenExit, nil
	}

	switch input := entry.Input.(type) {
	case ui.PlatformSelectionInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.PlatformSelectionOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastSelectedPosition = output.LastSelectedPosition
		}
		return entry.Screen, input

	case ui.GameListInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.GameListOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastSelectedPosition = output.LastSelectedPosition
		}
		return entry.Screen, input

	case ui.CollectionSelectionInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.CollectionSelectionOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastSelectedPosition = output.LastSelectedPosition
		}
		return entry.Screen, input

	case ui.CollectionPlatformSelectionInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.CollectionPlatformSelectionOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastSelectedPosition = output.LastSelectedPosition
		}
		return entry.Screen, input

	case ui.SettingsInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.SettingsOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastVisibleStartIndex = output.LastVisibleStartIndex
		}
		return entry.Screen, input

	case ui.AdvancedSettingsInput:
		if entry.Resume != nil {
			output := entry.Resume.(ui.AdvancedSettingsOutput)
			input.LastSelectedIndex = output.LastSelectedIndex
			input.LastVisibleStartIndex = output.LastVisibleStartIndex
		}
		return entry.Screen, input

	default:
		return entry.Screen, entry.Input
	}
}
