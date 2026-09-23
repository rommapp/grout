package auth

import (
	"context"
	"log/slog"
	"time"

	"grout/cfw"
	"grout/romm"
	"grout/settings"
	"grout/version"
)

// PairingOutcome says how a device pairing ended.
type PairingOutcome int

const (
	// PairingCancelled is the zero value: the user backed out. Nothing went
	// wrong, so it carries no error and needs nothing said about it.
	PairingCancelled PairingOutcome = iota
	PairingApproved
	PairingDenied
	PairingExpired
	PairingFailed
)

const (
	// defaultPollInterval is what the device-flow spec suggests when a server
	// names no interval. Falling back to something faster would have grout
	// hammering a server that simply left the field out.
	defaultPollInterval = 5 * time.Second
	// defaultPairingLifetime is how long to keep offering a request whose
	// server did not say when it expires. The server rejects the code once it
	// really has expired, so this only bounds the wait.
	defaultPairingLifetime = 15 * time.Minute
	// slowDownStep is added to the interval each time the server asks grout to
	// back off.
	slowDownStep = 5 * time.Second
	// maxPollFailures is how many network errors in a row end the attempt. One
	// or two are normal on a handheld's wifi.
	maxPollFailures = 3
)

// Pairing is a device-authorization request the server has issued and the user
// has not answered yet.
type Pairing struct {
	// VerificationURL is what the user opens to approve this device. It carries
	// the code, so scanning it is the whole approval.
	VerificationURL string

	client     *romm.Client
	host       settings.Host
	deviceName string
	deviceCode string
	interval   time.Duration
	// slowDown is added to interval each time the server asks grout to back off.
	slowDown  time.Duration
	expiresAt time.Time
}

// PairingResult is how a pairing ended, with the host holding the new token
// when it worked.
type PairingResult struct {
	Outcome PairingOutcome
	Host    settings.Host
	Err     error
}

// StartPairing asks the server to open a pairing request. The caller shows the
// returned URL and then calls Wait.
func StartPairing(host settings.Host, deviceName string) (*Pairing, error) {
	// The identifier is what lets the server recognise this device again on a
	// later pairing, so it is generated once and kept.
	if host.ClientDeviceID == "" {
		host.ClientDeviceID = settings.NewClientDeviceID()
	}

	client := romm.NewClient(host.URL(), romm.WithInsecureSkipVerify(host.InsecureSkipVerify))

	response, err := client.InitDeviceAuth(romm.DeviceAuthInitRequest{
		ClientDeviceIdentifier: host.ClientDeviceID,
		Name:                   deviceName,
		Client:                 "grout",
		Platform:               string(cfw.GetCFW()),
		ClientVersion:          version.Get().Version,
		RequestedScopes:        romm.DeviceAuthScopes,
	})
	if err != nil {
		return nil, err
	}

	interval := time.Duration(response.Interval) * time.Second
	if interval <= 0 {
		interval = defaultPollInterval
	}

	lifetime := time.Duration(response.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = defaultPairingLifetime
	}

	return &Pairing{
		VerificationURL: host.URL() + response.VerificationPathComplete,
		client:          client,
		host:            host,
		deviceName:      deviceName,
		deviceCode:      response.DeviceCode,
		interval:        interval,
		slowDown:        slowDownStep,
		expiresAt:       time.Now().Add(lifetime),
	}, nil
}

// Wait blocks until the user answers, the request expires, or ctx is done.
//
// On approval the returned host carries the new token, checked against the
// server before being handed back.
func (p *Pairing) Wait(ctx context.Context) PairingResult {
	outcome, token, err := p.poll(ctx)
	if outcome != PairingApproved {
		return PairingResult{Outcome: outcome, Host: p.host, Err: err}
	}

	host := p.host
	host.Token = token.AccessToken
	host.TokenName = p.deviceName
	host.TokenExpiresAt = token.ExpiresAt
	host.DeviceID = token.DeviceID
	host.DeviceName = p.deviceName
	host.DeviceClientVersion = version.Get().Version

	if missing := romm.MissingSyncScopes(token.Scopes); len(missing) > 0 {
		slog.Default().Warn("Paired token is missing scopes needed for save sync",
			"missing", missing, "granted", token.Scopes)
	}

	settleToken(ctx, host)

	// Shown on the settings screen. Not worth failing a working pairing over.
	if host.Username == "" {
		if user, err := romm.NewClientFromHost(host, settings.LoginTimeout).GetCurrentUser(); err == nil {
			host.Username = user.Username
		}
	}

	return PairingResult{Outcome: PairingApproved, Host: host}
}

// poll asks the server for the token until the request reaches a state worth
// reporting.
func (p *Pairing) poll(ctx context.Context) (PairingOutcome, *romm.DeviceAuthTokenResponse, error) {
	interval := p.interval
	failures := 0

	for {
		if !sleep(ctx, interval) {
			return PairingCancelled, nil, nil
		}
		if time.Now().After(p.expiresAt) {
			return PairingExpired, nil, nil
		}

		token, state, err := p.client.PollDeviceToken(p.deviceCode)
		if err != nil {
			failures++
			slog.Default().Warn("Device token poll failed", "error", err, "consecutive", failures)
			if failures >= maxPollFailures {
				return PairingFailed, nil, err
			}
			continue
		}
		failures = 0

		switch state {
		case romm.DeviceAuthSuccess:
			return PairingApproved, token, nil
		case romm.DeviceAuthDenied:
			return PairingDenied, nil, nil
		case romm.DeviceAuthExpired:
			return PairingExpired, nil, nil
		case romm.DeviceAuthSlowDown:
			interval += p.slowDown
		case romm.DeviceAuthPending:
			// Nobody has answered yet.
		}
	}
}

// sleep waits for d, reporting false if ctx finished first.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
