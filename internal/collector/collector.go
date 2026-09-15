package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	netstat "github.com/shirou/gopsutil/v4/net"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"spm/internal/model"
	"strings"
	"time"
)

func Collect(ctx context.Context) (model.Snapshot, error) {
	s := model.Snapshot{Time: time.Now().UnixMilli(), Arch: runtime.GOARCH, Cores: runtime.NumCPU(), Disks: []model.Disk{}, Networks: []model.Network{}}
	h, err := host.InfoWithContext(ctx)
	if err != nil {
		return s, fmt.Errorf("host collection failed")
	}
	s.Hostname = h.Hostname
	s.OS = h.OS
	s.Platform = h.Platform
	s.Uptime = h.Uptime
	s.BootTime = h.BootTime
	times, err := cpu.TimesWithContext(ctx, false)
	if err != nil || len(times) == 0 {
		return s, fmt.Errorf("CPU collection failed")
	}
	c := times[0]
	s.CPUTotal = c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq + c.Softirq + c.Steal
	s.CPUIdle = c.Idle + c.Iowait
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return s, fmt.Errorf("memory collection failed")
	}
	s.MemoryTotal = v.Total
	s.MemoryUsed = v.Used
	if swap, e := mem.SwapMemoryWithContext(ctx); e == nil {
		s.SwapTotal = swap.Total
		s.SwapUsed = swap.Used
	}
	if runtime.GOOS != "windows" {
		if avg, e := load.AvgWithContext(ctx); e == nil {
			s.Load1 = &avg.Load1
		}
	}
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return s, fmt.Errorf("disk collection failed")
	}
	seen := map[string]bool{}
	for _, p := range parts {
		if seen[p.Mountpoint] || len(s.Disks) >= 64 {
			continue
		}
		seen[p.Mountpoint] = true
		if usage, e := disk.UsageWithContext(ctx, p.Mountpoint); e == nil && usage.Total > 0 {
			s.Disks = append(s.Disks, model.Disk{Path: p.Mountpoint, Total: usage.Total, Used: usage.Used})
		}
	}
	counters, err := netstat.IOCountersWithContext(ctx, true)
	if err != nil {
		return s, fmt.Errorf("network collection failed")
	}
	for _, n := range counters {
		if n.Name == "lo" || strings.Contains(strings.ToLower(n.Name), "loopback") || len(s.Networks) >= 128 {
			continue
		}
		s.Networks = append(s.Networks, model.Network{Name: n.Name, RX: n.BytesRecv, TX: n.BytesSent})
	}
	return s, s.Validate()
}
func Upload(ctx context.Context, client *http.Client, endpoint, id, token string, s model.Snapshot) (time.Duration, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return 0, fmt.Errorf("invalid server URL")
	}
	if id == "" || strings.ContainsAny(id, "/\\?#") || token == "" {
		return 0, fmt.Errorf("invalid agent credentials")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/ingest/" + id
	payload, err := json.Marshal(s)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("cannot create upload")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := safe.Do(req)
	if err != nil {
		return 0, fmt.Errorf("upload connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return 0, fmt.Errorf("upload HTTP %d", res.StatusCode)
	}
	var result struct {
		Interval int `json:"intervalSeconds"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&result) != nil || result.Interval < 1 || result.Interval > 3600 {
		return 0, fmt.Errorf("invalid upload response")
	}
	return time.Duration(result.Interval) * time.Second, nil
}
