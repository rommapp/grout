package ui

import (
	"errors"
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

	// The QR encodes the full verification URL (including the user code), so
	// scanning is all that's needed to approve. ProcessMessage stacks this
	// instruction beneath the QR.
	message := i18n.Localize(&goi18n.Message{ID: "device_pairing_instructions", Other: "Scan the QR Code to Pair"}, nil)

	verificationURL := host.URL() + initResp.VerificationPathComplete
	const qrDrawSize = 320
	qrPath, err := imageutil.CreateTempQRCode(verificationURL, qrDrawSize)
	if err != nil {
		logger.Warn("Unable to generate pairing QR code", "error", err)
		qrPath = ""
	} else {
		defer os.Remove(qrPath)
	}

	cancelled := &atomic.Bool{}

	result, msgErr := gaba.ProcessMessage(message, gaba.ProcessMessageOptions{
		Image:        qrPath,
		ImageWidth:   qrDrawSize,
		ImageHeight:  qrDrawSize,
		CancelButton: constants.VirtualButtonB,
		FooterHelpItems: []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil)},
		},
	}, func() (pollResult, error) {
		res := s.poll(client, initResp, cancelled, pollTickDefault)
		if res.Outcome == DevicePairingSuccess && res.Token != nil {
			// The token was minted microseconds ago; RomM can briefly reject
			// resource reads with it before its scopes take effect. Wait that
			// window out here — while the pairing screen is still up — so the
			// first platform load after login doesn't hit the race.
			warm := host
			warm.Token = res.Token.AccessToken
			warmUpToken(warm, cancelled)
		}
		return res, nil
	})

	// Stop the background poller no matter how ProcessMessage exited; harmless
	// if the poll loop already returned.
	cancelled.Store(true)

	if msgErr != nil {
		// A cancel that raced pairing completion still has the token — honor
		// the success instead of discarding an issued credential.
		if errors.Is(msgErr, gaba.ErrCancelled) && result.Outcome != DevicePairingSuccess {
			return DevicePairingOutput{Outcome: DevicePairingCancelled, Host: host}
		}
		if !errors.Is(msgErr, gaba.ErrCancelled) {
			return DevicePairingOutput{Outcome: DevicePairingFailed, Host: host, Err: msgErr}
		}
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

// warmUpTokenAttempts and warmUpTokenBackoff bound how long pairing waits for a
// freshly issued token to become effective for resource reads.
const (
	warmUpTokenAttempts = 6
	warmUpTokenBackoff  = 700 * time.Millisecond
)

// warmUpToken retries a platforms read (the same call first-launch setup makes)
// until the just-issued token works, the user cancels, or the attempts are
// exhausted. RomM can transiently 403 a token in the moment right after
// approval; retrying here keeps that race from cascading into a failed — and
// silently fatal — platform load on first launch. A token that never succeeds
// is left to the normal downstream error handling, so a genuine permission
// problem still surfaces rather than being masked.
func warmUpToken(host romm.Host, cancelled *atomic.Bool) {
	client := romm.NewClientFromHost(host, internal.LoginTimeout)
	warmUpTokenWith(client, cancelled, warmUpTokenAttempts, warmUpTokenBackoff)
}

// warmUpTokenWith is the retry core, with attempts and backoff injected so it
// can be tested without real delays.
func warmUpTokenWith(client *romm.Client, cancelled *atomic.Bool, attempts int, backoff time.Duration) {
	for attempt := 0; attempt < attempts; attempt++ {
		if cancelled.Load() {
			return
		}
		if err := client.ValidateToken(); err == nil {
			return
		}
		if attempt < attempts-1 {
			time.Sleep(backoff)
		}
	}
	gaba.GetLogger().Warn("Device token not effective for platform reads after pairing; continuing")
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
