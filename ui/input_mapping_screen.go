package ui

import (
	"os"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type InputMappingScreen struct{}

func NewInputMappingScreen() *InputMappingScreen {
	return &InputMappingScreen{}
}

func (s *InputMappingScreen) Execute() {
	mapping := gaba.ShowInputCapture(gaba.InputCaptureOptions{
		Title:             i18n.Localize(&goi18n.Message{ID: "input_capture_title", Other: "Grout Input Mapping"}, nil),
		InstructionText:   i18n.Localize(&goi18n.Message{ID: "input_capture_instruction", Other: "Press and hold each button when prompted."}, nil),
		ReleasedEarlyText: i18n.Localize(&goi18n.Message{ID: "input_capture_released_early", Other: "Released too early!"}, nil),
		CompleteText:      i18n.Localize(&goi18n.Message{ID: "input_capture_complete", Other: "Input Mapping Complete!"}, nil),
		HoldDuration:      500 * time.Millisecond,
	})
	if mapping == nil {
		return
	}

	data, err := mapping.ToJSON()
	if err != nil {
		gaba.GetLogger().Error("Failed to serialize input mapping", "error", err)
		return
	}

	if err := mapping.SaveToJSON("input_mapping.json"); err != nil {
		gaba.GetLogger().Error("Failed to save input mapping", "error", err)
		return
	}

	gaba.SetInputMappingBytes(data)

	gaba.ConfirmationMessage(
		i18n.Localize(&goi18n.Message{ID: "input_mapping_saved", Other: "Input mapping saved.\nGrout needs to restart to apply changes."}, nil),
		[]gaba.FooterHelpItem{
			{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_exit", Other: "Exit"}, nil)},
		},
		gaba.MessageOptions{},
	)
	os.Exit(0)
}
