package ui

import (
	"errors"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SearchInput struct {
	InitialText string
}

type SearchOutput struct {
	Action SearchAction
	Query  string
}

type SearchScreen struct{}

func NewSearchScreen() *SearchScreen {
	return &SearchScreen{}
}

func (s *SearchScreen) Draw(input SearchInput) (SearchOutput, error) {
	res, err := gaba.Keyboard(input.InitialText, i18n.Localize(&goi18n.Message{ID: "help_exit_text", Other: "Press any button to close help"}, nil))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return SearchOutput{Action: SearchActionCancel}, nil
		}
		gaba.GetLogger().Error("Error with keyboard", "error", err)
		return SearchOutput{Action: SearchActionCancel}, err
	}

	return SearchOutput{Action: SearchActionApply, Query: res.Text}, nil
}
