package store

import (
	"context"
	"os"
	"path/filepath"
	"spm/internal/model"
	"testing"
)

func TestDatabaseURLRejectsRelativeSQLitePaths(t *testing.T) {
	for _, raw := range []string{"sqlite:relative.db", "sqlite:///../relative.db?", "sqlite:relative/path.db", "sqlite:///%00.db"} {
		if ValidateURL(raw) == nil {
			t.Errorf("unsafe/non-absolute URL accepted: %q", raw)
		}
	}
}

func TestEnvironmentDatabaseOverridesCorruptWebConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "database.json"), []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	url := "sqlite:///" + filepath.ToSlash(filepath.Join(dir, "environment.db"))
	db, runtime, err := Load(context.Background(), dir, url)
	if err != nil {
		t.Fatalf("valid environment URL blocked by corrupt web config: %v", err)
	}
	defer db.Close()
	if !runtime.Environment || runtime.Active != url {
		t.Fatal("environment URL did not take precedence")
	}
	if _, _, err := Load(context.Background(), dir, ""); err == nil {
		t.Fatal("corrupt active config silently ignored without environment URL")
	}
}

func TestMigrationRollbackOnMidCopyFailure(t *testing.T) {
	ctx := context.Background()
	source, target := database(t, "source.db"), database(t, "target.db")
	n := model.Node{ID: "a", Name: "original", TokenHash: "private-hash"}
	sample := model.Sample{Time: 1000, Metrics: model.Metrics{Memory: 50}}
	if err := source.Record(ctx, n, &sample, []model.Alert{{ID: "event", NodeID: "a", Kind: "cpu"}}, []string{"webhook"}); err != nil {
		t.Fatal(err)
	}
	// The failure happens after node/sample insertion, exercising rollback rather than preflight rejection.
	if _, err := target.DB.ExecContext(ctx, `CREATE TRIGGER reject_alert BEFORE INSERT ON alerts BEGIN SELECT RAISE(ABORT, 'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, source, target, "failing"); err == nil {
		t.Fatal("injected destination failure was ignored")
	}
	for _, table := range []string{"nodes", "samples", "alerts", "deliveries", "metadata"} {
		var count int
		if err := target.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial migration survived rollback in %s", table)
		}
	}
	nodes, err := source.Nodes(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].TokenHash != "private-hash" {
		t.Fatal("source changed after failed migration", err)
	}
}
