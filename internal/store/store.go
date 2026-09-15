package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"spm/internal/model"
	"strings"
	"time"
)

type Store struct{ DB *sql.DB }
type Point struct {
	Time   int64    `json:"time"`
	CPU    *float64 `json:"cpu"`
	Memory float64  `json:"memory"`
	Disk   float64  `json:"disk"`
	RX     *float64 `json:"rx"`
	TX     *float64 `json:"tx"`
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid database URL")
	}
	switch u.Scheme {
	case "sqlite":
		if u.Host != "" || !strings.HasPrefix(u.Path, "/") || u.Path == "/" || strings.ContainsRune(u.Path, 0) || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return errors.New("use sqlite:///absolute/path.db without query parameters")
		}
	case "postgres", "postgresql":
		if u.Hostname() == "" || strings.Trim(u.Path, "/") == "" || u.Fragment != "" {
			return errors.New("PostgreSQL URL requires host and database")
		}
	default:
		return errors.New("only sqlite:///path.db or postgresql://user:password@host/database are supported")
	}
	return nil
}
func Open(ctx context.Context, raw string) (*Store, error) {
	if err := ValidateURL(raw); err != nil {
		return nil, err
	}
	u, _ := url.Parse(raw)
	driver := "pgx"
	dsn := raw
	if u.Scheme == "sqlite" {
		driver = "sqlite"
		p := u.Path
		if len(p) > 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		if err := os.MkdirAll(filepath.Dir(filepath.FromSlash(p)), 0700); err != nil {
			return nil, err
		}
		fileURL := &url.URL{Scheme: "file", Path: u.Path, RawQuery: "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"}
		dsn = fileURL.String()
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS nodes (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS samples (node_id TEXT NOT NULL, ts BIGINT NOT NULL, cpu DOUBLE PRECISION, memory DOUBLE PRECISION NOT NULL, disk DOUBLE PRECISION NOT NULL, rx DOUBLE PRECISION, tx DOUBLE PRECISION, PRIMARY KEY(node_id,ts))`,
		`CREATE INDEX IF NOT EXISTS samples_time ON samples(ts)`,
		`CREATE TABLE IF NOT EXISTS alerts (id TEXT PRIMARY KEY, ts BIGINT NOT NULL, data TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS alerts_time ON alerts(ts)`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS deliveries (id TEXT PRIMARY KEY, channel TEXT NOT NULL, payload TEXT NOT NULL, attempts INTEGER NOT NULL, next_at BIGINT NOT NULL, done INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	} {
		if _, err = db.ExecContext(ctx, query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Close() error {
	if s.DB == nil {
		return nil
	}
	return s.DB.Close()
}
func (s *Store) Settings(ctx context.Context) (model.Settings, error) {
	v := model.DefaultSettings()
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='global'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(raw), &v)
	return v, err
}
func (s *Store) SaveSettings(ctx context.Context, v model.Settings) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('global',$1) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(b))
	return err
}

// Secret authentication data and alarm state are persisted but never serialized in the public node DTO.
type storedNode struct {
	Node   model.Node                  `json:"node"`
	Hash   string                      `json:"hash"`
	States map[string]model.AlarmState `json:"states"`
}

func encodeNode(n model.Node) (string, error) {
	b, e := json.Marshal(storedNode{n, n.TokenHash, n.States})
	return string(b), e
}
func (s *Store) Nodes(ctx context.Context) ([]model.Node, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT data FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Node{}
	for rows.Next() {
		var raw string
		var n storedNode
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &n); err != nil {
			return nil, err
		}
		n.Node.TokenHash = n.Hash
		n.Node.States = n.States
		out = append(out, n.Node)
	}
	return out, rows.Err()
}
func (s *Store) SaveNode(ctx context.Context, n model.Node) error {
	return s.Record(ctx, n, nil, nil, nil)
}
func (s *Store) Record(ctx context.Context, n model.Node, sample *model.Sample, events []model.Alert, channels []string) error {
	data, err := encodeNode(n)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO nodes(id,data) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, n.ID, data); err != nil {
		return err
	}
	if sample != nil {
		m := sample.Metrics
		if _, err = tx.ExecContext(ctx, `INSERT INTO samples(node_id,ts,cpu,memory,disk,rx,tx) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(node_id,ts) DO NOTHING`, n.ID, sample.Time, m.CPU, m.Memory, m.Disk, m.RX, m.TX); err != nil {
			return err
		}
	}
	for _, e := range events {
		b, marshalErr := json.Marshal(e)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO alerts(id,ts,data) VALUES($1,$2,$3)`, e.ID, e.Time, string(b)); err != nil {
			return err
		}
		for _, channel := range channels {
			if _, err = tx.ExecContext(ctx, `INSERT INTO deliveries(id,channel,payload,attempts,next_at,done) VALUES($1,$2,$3,0,0,0)`, e.ID+":"+channel, channel, string(b)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (s *Store) History(ctx context.Context, id string, from, to int64, limit int) ([]Point, error) {
	if limit < 1 || limit > 1000 || to <= from {
		return nil, errors.New("invalid history range")
	}
	width := max(int64(1), (to-from+int64(limit)-1)/int64(limit))
	rows, err := s.DB.QueryContext(ctx, `SELECT MIN(ts),AVG(cpu),AVG(memory),MAX(disk),AVG(rx),AVG(tx) FROM samples WHERE node_id=$1 AND ts >= $2 AND ts < $3 GROUP BY CAST((ts-$2)/$4 AS BIGINT) ORDER BY MIN(ts)`, id, from, to, width)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []Point{}
	for rows.Next() {
		var p Point
		if err = rows.Scan(&p.Time, &p.CPU, &p.Memory, &p.Disk, &p.RX, &p.TX); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
func (s *Store) Alerts(ctx context.Context, limit int) ([]model.Alert, error) {
	limit = min(200, max(1, limit))
	rows, err := s.DB.QueryContext(ctx, `SELECT data FROM alerts ORDER BY ts DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Alert{}
	for rows.Next() {
		var raw string
		var a model.Alert
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

var tables = []struct {
	name, columns string
	count         int
}{{"nodes", "id,data", 2}, {"samples", "node_id,ts,cpu,memory,disk,rx,tx", 7}, {"alerts", "id,ts,data", 3}, {"settings", "key,value", 2}, {"deliveries", "id,channel,payload,attempts,next_at,done", 6}}

func Copy(ctx context.Context, source, target *Store, migrationID string) error {
	if migrationID == "" {
		return errors.New("migration ID required")
	}
	tx, err := target.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var marker string
	err = tx.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key='migration'`).Scan(&marker)
	owned := err == nil && marker == migrationID
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for _, table := range tables {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table.name).Scan(&count); err != nil {
			return err
		}
		if count != 0 && !owned {
			return errors.New("destination database must be empty")
		}
	}
	// A prior commit may have succeeded while the configuration switch failed.
	// Re-copy our own destination to include new writes made to the original since then.
	if owned {
		for _, table := range tables {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table.name); err != nil {
				return err
			}
		}
	}
	for _, table := range tables {
		rows, e := source.DB.QueryContext(ctx, "SELECT "+table.columns+" FROM "+table.name)
		if e != nil {
			return e
		}
		args := make([]any, table.count)
		ptrs := make([]any, table.count)
		placeholders := make([]string, table.count)
		for i := range args {
			ptrs[i] = &args[i]
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		query := "INSERT INTO " + table.name + "(" + table.columns + ") VALUES(" + strings.Join(placeholders, ",") + ")"
		for rows.Next() {
			if e = rows.Scan(ptrs...); e != nil {
				rows.Close()
				return e
			}
			for i, v := range args {
				if b, ok := v.([]byte); ok {
					args[i] = string(b)
				}
			}
			if _, e = tx.ExecContext(ctx, query, args...); e != nil {
				rows.Close()
				return e
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO metadata(key,value) VALUES('migration',$1) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, migrationID); err != nil {
		return err
	}
	return tx.Commit()
}
