package auth

import (
	"context"
	"time"
)

const (
	settleAttempts = 6
	settleGap      = 700 * time.Millisecond
)

type Validator func(ctx context.Context) error

func Settle(ctx context.Context, validate Validator, t Timing) error {
	t = t.withDefaults()
	for i := 0; i < settleAttempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if validate(ctx) == nil {
			return nil
		}
		if i == settleAttempts-1 {
			break
		}
		if err := t.Sleep(ctx, settleGap); err != nil {
			return err
		}
	}
	return nil
}
