package ui

import (
	"errors"
	"grout/saves"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveConflictInput struct {
	Items           []saves.SyncItem
	AllItems        []saves.SyncItem // Full items list (passed through for transition)
	ConflictIndices map[int]int      // Conflict index → AllItems index (passed through)
	SessionID       int              // Sync session ID (passed through)
}

type SaveConflictOutput struct {
	Action          SaveConflictAction
	Items           []saves.SyncItem
	AllItems        []saves.SyncItem // Passed through from input
	ConflictIndices map[int]int      // Passed through from input
	SessionID       int              // Sync session ID (passed through)
}

type SaveConflictScreen struct{}

func NewSaveConflictScreen() *SaveConflictScreen {
	return &SaveConflictScreen{}
}

func (s *SaveConflictScreen) Draw(input SaveConflictInput) (SaveConflictOutput, error) {
	output := SaveConflictOutput{
		Action:          SaveConflictActionCancel,
		Items:           input.Items,
		AllItems:        input.AllItems,
		ConflictIndices: input.ConflictIndices,
		SessionID:       input.SessionID,
	}

	items := s.buildMenuItems(input.Items)

	title := localize("save_conflict_title", "Resolve Conflicts")

	result, err := gaba.OptionsList(
		title,
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: 0,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Save conflict screen error", "error", err)
		return output, err
	}

	s.applyResolutions(input.Items, result.Items)
	output.Action = SaveConflictActionResolved
	output.Items = input.Items

	return output, nil
}

func (s *SaveConflictScreen) buildMenuItems(conflicts []saves.SyncItem) []gaba.ItemWithOptions {
	skip := localize("save_conflict_skip", "Skip")
	keepLocal := localize("save_conflict_keep_local", "Keep Local")
	keepRemote := localize("save_conflict_keep_remote", "Keep Remote")

	items := make([]gaba.ItemWithOptions, 0, len(conflicts))

	for _, item := range conflicts {
		items = append(items, gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: item.LocalSave.RomName},
			// Skip is the default: a conflict is only acted on if the user actively picks
			// Keep Local or Keep Remote, so confirming through never silently overwrites.
			Options: []gaba.Option{
				{DisplayName: skip, Value: "skip"},
				{DisplayName: keepLocal, Value: "local"},
				{DisplayName: keepRemote, Value: "remote"},
			},
			SelectedOption: 0,
		})
	}

	return items
}

func (s *SaveConflictScreen) applyResolutions(conflicts []saves.SyncItem, resultItems []gaba.ItemWithOptions) {
	for i := range conflicts {
		if i >= len(resultItems) {
			break
		}
		selected := resultItems[i].SelectedOption
		if selected >= 0 && selected < len(resultItems[i].Options) {
			switch resultItems[i].Options[selected].Value {
			case "local":
				conflicts[i].Resolve(saves.ActionUpload)
				conflicts[i].ForceOverwrite = true
			case "remote":
				conflicts[i].Resolve(saves.ActionDownload)
			case "skip":
				// Leave it as ActionConflict: not executed this run, re-offered next sync.
			}
		}
	}
}
