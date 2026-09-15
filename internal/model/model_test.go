package model

import (
	"math"
	"testing"
)

func snapshot() Snapshot {
	return Snapshot{Time: 1000, Hostname: "vps", OS: "linux", Cores: 2, BootTime: 1, Uptime: 100, CPUTotal: 100, CPUIdle: 50, MemoryTotal: 1000, MemoryUsed: 600, Disks: []Disk{{Path: "/", Total: 100, Used: 80}}, Networks: []Network{{Name: "eth0", RX: 100, TX: 200}}}
}
func TestSnapshotValidation(t *testing.T) {
	if err := snapshot().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Snapshot){
		"memory overflow":     func(s *Snapshot) { s.MemoryUsed = 1001 },
		"cpu NaN":             func(s *Snapshot) { s.CPUTotal = math.NaN() },
		"cpu inconsistent":    func(s *Snapshot) { s.CPUIdle = 101 },
		"empty hostname":      func(s *Snapshot) { s.Hostname = "" },
		"disk overflow":       func(s *Snapshot) { s.Disks[0].Used = 101 },
		"duplicate interface": func(s *Snapshot) { s.Networks = append(s.Networks, s.Networks[0]) },
		"bad time":            func(s *Snapshot) { s.Time = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot()
			mutate(&s)
			if s.Validate() == nil {
				t.Fatal("invalid sample accepted")
			}
		})
	}
}
func TestDeriveCountersAndReset(t *testing.T) {
	p := snapshot()
	c := snapshot()
	c.Time = 4000
	c.CPUTotal = 106
	c.CPUIdle = 51.5
	c.Networks[0].RX = 1000
	c.Networks[0].TX = 800
	m := Derive(&p, c)
	if m.CPU == nil || *m.CPU != 75 || m.RX == nil || *m.RX != 300 || *m.TX != 200 || m.Memory != 60 || m.Disk != 80 {
		t.Fatalf("bad derived metrics: %+v", m)
	}
	for _, kind := range []string{"first", "restart", "reset", "same time"} {
		t.Run(kind, func(t *testing.T) {
			prev := &p
			next := c
			switch kind {
			case "first":
				prev = nil
			case "restart":
				next.BootTime++
			case "reset":
				next.CPUTotal = 1
				next.CPUIdle = 0
				next.Networks = []Network{{Name: "eth0", RX: 1, TX: 1}}
			case "same time":
				next.Time = p.Time
			}
			m := Derive(prev, next)
			if m.CPU != nil || m.RX != nil {
				t.Fatal("invalid delta must not produce rates")
			}
		})
	}
}
func TestRulesValidationAndOverride(t *testing.T) {
	r := DefaultSettings().Rules
	custom := r
	custom.IntervalSeconds = 20
	custom.CPU = 75
	e := Effective(r, &custom)
	if e.CPU != 75 || e.OfflineSeconds != 63 {
		t.Fatalf("override/offline floor: %+v", e)
	}
	for _, mutate := range []func(*Rules){func(r *Rules) { r.CPU = 101 }, func(r *Rules) { r.IntervalSeconds = 0 }, func(r *Rules) { r.HoldSeconds = -1 }, func(r *Rules) { r.Memory = math.NaN() }} {
		bad := r
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("bad rules accepted")
		}
	}
}
func TestAlarmHoldDedupRecoveryOffline(t *testing.T) {
	r := DefaultSettings().Rules
	cpu := 95.0
	n := Node{ID: "node", Name: "node", Created: 1000, LastSeen: 1000, Latest: &Sample{Metrics: Metrics{CPU: &cpu, Memory: 50, Disk: 50}}}
	if e := Evaluate(&n, r, 1000); len(e) != 0 {
		t.Fatal("hold not elapsed")
	}
	n.LastSeen = 31000
	e := Evaluate(&n, r, 31000)
	if len(e) != 1 || !e[0].Active || e[0].Kind != "cpu" {
		t.Fatalf("no CPU trigger: %+v", e)
	}
	if e := Evaluate(&n, r, 32000); len(e) != 0 {
		t.Fatal("duplicate alert")
	}
	cpu = 20
	e = Evaluate(&n, r, 33000)
	if len(e) != 1 || e[0].Active {
		t.Fatal("missing recovery")
	}
	e = Evaluate(&n, r, 47000)
	if len(e) != 1 || e[0].Kind != "offline" || !e[0].Active {
		t.Fatalf("missing offline %+v", e)
	}
	n.LastSeen = 48000
	e = Evaluate(&n, r, 48000)
	if len(e) != 1 || e[0].Active {
		t.Fatal("missing online recovery")
	}
}
