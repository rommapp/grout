package library

import (
	"context"
	"time"

	"github.com/rommapp/grout/core/firmware"
)

type Fired struct {
	Conditions []firmware.ExitCondition
	Seq        int64
}

func (l *Library) FireConditions(ctx context.Context, conds ...firmware.ExitCondition) error {
	if len(conds) == 0 {
		return nil
	}
	tx, err := l.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, c := range conds {
		res, err := tx.ExecContext(ctx, `INSERT INTO exit_conditions (condition, fired_at) VALUES (?, ?)`, string(c), now)
		if err != nil {
			return err
		}
		seq, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM exit_conditions WHERE condition = ? AND seq < ?`, string(c), seq); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (l *Library) FiredConditions(ctx context.Context) (Fired, error) {
	rows, err := l.Store.DB.QueryContext(ctx, `SELECT condition, seq FROM exit_conditions`)
	if err != nil {
		return Fired{}, err
	}
	defer rows.Close()
	var out Fired
	stored := map[firmware.ExitCondition]bool{}
	for rows.Next() {
		var c string
		var seq int64
		if err := rows.Scan(&c, &seq); err != nil {
			return Fired{}, err
		}
		stored[firmware.ExitCondition(c)] = true
		out.Seq = max(out.Seq, seq)
	}
	if err := rows.Err(); err != nil {
		return Fired{}, err
	}
	for _, c := range firmware.ExitConditions() {
		if stored[c] {
			out.Conditions = append(out.Conditions, c)
		}
	}
	return out, nil
}

func (l *Library) ClearConditions(ctx context.Context, f Fired) error {
	if len(f.Conditions) == 0 {
		return nil
	}
	tx, err := l.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range f.Conditions {
		if _, err := tx.ExecContext(ctx, `DELETE FROM exit_conditions WHERE condition = ? AND seq <= ?`, string(c), f.Seq); err != nil {
			return err
		}
	}
	return tx.Commit()
}
