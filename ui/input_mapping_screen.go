package ui

import (
	"time"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type InputMappingOutput struct {
	// Saved means a new mapping is on disk. The toolkit reads it once at
	// startup, so the caller ends the app for it to take effect.
	Saved bool
}

type InputMappingScreen struct{}

func NewInputMappingScreen() *InputMappingScreen {
	return &InputMappingScreen{}
}

// Execute walks the user through pressing every button, so a device whose
// layout the toolkit guesses wrong can be told what it really has.
func (s *InputMappingScreen) Execute() InputMappingOutput {
	mapping := gaba.ShowInputCapture(gaba.InputCaptureOptions{
		Title:             localize("input_capture_title", "Grout Input Mapping"),
		InstructionText:   localize("input_capture_instruction", "Press and hold each button when prompted."),
		ReleasedEarlyText: localize("input_capture_released_early", "Released too early!"),
		CompleteText:      localize("input_capture_complete", "Input Mapping Complete!"),
		HoldDuration:      500 * time.Millisecond,
	})
	if mapping == nil {
		return InputMappingOutput{}
	}

	data, err := mapping.ToJSON()
	if err == nil {
		err = mapping.SaveToJSON(settings.InputMappingFileName)
	}
	if err != nil {
		// Without this the screen would simply close, which is what it does
		// when the user backs out. There would be nothing to tell the two
		// apart, and the buttons they just pressed would be gone.
		gaba.GetLogger().Error("Failed to save input mapping", "error", err)
		gaba.ConfirmationMessage(
			localize("input_mapping_save_failed", "Could not save the input mapping.\nCheck the logs for more info."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return InputMappingOutput{}
	}

	gaba.SetInputMappingBytes(data)

	gaba.ConfirmationMessage(
		localize("input_mapping_saved", "Input mapping saved.\nGrout needs to restart to apply changes."),
		[]gaba.FooterHelpItem{{ButtonName: "A", HelpText: localize("button_exit", "Exit")}},
		gaba.MessageOptions{},
	)

	return InputMappingOutput{Saved: true}
}
