package auth

import (
	"context"
	"log/slog"
	"time"

	"grout/romm"
	"grout/settings"
)

const (
	settleAttempts = 6
	settleBackoff  = 700 * time.Millisecond
)

// settleToken waits for a freshly issued token to start working.
//
// RomM can reject reads with a token for a moment after issuing it, while its
// scopes take effect. Waiting that out here, with the pairing screen still up,
// keeps the race from surfacing later as a failed platform load on first
// launch. A token that never works is left alone, so a real permission problem
// still surfaces downstream rather than being hidden here.
func settleToken(ctx context.Context, host settings.Host) {
	settleTokenWith(ctx, romm.NewClientFromHost(host, settings.LoginTimeout), settleAttempts, settleBackoff)
}

// settleTokenWith is the retry itself, with the timing injected so tests do not
// wait on it.
func settleTokenWith(ctx context.Context, client *romm.Client, attempts int, backoff time.Duration) {
	for attempt := range attempts {
		if ctx.Err() != nil {
			return
		}
		if err := client.ValidateToken(); err == nil {
			return
		}
		if attempt < attempts-1 && !sleep(ctx, backoff) {
			return
		}
	}

	slog.Default().Warn("Device token not usable for reads after pairing, continuing anyway")
}
