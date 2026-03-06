package ui

import (
	"errors"
	"grout/sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveConflictInput struct {
	Items           []sync.SyncItem
	AllItems        []sync.SyncItem // Full items list (passed through for transition)
	ConflictIndices map[int]int     // Conflict index → AllItems index (passed through)
}

type SaveConflictOutput struct {
	Action          SaveConflictAction
	Items           []sync.SyncItem
	AllItems        []sync.SyncItem // Passed through from input
	ConflictIndices map[int]int     // Passed through from input
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
	}

	items := s.buildMenuItems(input.Items)

	title := i18n.Localize(&goi18n.Message{ID: "save_conflict_title", Other: "Resolve Conflicts"}, nil)

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

func (s *SaveConflictScreen) buildMenuItems(conflicts []sync.SyncItem) []gaba.ItemWithOptions {
	keepLocal := i18n.Localize(&goi18n.Message{ID: "save_conflict_keep_local", Other: "Keep Local"}, nil)
	keepRemote := i18n.Localize(&goi18n.Message{ID: "save_conflict_keep_remote", Other: "Keep Remote"}, nil)

	items := make([]gaba.ItemWithOptions, 0, len(conflicts))

	for _, item := range conflicts {
		items = append(items, gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: item.LocalSave.RomName},
			Options: []gaba.Option{
				{DisplayName: keepLocal, Value: "local"},
				{DisplayName: keepRemote, Value: "remote"},
			},
			SelectedOption: 0,
		})
	}

	return items
}

func (s *SaveConflictScreen) applyResolutions(conflicts []sync.SyncItem, resultItems []gaba.ItemWithOptions) {
	for i := range conflicts {
		if i >= len(resultItems) {
			break
		}
		selected := resultItems[i].SelectedOption
		if selected >= 0 && selected < len(resultItems[i].Options) {
			switch resultItems[i].Options[selected].Value {
			case "local":
				conflicts[i].Resolve(sync.ActionUpload)
				conflicts[i].ForceOverwrite = true
			case "remote":
				conflicts[i].Resolve(sync.ActionDownload)
			}
		}
	}
}
