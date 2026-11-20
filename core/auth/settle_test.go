package auth

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSettleStopsOnFirstSuccess(t *testing.T) {
	f := &fakeTiming{now: epoch}
	calls := 0
	v := func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	}
	if err := Settle(context.Background(), v, f.timing()); err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{700 * time.Millisecond, 700 * time.Millisecond}
	if calls != 3 || !reflect.DeepEqual(f.sleeps, want) {
		t.Fatalf("calls=%d sleeps=%v", calls, f.sleeps)
	}
}

func TestSettleNeverFails(t *testing.T) {
	f := &fakeTiming{now: epoch}
	calls := 0
	v := func(ctx context.Context) error {
		calls++
		return errors.New("no")
	}
	if err := Settle(context.Background(), v, f.timing()); err != nil || calls != 6 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if len(f.sleeps) != 5 {
		t.Fatalf("sleeps=%v", f.sleeps)
	}
}

func TestSettleReturnsContextError(t *testing.T) {
	f := &fakeTiming{now: epoch, err: context.Canceled}
	v := func(ctx context.Context) error { return errors.New("no") }
	if err := Settle(context.Background(), v, f.timing()); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
