package model

import (
	"fmt"
	"math"
	"strings"
)

type Disk struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}
type Network struct {
	Name string `json:"name"`
	RX   uint64 `json:"rx"`
	TX   uint64 `json:"tx"`
}
type Snapshot struct {
	Time        int64     `json:"time"`
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Platform    string    `json:"platform"`
	Arch        string    `json:"arch"`
	Cores       int       `json:"cores"`
	Uptime      uint64    `json:"uptime"`
	BootTime    uint64    `json:"bootTime"`
	CPUTotal    float64   `json:"cpuTotal"`
	CPUIdle     float64   `json:"cpuIdle"`
	MemoryTotal uint64    `json:"memoryTotal"`
	MemoryUsed  uint64    `json:"memoryUsed"`
	SwapTotal   uint64    `json:"swapTotal"`
	SwapUsed    uint64    `json:"swapUsed"`
	Load1       *float64  `json:"load1"`
	Disks       []Disk    `json:"disks"`
	Networks    []Network `json:"networks"`
}
type Metrics struct {
	CPU    *float64 `json:"cpu"`
	Memory float64  `json:"memory"`
	Disk   float64  `json:"disk"`
	RX     *float64 `json:"rx"`
	TX     *float64 `json:"tx"`
}
type Sample struct {
	Time     int64    `json:"time"`
	Snapshot Snapshot `json:"snapshot"`
	Metrics  Metrics  `json:"metrics"`
}
type Rules struct {
	CPU             float64 `json:"cpu"`
	Memory          float64 `json:"memory"`
	Disk            float64 `json:"disk"`
	HoldSeconds     int     `json:"holdSeconds"`
	OfflineSeconds  int     `json:"offlineSeconds"`
	IntervalSeconds int     `json:"intervalSeconds"`
}
type AlarmState struct {
	Since       int64   `json:"since"`
	Active      bool    `json:"active"`
	Threshold   float64 `json:"threshold,omitempty"`
	HoldSeconds int     `json:"holdSeconds,omitempty"`
}
type Node struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	TokenHash string                `json:"-"`
	Created   int64                 `json:"created"`
	LastSeen  int64                 `json:"lastSeen"`
	Rules     *Rules                `json:"rules"`
	Latest    *Sample               `json:"latest"`
	States    map[string]AlarmState `json:"-"`
	Online    bool                  `json:"online"`
}
type Alert struct {
	ID       string `json:"id"`
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
	Kind     string `json:"kind"`
	Active   bool   `json:"active"`
	Time     int64  `json:"time"`
	Message  string `json:"message"`
}
type Notifications struct {
	WebhookEnabled  bool   `json:"webhookEnabled"`
	WebhookURL      string `json:"webhookURL"`
	TelegramEnabled bool   `json:"telegramEnabled"`
	TelegramToken   string `json:"telegramToken"`
	TelegramChat    string `json:"telegramChat"`
}
type Settings struct {
	Public        bool          `json:"public"`
	Rules         Rules         `json:"rules"`
	RetentionDays int           `json:"retentionDays"`
	Notifications Notifications `json:"notifications"`
}

