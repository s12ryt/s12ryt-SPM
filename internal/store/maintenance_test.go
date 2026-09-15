package store

import (
	"context"
	"spm/internal/model"
	"testing"
)

func TestOutboxRetryRetentionAndDeletion(t *testing.T) {
	ctx := context.Background()
	s := database(t, "outbox.db")
	n := model.Node{ID: "a"}
	sample := model.Sample{Time: 1000}
	event := model.Alert{ID: "e", Time: 1000, NodeID: "a"}
	if err := s.Record(ctx, n, &sample, []model.Alert{event}, []string{"webhook"}); err != nil {
		t.Fatal(err)
	}
	due, err := s.Due(ctx, 1000)
	if err != nil || len(due) != 1 {
		t.Fatalf("missing queued notification: %+v %v", due, err)
	}
	if err = s.Delivered(ctx, due[0], 1000, false); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.Due(ctx, 1001); len(d) != 0 {
		t.Fatal("retry has no backoff")
	}
	due, _ = s.Due(ctx, 100000)
	if len(due) != 1 || due[0].Attempts != 1 {
		t.Fatal("missing retry")
	}
	if err = s.Delivered(ctx, due[0], 100000, true); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.Due(ctx, 200000); len(d) != 0 {
		t.Fatal("successful notification duplicated")
	}
	if err = s.Prune(ctx, 2000); err != nil {
		t.Fatal(err)
	}
	points, _ := s.History(ctx, "a", 0, 3000, 10)
	events, _ := s.Alerts(ctx, 10)
	if len(points) != 0 || len(events) != 0 {
		t.Fatal("retention did not prune")
	}
	if err = s.DeleteNode(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	nodes, _ := s.Nodes(ctx)
	if len(nodes) != 0 {
		t.Fatal("node not deleted")
	}
}
