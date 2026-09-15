package store

import (
	"context"
	"path/filepath"
	"spm/internal/model"
	"testing"
)

func TestRestartMigrationAndEnvironmentPrecedence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, r, err := Load(ctx, dir, "")
	if err != nil || s == nil {
		t.Fatalf("default database not loaded: %v", err)
	}
	if err = s.SaveNode(ctx, model.Node{ID: "a", Name: "a"}); err != nil {
		t.Fatal(err)
	}
	target := "sqlite:///" + filepath.ToSlash(filepath.Join(dir, "target.db"))
	if err = r.Schedule(target); err != nil {
		t.Fatal(err)
	}
	source := r.Active
	s.Close()
	s, r, err = Load(ctx, dir, source)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Environment || r.Pending != target {
		t.Fatal("environment must leave migration pending")
	}
	s.Close()
	s, r, err = Load(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	nodes, _ := s.Nodes(ctx)
	if r.Active != target || r.Pending != "" || len(nodes) != 1 {
		t.Fatal("restart failed to migrate")
	}
	original, err := Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	nodes, _ = original.Nodes(ctx)
	if len(nodes) != 1 {
		t.Fatal("original data destroyed")
	}
}
func TestFailedMigrationPreservesSource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, r, err := Load(ctx, dir, "")
	if err != nil || s == nil {
		t.Fatalf("default database missing %v", err)
	}
	source := r.Active
	target := "sqlite:///" + filepath.ToSlash(filepath.Join(dir, "occupied.db"))
	occupied, err := Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if err = occupied.SaveNode(ctx, model.Node{ID: "foreign"}); err != nil {
		t.Fatal(err)
	}
	occupied.Close()
	if err = r.Schedule(target); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, r, err = Load(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if r.Active != source || r.LastError == "" || r.Pending != target {
		t.Fatal("failed migration must retain source, pending URL and status")
	}
}
