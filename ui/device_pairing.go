package ui

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"grout/cfw"
	"grout/internal"
	"grout/internal/imageutil"
	"grout/romm"
	"grout/version"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type DevicePairingOutcome int

const (
	DevicePairingCancelled DevicePairingOutcome = iota
	DevicePairingSuccess
	DevicePairingDenied
	DevicePairingExpired
	DevicePairingFailed
)

type DevicePairingInput struct {
	Host       romm.Host
	DeviceName string
}

type DevicePairingOutput struct {
	Outcome DevicePairingOutcome
	Host    romm.Host
	Err     error
}

type DevicePairingScreen struct{}

func NewDevicePairingScreen() *DevicePairingScreen {
	return &DevicePairingScreen{}
}

// pollResult carries the terminal state of the polling loop out of ProcessMessage.
type pollResult struct {
	Outcome DevicePairingOutcome
	Token   *romm.DeviceAuthTokenResponse
	Err     error
}

// pollTickDefault is how often the poll loop checks for cancellation while
// sleeping between server polls.
const pollTickDefault = 200 * time.Millisecond

func (s *DevicePairingScreen) Execute(input DevicePairingInput) DevicePairingOutput {
	logger := gaba.GetLogger()
	host := input.Host

	if host.ClientDeviceID == "" {
		host.ClientDeviceID = romm.NewClientDeviceID()
	}

	client := romm.NewClient(host.URL(), romm.WithInsecureSkipVerify(host.InsecureSkipVerify))

	initResp, err := client.InitDeviceAuth(romm.DeviceAuthInitRequest{
		ClientDeviceIdentifier: host.ClientDeviceID,
		Name:                   input.DeviceName,
		Client:                 "grout",
		Platform:               string(cfw.GetCFW()),
		ClientVersion:          version.Get().Version,
		RequestedScopes:        romm.DeviceAuthScopes,
	})
	if err != nil {
		logger.Error("Device auth init failed", "error", err)
		return DevicePairingOutput{Outcome: DevicePairingFailed, Host: host, Err: err}
	}

	verificationURL := host.URL() + initResp.VerificationPathComplete
	qrPath, err := imageutil.CreateTempQRCode(verificationURL, 300)
	if err != nil {
		logger.Warn("Unable to generate pairing QR code", "error", err)
		qrPath = ""
	} else {
		defer os.Remove(qrPath)
	}

	message := fmt.Sprintf("%s\n%s%s\n\n%s %s",
		i18n.Localize(&goi18n.Message{ID: "device_pairing_instructions", Other: "Scan the QR code or visit:"}, nil),
		host.URL(), initResp.VerificationPath,
		i18n.Localize(&goi18n.Message{ID: "device_pairing_code", Other: "Code:"}, nil),
		initResp.UserCode,
	)

	cancelled := &atomic.Bool{}

	result, msgErr := gaba.ProcessMessage(message, gaba.ProcessMessageOptions{
		Image:        qrPath,
		ImageWidth:   300,
		ImageHeight:  300,
		CancelButton: constants.VirtualButtonB,
		FooterHelpItems: []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil)},
		},
	}, func() (pollResult, error) {
		return s.poll(client, initResp, cancelled, pollTickDefault), nil
	})

	if msgErr != nil && errors.Is(msgErr, gaba.ErrCancelled) {
		// Stop the background polling goroutine promptly; the server-side
		// pairing request expires on its own.
		cancelled.Store(true)
		return DevicePairingOutput{Outcome: DevicePairingCancelled, Host: host}
	}

	if result.Outcome != DevicePairingSuccess {
		return DevicePairingOutput{Outcome: result.Outcome, Host: host, Err: result.Err}
	}

	token := result.Token
	host.Token = token.AccessToken
	host.TokenName = input.DeviceName
	host.TokenExpiresAt = token.ExpiresAt
	host.DeviceID = token.DeviceID
	host.DeviceName = input.DeviceName
	host.DeviceClientVersion = version.Get().Version

	if missing := romm.MissingSyncScopes(token.Scopes); len(missing) > 0 {
		logger.Warn("Paired token is missing scopes needed for save sync",
			"missing", missing, "granted", token.Scopes)
	}

	if host.Username == "" {
		authClient := romm.NewClientFromHost(host, internal.LoginTimeout)
		if user, err := authClient.GetCurrentUser(); err == nil {
			host.Username = user.Username
		}
	}

	return DevicePairingOutput{Outcome: DevicePairingSuccess, Host: host}
}

// poll drives the device-auth polling loop. It returns when pairing reaches a
// terminal state or cancelled is set (checked every tick so B stays responsive).
func (s *DevicePairingScreen) poll(client *romm.Client, initResp *romm.DeviceAuthInitResponse, cancelled *atomic.Bool, tick time.Duration) pollResult {
	logger := gaba.GetLogger()

	interval := time.Duration(initResp.Interval) * time.Second
	if interval <= 0 {
		interval = tick // server gave no interval; poll at the tick rate
	}
	deadline := time.Now().Add(time.Duration(initResp.ExpiresIn) * time.Second)

	consecutiveErrors := 0
	for {
		// Sleep one interval in short slices so cancellation stays responsive.
		sleepUntil := time.Now().Add(interval)
		for {
			if cancelled.Load() {
				return pollResult{Outcome: DevicePairingCancelled}
			}
			if !time.Now().Before(sleepUntil) {
				break
			}
			time.Sleep(tick)
		}

		if time.Now().After(deadline) {
			return pollResult{Outcome: DevicePairingExpired}
		}

		token, state, err := client.PollDeviceToken(initResp.DeviceCode)
		if err != nil {
			consecutiveErrors++
			logger.Warn("Device token poll failed", "error", err, "consecutive", consecutiveErrors)
			if consecutiveErrors >= 3 {
				return pollResult{Outcome: DevicePairingFailed, Err: err}
			}
			continue
		}
		consecutiveErrors = 0

		switch state {
		case romm.DeviceAuthSuccess:
			return pollResult{Outcome: DevicePairingSuccess, Token: token}
		case romm.DeviceAuthSlowDown:
			interval += 5 * time.Second
		case romm.DeviceAuthDenied:
			return pollResult{Outcome: DevicePairingDenied}
		case romm.DeviceAuthExpired:
			return pollResult{Outcome: DevicePairingExpired}
		case romm.DeviceAuthPending:
			// keep waiting
		}
	}
}