func DefaultSettings() Settings {
	return Settings{Rules: Rules{CPU: 90, Memory: 90, Disk: 90, HoldSeconds: 30, OfflineSeconds: 15, IntervalSeconds: 3}, RetentionDays: 7}
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
func (s Snapshot) Validate() error {
	if s.Time <= 0 || strings.TrimSpace(s.Hostname) == "" || len(s.Hostname) > 255 || len(s.Platform) > 255 || len(s.OS) > 32 || len(s.Arch) > 32 || s.Cores < 1 || s.Cores > 65536 {
		return fmt.Errorf("invalid host information")
	}
	if !finite(s.CPUTotal) || !finite(s.CPUIdle) || s.CPUIdle > s.CPUTotal || s.Load1 != nil && !finite(*s.Load1) {
		return fmt.Errorf("invalid CPU counters")
	}
	if s.MemoryTotal == 0 || s.MemoryUsed > s.MemoryTotal || s.SwapUsed > s.SwapTotal {
		return fmt.Errorf("invalid memory counters")
	}
	if len(s.Disks) > 64 || len(s.Networks) > 128 {
		return fmt.Errorf("too many devices")
	}
	seen := map[string]bool{}
	for _, d := range s.Disks {
		if d.Path == "" || len(d.Path) > 512 || d.Used > d.Total || seen[d.Path] {
			return fmt.Errorf("invalid disk")
		}
		seen[d.Path] = true
	}
	seen = map[string]bool{}
	for _, n := range s.Networks {
		if n.Name == "" || len(n.Name) > 255 || seen[n.Name] {
			return fmt.Errorf("invalid network")
		}
		seen[n.Name] = true
	}
	return nil
}
func (r Rules) Validate() error {
	for _, v := range []float64{r.CPU, r.Memory, r.Disk} {
		if !finite(v) || v < 1 || v > 100 {
			return fmt.Errorf("thresholds must be 1–100")
		}
	}
	if r.IntervalSeconds < 1 || r.IntervalSeconds > 3600 || r.HoldSeconds < 0 || r.HoldSeconds > 86400 || r.OfflineSeconds < 5 || r.OfflineSeconds > 86400 {
		return fmt.Errorf("invalid time interval")
	}
	return nil
}
func Effective(global Rules, override *Rules) Rules {
	if override != nil {
		global = *override
	}
	global.OfflineSeconds = max(global.OfflineSeconds, global.IntervalSeconds*3+3)
	return global
}
func Derive(previous *Snapshot, current Snapshot) Metrics {
	m := Metrics{Memory: 100 * float64(current.MemoryUsed) / float64(current.MemoryTotal)}
	for _, d := range current.Disks {
		if d.Total > 0 {
			m.Disk = max(m.Disk, 100*float64(d.Used)/float64(d.Total))
		}
	}
	if previous == nil || current.Time <= previous.Time || current.BootTime != previous.BootTime || current.Uptime < previous.Uptime {
		return m
	}
	total := current.CPUTotal - previous.CPUTotal
	idle := current.CPUIdle - previous.CPUIdle
	if total > 0 && idle >= 0 && idle <= total {
		v := 100 * (total - idle) / total
		m.CPU = &v
	}
	old := make(map[string]Network, len(previous.Networks))
	for _, n := range previous.Networks {
		old[n.Name] = n
	}
	seconds := float64(current.Time-previous.Time) / 1000
	rx, tx := 0.0, 0.0
	valid := false
	for _, n := range current.Networks {
		if p, ok := old[n.Name]; ok && n.RX >= p.RX && n.TX >= p.TX {
			rx += float64(n.RX-p.RX) / seconds
			tx += float64(n.TX-p.TX) / seconds
			valid = true
		}
	}
	if valid {
		m.RX = &rx
		m.TX = &tx
	}
	return m
}
func Evaluate(n *Node, r Rules, now int64) []Alert {
	r = Effective(r, nil)
	if n.States == nil {
		n.States = map[string]AlarmState{}
	}
	base := n.LastSeen
	if base == 0 {
		base = n.Created
	}
	offline := now-base > int64(r.OfflineSeconds)*1000
	n.Online = n.LastSeen > 0 && !offline
	type condition struct {
		kind      string
		high      bool
		hold      int
		threshold float64
		observed  int64
	}
	conditions := []condition{{"offline", offline, 0, 0, now}}
	if !offline && n.Latest != nil {
		m := n.Latest.Metrics
		if m.CPU != nil {
			conditions = append(conditions, condition{"cpu", *m.CPU > r.CPU, r.HoldSeconds, r.CPU, n.LastSeen})
		} else if state := n.States["cpu"]; !state.Active {
			state.Since = 0
			n.States["cpu"] = state
		}
		conditions = append(conditions, condition{"memory", m.Memory > r.Memory, r.HoldSeconds, r.Memory, n.LastSeen}, condition{"disk", m.Disk > r.Disk, 0, r.Disk, n.LastSeen})
	}
	var events []Alert
	for _, c := range conditions {
		state := n.States[c.kind]
		if !state.Active && (state.Threshold != c.threshold || state.HoldSeconds != c.hold) {
			state.Since = 0
		}
		state.Threshold, state.HoldSeconds = c.threshold, c.hold
		if c.high && state.Since == 0 {
			state.Since = now
		}
		active := c.high && (state.Active || c.hold == 0 || c.observed-state.Since >= int64(c.hold)*1000)
		if active != state.Active {
			state.Active = active
			status := "recovered"
			if active {
				status = "triggered"
			}
			events = append(events, Alert{NodeID: n.ID, NodeName: n.Name, Kind: c.kind, Active: active, Time: now, Message: fmt.Sprintf("%s · %s %s", n.Name, c.kind, status)})
		}
		if !c.high {
			state.Since = 0
		}
		n.States[c.kind] = state
	}
	// Missing samples cannot count toward a continuous resource threshold.
	if offline {
		for k, s := range n.States {
			if k != "offline" && !s.Active {
				s.Since = 0
				n.States[k] = s
			}
		}
	}
	return events
}
