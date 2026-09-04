package ui

import (
	"context"
	"errors"
	"os"

	"grout/auth"
	"grout/imaging"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type DevicePairingInput struct {
	Host       settings.Host
	DeviceName string
}

type DevicePairingScreen struct{}

func NewDevicePairingScreen() *DevicePairingScreen {
	return &DevicePairingScreen{}
}

// qrDrawSize is both the size the QR is rendered at and the size it is drawn,
// so it stays sharp on the small screens these devices have.
const qrDrawSize = 320

// Execute shows a QR code for the user to scan and waits for them to approve
// the device in RomM.
func (s *DevicePairingScreen) Execute(input DevicePairingInput) auth.PairingResult {
	logger := gaba.GetLogger()

	pairing, err := auth.StartPairing(input.Host, input.DeviceName)
	if err != nil {
		logger.Error("Device auth init failed", "error", err)
		return auth.PairingResult{Outcome: auth.PairingFailed, Host: input.Host, Err: err}
	}

	// The QR encodes the verification URL with the code already in it, so
	// scanning is the whole approval. Without one the user is stuck, but the
	// poll still runs and a second device can approve it.
	qrPath, err := imaging.CreateTempQRCode(pairing.VerificationURL, qrDrawSize)
	if err != nil {
		logger.Warn("Unable to generate pairing QR code", "error", err)
		qrPath = ""
	} else {
		defer os.Remove(qrPath)
	}

	// Cancelling leaves the poll running in its own goroutine, so it needs
	// telling either way.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := gaba.ProcessMessage(
		localize("device_pairing_instructions", "Scan the QR Code to Pair"),
		gaba.ProcessMessageOptions{
			Image:        qrPath,
			ImageWidth:   qrDrawSize,
			ImageHeight:  qrDrawSize,
			CancelButton: constants.VirtualButtonB,
			FooterHelpItems: []gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			},
		},
		func() (auth.PairingResult, error) { return pairing.Wait(ctx), nil },
	)
	cancel()

	if err != nil && !errors.Is(err, gaba.ErrCancelled) {
		return auth.PairingResult{Outcome: auth.PairingFailed, Host: input.Host, Err: err}
	}

	// A cancel can land in the same moment the pairing completes. The token is
	// already issued by then, so honour it rather than throwing it away.
	if errors.Is(err, gaba.ErrCancelled) && result.Outcome != auth.PairingApproved {
		return auth.PairingResult{Outcome: auth.PairingCancelled, Host: input.Host}
	}

	return result
}
