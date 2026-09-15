package store

import "context"

type Delivery struct {
	ID, Channel, Payload string
	Attempts             int
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM samples WHERE node_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM nodes WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Due(ctx context.Context, now int64) ([]Delivery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,channel,payload,attempts FROM deliveries WHERE done=0 AND next_at <= $1 ORDER BY next_at,id LIMIT 20`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err = rows.Scan(&d.ID, &d.Channel, &d.Payload, &d.Attempts); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Delivered(ctx context.Context, d Delivery, now int64, success bool) error {
	attempts := d.Attempts + 1
	done := 0
	if success {
		done = 1
	} else if attempts >= 5 {
		done = 2
	}
	delay := int64(5000) * (1 << min(attempts, 5))
	_, err := s.DB.ExecContext(ctx, `UPDATE deliveries SET attempts=$1,next_at=$2,done=$3 WHERE id=$4`, attempts, now+delay, done, d.ID)
	return err
}
func (s *Store) Prune(ctx context.Context, before int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{`DELETE FROM samples WHERE ts < $1`, `DELETE FROM alerts WHERE ts < $1`, `DELETE FROM deliveries WHERE done<>0 AND next_at < $1`} {
		if _, err = tx.ExecContext(ctx, query, before); err != nil {
			return err
		}
	}
	return tx.Commit()
}
