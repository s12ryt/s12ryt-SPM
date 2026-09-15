package model

import "testing"

func TestHoldRequiresFreshSamples(t *testing.T) {
	r := DefaultSettings().Rules
	r.IntervalSeconds = 60
	cpu := 95.0
	n := Node{ID: "n", LastSeen: 1000, Latest: &Sample{Metrics: Metrics{CPU: &cpu, Memory: 95}}}
	Evaluate(&n, r, 1000)
	if events := Evaluate(&n, r, 31000); len(events) != 0 {
		t.Fatalf("a single stale sample triggered a continuous threshold: %+v", events)
	}
	n.LastSeen = 61000
	if events := Evaluate(&n, r, 61000); len(events) != 2 {
		t.Fatalf("fresh sustained CPU/memory samples should trigger: %+v", events)
	}
}

func TestUnknownCPUResetsPendingHold(t *testing.T) {
	r := DefaultSettings().Rules
	cpu := 95.0
	n := Node{ID: "n", LastSeen: 1000, Latest: &Sample{Metrics: Metrics{CPU: &cpu}}}
	Evaluate(&n, r, 1000)
	n.LastSeen = 28000
	n.Latest.Metrics.CPU = nil
	Evaluate(&n, r, 28000)
	n.LastSeen = 31000
	n.Latest.Metrics.CPU = &cpu
	if events := Evaluate(&n, r, 31000); len(events) != 0 {
		t.Fatalf("unknown CPU sample counted toward hold: %+v", events)
	}
	n.LastSeen = 61000
	if events := Evaluate(&n, r, 61000); len(events) != 1 || !events[0].Active {
		t.Fatalf("new valid hold did not trigger: %+v", events)
	}
}

func TestChangedThresholdStartsNewPendingHold(t *testing.T) {
	r := DefaultSettings().Rules
	cpu := 95.0
	n := Node{ID: "n", LastSeen: 1000, Latest: &Sample{Metrics: Metrics{CPU: &cpu}}}
	Evaluate(&n, r, 1000)
	r.CPU = 92
	n.LastSeen = 31000
	if events := Evaluate(&n, r, 31000); len(events) != 0 {
		t.Fatalf("old rule's pending hold triggered under new threshold: %+v", events)
	}
	n.LastSeen = 61000
	if events := Evaluate(&n, r, 61000); len(events) != 1 {
		t.Fatalf("new threshold hold did not trigger: %+v", events)
	}
}
