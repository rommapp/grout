// Package auth connects grout to a RomM server and turns a pairing code into a
// token. It does the work behind the login screen and knows nothing about how
// any of it is drawn.
package auth

import (
	"errors"
	"fmt"
	"log/slog"

	"grout/romm"
	"grout/settings"
)

var (
	// ErrNoCode means the pairing code field was left empty.
	ErrNoCode = errors.New("no pairing code entered")
	// ErrInvalidCode is a code the server would not accept: unknown, already
	// used, expired, or tried too many times.
	ErrInvalidCode = errors.New("invalid or expired pairing code")
)

// Capabilities is what a reachable server offers.
type Capabilities struct {
	// SupportsDevicePairing is RomM 5.0 and later, where the user approves the
	// device in a browser instead of typing a code.
	SupportsDevicePairing bool
}

// Connect checks that a server is reachable and reports what it supports.
func Connect(host settings.Host) (Capabilities, error) {
	client := romm.NewClient(host.URL(),
		romm.WithInsecureSkipVerify(host.InsecureSkipVerify),
		romm.WithTimeout(settings.ValidationTimeout))

	if err := client.ValidateConnection(); err != nil {
		return Capabilities{}, err
	}

	// Older servers have no field for this. Failing to read it only costs the
	// user the nicer pairing flow, so it is not worth failing the connection.
	heartbeat, err := client.GetHeartbeat()
	if err != nil {
		slog.Default().Debug("Could not read heartbeat, assuming no device pairing", "error", err)
		return Capabilities{}, nil
	}

	return Capabilities{SupportsDevicePairing: heartbeat.SupportsDeviceAuth()}, nil
}

// ExchangeCode trades a pairing code for a token and returns the host holding
// it.
//
// The token is checked against the server before being handed back, so a code
// that exchanges but grants nothing usable fails here rather than on the next
// screen.
func ExchangeCode(host settings.Host, code string) (settings.Host, error) {
	if code == "" {
		return host, ErrNoCode
	}

	token, err := romm.ExchangeToken(host.URL(), code, host.InsecureSkipVerify)
	if err != nil {
		// Only the exchange can say a code is bad, so the reading happens here
		// rather than in Classify, where a 404 could mean anything.
		if errors.Is(err, romm.ErrNotFound) || errors.Is(err, romm.ErrRateLimited) {
			return host, fmt.Errorf("%w: %w", ErrInvalidCode, err)
		}
		return host, err
	}

	host.Token = token.RawToken
	host.TokenName = token.Name
	host.TokenExpiresAt = token.ExpiresAt

	if missing := romm.MissingSyncScopes(token.Scopes); len(missing) > 0 {
		slog.Default().Warn("Paired token is missing scopes needed for save sync",
			"missing", missing, "granted", token.Scopes)
	}

	client := romm.NewClientFromHost(host, settings.LoginTimeout)
	if err := client.ValidateToken(); err != nil {
		return host, err
	}

	// Shown on the settings screen. Not worth failing a working login over.
	if host.Username == "" {
		if user, err := client.GetCurrentUser(); err == nil {
			host.Username = user.Username
		}
	}

	return host, nil
}

// CheckToken reports whether a host is reachable and the token grout already
// holds still works against it. It is what a changed server address has to
// pass before being saved.
func CheckToken(host settings.Host) error {
	if err := romm.NewClientFromHost(host, settings.ValidationTimeout).ValidateConnection(); err != nil {
		return err
	}
	return romm.NewClientFromHost(host, settings.LoginTimeout).ValidateToken()
}
