package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rommapp/grout/core/romm"
)

var (
	ErrNoCode      = errors.New("no pairing code entered")
	ErrInvalidCode = errors.New("invalid or expired pairing code")
	ErrRateLimited = errors.New("too many attempts, try again in a minute")
)

func NormalizeCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	return strings.ToUpper(code)
}

func NormalizeExpiry(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02T15:04:05.999999999", s, time.UTC)
	}
	if err != nil {
		return ""
	}
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func Exchange(ctx context.Context, c *romm.Client, code string, t Timing) (romm.ClientToken, error) {
	t = t.withDefaults()
	code = NormalizeCode(code)
	if code == "" {
		return romm.ClientToken{}, ErrNoCode
	}
	xctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), t.PollTimeout)
	tok, err := c.ExchangeCode(xctx, code)
	cancel()
	switch {
	case err == nil:
		tok.ExpiresAt = NormalizeExpiry(tok.ExpiresAt)
		return tok, nil
	case romm.IsNotFound(err):
		return romm.ClientToken{}, ErrInvalidCode
	case romm.IsRateLimited(err):
		return romm.ClientToken{}, ErrRateLimited
	}
	return romm.ClientToken{}, err
}
