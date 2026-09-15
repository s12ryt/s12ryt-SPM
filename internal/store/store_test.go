package store

import (
	"context"
	"path/filepath"
	"spm/internal/model"
	"testing"
)

func database(t *testing.T, name string) *Store {
	t.Helper()
	s, err := Open(context.Background(), "sqlite:///"+filepath.ToSlash(filepath.Join(t.TempDir(), name)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestURLValidation(t *testing.T) {
	for _, raw := range []string{"host=localhost user=admin", "mysql://db", "postgres://", "postgres://localhost", "sqlite://remote/data.db", ""} {
		if ValidateURL(raw) == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"sqlite:///data/spm.db", "postgresql://user:pass@localhost:5432/spm?sslmode=require"} {
		if err := ValidateURL(raw); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRoundtripAndHistoryBuckets(t *testing.T) {
	ctx := context.Background()
	s := database(t, "source.db")
	settings := model.DefaultSettings()
	settings.Public = true
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Settings(ctx)
	if !got.Public {
		t.Fatal("settings not persisted")
	}
	n := model.Node{ID: "a", Name: "測試", TokenHash: "hash", States: map[string]model.AlarmState{"cpu": {Active: true, Since: 1}}}
	for i := 0; i < 20; i++ {
		v := float64(i)
		sample := model.Sample{Time: int64(i * 1000), Metrics: model.Metrics{CPU: &v, Memory: 50, Disk: 20}}
		n.Latest = &sample
		if err := s.Record(ctx, n, &sample, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	nodes, err := s.Nodes(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].TokenHash != "hash" || !nodes[0].States["cpu"].Active {
		t.Fatalf("bad persisted node %+v %v", nodes, err)
	}
	points, err := s.History(ctx, "a", 0, 20000, 5)
	if err != nil || len(points) != 5 || points[0].CPU == nil || *points[0].CPU != 1.5 {
		t.Fatalf("incorrect bucket aggregation %+v %v", points, err)
	}
}
func TestCopyAtomicIdempotentAndRejectOccupied(t *testing.T) {
	ctx := context.Background()
	source := database(t, "source.db")
	target := database(t, "target.db")
	n := model.Node{ID: "a", Name: "host", TokenHash: "secret-hash"}
	sample := model.Sample{Time: 1, Metrics: model.Metrics{Memory: 20}}
	event := model.Alert{ID: "event", NodeID: "a", Time: 1, Kind: "cpu", Active: true}
	if err := source.Record(ctx, n, &sample, []model.Alert{event}, []string{"webhook"}); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, source, target, "migration-1"); err != nil {
		t.Fatal(err)
	}
	nodes, _ := target.Nodes(ctx)
	alerts, _ := target.Alerts(ctx, 10)
	points, _ := target.History(ctx, "a", 0, 10, 10)
	if len(nodes) != 1 || len(alerts) != 1 || len(points) != 1 {
		t.Fatal("migration lost records")
	}
	if err := Copy(ctx, source, target, "migration-1"); err != nil {
		t.Fatal("retry must be idempotent", err)
	}
	if err := source.SaveNode(ctx, model.Node{ID: "b", Name: "new write after failed switch"}); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, source, target, "migration-1"); err != nil {
		t.Fatal(err)
	}
	if nodes, _ := target.Nodes(ctx); len(nodes) != 2 {
		t.Fatal("retry lost source writes")
	}
	if err := Copy(ctx, source, target, "migration-2"); err == nil {
		t.Fatal("occupied destination accepted")
	}
	if nodes, _ := source.Nodes(ctx); len(nodes) != 2 {
		t.Fatal("source modified")
	}
}
