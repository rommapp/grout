package library

import (
	"context"
	"reflect"
	"testing"

	"github.com/rommapp/grout/core/firmware"
)

func fired(t *testing.T, l *Library) []firmware.ExitCondition {
	t.Helper()
	got, err := l.FiredConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got.Conditions
}

func TestConditionsFireListClear(t *testing.T) {
	l := New(openTestStore(t), nil, nil, nil)
	ctx := context.Background()
	if got := fired(t, l); len(got) != 0 {
		t.Fatalf("fired = %v", got)
	}
	if err := l.FireConditions(ctx, firmware.CondGamesChanged, firmware.CondGamelistChanged); err != nil {
		t.Fatal(err)
	}
	if got := fired(t, l); !reflect.DeepEqual(got, firmware.ExitConditions()) {
		t.Fatalf("fired = %v", got)
	}
	read, err := l.FiredConditions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	read.Conditions = []firmware.ExitCondition{firmware.CondGamelistChanged}
	if err := l.ClearConditions(ctx, read); err != nil {
		t.Fatal(err)
	}
	if got := fired(t, l); !reflect.DeepEqual(got, []firmware.ExitCondition{firmware.CondGamesChanged}) {
		t.Fatalf("fired = %v", got)
	}
}

func TestConditionsFireTwiceIsOne(t *testing.T) {
	l := New(openTestStore(t), nil, nil, nil)
	ctx := context.Background()
	for range 2 {
		if err := l.FireConditions(ctx, firmware.CondGamelistChanged); err != nil {
			t.Fatal(err)
		}
	}
	if got := fired(t, l); !reflect.DeepEqual(got, []firmware.ExitCondition{firmware.CondGamelistChanged}) {
		t.Fatalf("fired = %v", got)
	}
}

func TestConditionsSurviveReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	l := New(s, nil, nil, nil)
	if err := l.FireConditions(context.Background(), firmware.CondGamesChanged); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if got := fired(t, New(s2, nil, nil, nil)); !reflect.DeepEqual(got, []firmware.ExitCondition{firmware.CondGamesChanged}) {
		t.Fatalf("fired = %v", got)
	}
}

func TestConditionsRefiredAfterReadSurviveClear(t *testing.T) {
	l := New(openTestStore(t), nil, nil, nil)
	ctx := context.Background()
	if err := l.FireConditions(ctx, firmware.CondGamesChanged, firmware.CondGamelistChanged); err != nil {
		t.Fatal(err)
	}
	read, err := l.FiredConditions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.FireConditions(ctx, firmware.CondGamesChanged); err != nil {
		t.Fatal(err)
	}
	if err := l.ClearConditions(ctx, read); err != nil {
		t.Fatal(err)
	}
	if got := fired(t, l); !reflect.DeepEqual(got, []firmware.ExitCondition{firmware.CondGamesChanged}) {
		t.Fatalf("fired = %v", got)
	}
}

func TestConditionsStaleReadAfterClearKeepsNewFire(t *testing.T) {
	l := New(openTestStore(t), nil, nil, nil)
	ctx := context.Background()
	if err := l.FireConditions(ctx, firmware.CondGamesChanged); err != nil {
		t.Fatal(err)
	}
	stale, err := l.FiredConditions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	other, err := l.FiredConditions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ClearConditions(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := l.FireConditions(ctx, firmware.CondGamesChanged); err != nil {
		t.Fatal(err)
	}
	if err := l.ClearConditions(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if got := fired(t, l); !reflect.DeepEqual(got, []firmware.ExitCondition{firmware.CondGamesChanged}) {
		t.Fatalf("fired = %v", got)
	}
}
