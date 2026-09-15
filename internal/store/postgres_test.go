package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"spm/internal/model"
	"strings"
	"testing"
	"time"
)

// Opt-in integration test: its randomly named schema is removed on completion.
// TEST_POSTGRES_URL is used by tests only; the application accepts DATABASE_URL.
func TestPostgresIntegration(t *testing.T) {
	raw := os.Getenv("TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("TEST_POSTGRES_URL is not set; PostgreSQL integration not executed")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_POSTGRES_URL must be a PostgreSQL URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal("cannot initialize PostgreSQL integration connection")
	}
	defer admin.Close()
	schema := "spm_test_" + strings.ToLower(rand.Text())
	if _, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal("cannot create isolated PostgreSQL test schema", err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("test schema cleanup failed", schema, err)
		}
	}()
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pg, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	source := database(t, "source.db")
	settings := model.DefaultSettings()
	settings.Public = true
	if err := source.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	n := model.Node{ID: "pg-node", Name: "跨庫測試", TokenHash: "test-hash", States: map[string]model.AlarmState{"cpu": {Active: true}}}
	for i := 0; i < 20; i++ {
		cpu := float64(i)
		sample := model.Sample{Time: int64(i * 1000), Metrics: model.Metrics{CPU: &cpu, Memory: 50, Disk: 30}}
		if err := source.Record(ctx, n, &sample, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	event := model.Alert{ID: "pg-event", NodeID: n.ID, Kind: "cpu", Active: true, Time: 20000}
	if err := source.Record(ctx, n, nil, []model.Alert{event}, []string{"webhook"}); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, source, pg, "sqlite-to-pg"); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, source, pg, "sqlite-to-pg"); err != nil {
		t.Fatal("migration retry", err)
	}
	verify := func(s *Store) {
		t.Helper()
		nodes, err := s.Nodes(ctx)
		if err != nil || len(nodes) != 1 || nodes[0].TokenHash != n.TokenHash || !nodes[0].States["cpu"].Active {
			t.Fatalf("node/state roundtrip: %v %v", nodes, err)
		}
		got, err := s.Settings(ctx)
		if err != nil || !got.Public {
			t.Fatal("settings roundtrip", err)
		}
		points, err := s.History(ctx, n.ID, 0, 20000, 5)
		if err != nil || len(points) != 5 || points[0].CPU == nil || *points[0].CPU != 1.5 {
			t.Fatalf("history buckets: %v %v", points, err)
		}
		alerts, err := s.Alerts(ctx, 10)
		if err != nil || len(alerts) != 1 || alerts[0].ID != event.ID {
			t.Fatal("alert roundtrip", err)
		}
		queue, err := s.Due(ctx, time.Now().UnixMilli())
		if err != nil || len(queue) != 1 || queue[0].Channel != "webhook" {
			t.Fatal("outbox roundtrip", err)
		}
	}
	verify(pg)
	back := database(t, "return.db")
	if err := Copy(ctx, pg, back, "pg-to-sqlite"); err != nil {
		t.Fatal(err)
	}
	verify(back)
	if err := pg.DeleteNode(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if nodes, err := pg.Nodes(ctx); err != nil || len(nodes) != 0 {
		t.Fatal("PostgreSQL delete", err)
	}
}
