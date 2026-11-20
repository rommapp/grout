package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rommapp/grout/core/romm"
)

var ErrNoToken = errors.New("RomM approved the pairing without a token")

type Sleep func(ctx context.Context, d time.Duration) error

type Timing struct {
	Sleep        Sleep
	Now          func() time.Time
	SlowDownStep time.Duration
	MaxNetErrors int
	DefaultEvery time.Duration
	DefaultLife  time.Duration
	PollTimeout  time.Duration
}

func DefaultTiming() Timing {
	return Timing{
		Sleep:        sleepTimer,
		Now:          time.Now,
		SlowDownStep: 5 * time.Second,
		MaxNetErrors: 3,
		DefaultEvery: 5 * time.Second,
		DefaultLife:  15 * time.Minute,
		PollTimeout:  30 * time.Second,
	}
}

func (t Timing) withDefaults() Timing {
	d := DefaultTiming()
	if t.Sleep == nil {
		t.Sleep = d.Sleep
	}
	if t.Now == nil {
		t.Now = d.Now
	}
	if t.SlowDownStep <= 0 {
		t.SlowDownStep = d.SlowDownStep
	}
	if t.MaxNetErrors <= 0 {
		t.MaxNetErrors = d.MaxNetErrors
	}
	if t.DefaultEvery <= 0 {
		t.DefaultEvery = d.DefaultEvery
	}
	if t.DefaultLife <= 0 {
		t.DefaultLife = d.DefaultLife
	}
	if t.PollTimeout <= 0 {
		t.PollTimeout = d.PollTimeout
	}
	return t
}

func sleepTimer(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Pending struct {
	DeviceCode      string
	UserCode        string
	VerificationURL string
	Interval        time.Duration
	ExpiresAt       time.Time
}

func Start(ctx context.Context, c *romm.Client, baseURL string, r romm.DeviceInitRequest, t Timing) (Pending, error) {
	t = t.withDefaults()
	init, err := c.DeviceInitiate(ctx, r)
	if err != nil {
		return Pending{}, err
	}
	interval := time.Duration(init.Interval) * time.Second
	if interval <= 0 {
		interval = t.DefaultEvery
	}
	life := time.Duration(init.ExpiresIn) * time.Second
	if life <= 0 {
		life = t.DefaultLife
	}
	return Pending{
		DeviceCode:      init.DeviceCode,
		UserCode:        init.UserCode,
		VerificationURL: strings.TrimSuffix(baseURL, "/") + init.VerificationPathComplete,
		Interval:        interval,
		ExpiresAt:       t.Now().Add(life),
	}, nil
}

type Outcome string

const (
	OutcomeApproved Outcome = "approved"
	OutcomeDenied   Outcome = "denied"
	OutcomeExpired  Outcome = "expired"
	OutcomeFailed   Outcome = "failed"
)

func Outcomes() []Outcome {
	return []Outcome{OutcomeApproved, OutcomeDenied, OutcomeExpired, OutcomeFailed}
}

type Approval struct {
	Token     string
	DeviceID  string
	Scopes    []string
	ExpiresAt string
}

func Wait(ctx context.Context, c *romm.Client, p Pending, t Timing) (Approval, Outcome, error) {
	t = t.withDefaults()
	interval := p.Interval
	netErrors := 0
	for {
		if err := t.Sleep(ctx, interval); err != nil {
			return Approval{}, OutcomeFailed, err
		}
		if t.Now().After(p.ExpiresAt) {
			return Approval{}, OutcomeExpired, nil
		}
		if err := ctx.Err(); err != nil {
			return Approval{}, OutcomeFailed, err
		}
		pollCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), t.PollTimeout)
		tok, err := c.PollDeviceToken(pollCtx, p.DeviceCode)
		cancel()
		if err == nil && tok.AccessToken == "" {
			return Approval{}, OutcomeFailed, ErrNoToken
		}
		if err == nil {
			return Approval{Token: tok.AccessToken, DeviceID: tok.DeviceID, Scopes: tok.Scopes, ExpiresAt: NormalizeExpiry(tok.ExpiresAt)}, OutcomeApproved, nil
		}
		var pe *romm.PairStateError
		switch {
		case errors.As(err, &pe):
			netErrors = 0
			switch pe.State {
			case romm.PairDenied:
				return Approval{}, OutcomeDenied, nil
			case romm.PairExpired:
				return Approval{}, OutcomeExpired, nil
			case romm.PairSlowDown:
				interval += t.SlowDownStep
			case romm.PairPending:
			}
		case romm.IsRateLimited(err):
			netErrors = 0
			interval += t.SlowDownStep
		default:
			netErrors++
			if netErrors >= t.MaxNetErrors {
				return Approval{}, OutcomeFailed, err
			}
		}
	}
}
