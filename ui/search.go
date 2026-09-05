package ui

import (
	"errors"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
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
	res, err := gaba.Keyboard(input.InitialText, localize("help_exit_text", "Press any button to close help"))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return SearchOutput{Action: SearchActionCancel}, nil
		}
		gaba.GetLogger().Error("Error with keyboard", "error", err)
		return SearchOutput{Action: SearchActionCancel}, err
	}

	return SearchOutput{Action: SearchActionApply, Query: res.Text}, nil
}
