// Package sysman exposes a high-level interface to the host machine:
// CPU/RAM/disk stats, systemd services (whitelisted), top processes, and
// connection state (WhatsApp, WS clients, tunnels).
//
// systemd interactions go through `systemctl`; that's good enough for v0.
// dbus is overkill for a single-user box.
package sysman

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/snestors/agenteshub/internal/config"
)

// Stats is a snapshot of host resource usage.
type Stats struct {
	CPUPct     float64    `json:"cpu_pct"`
	LoadAvg    [3]float64 `json:"load_avg"`
	Cores      int        `json:"cores"`
	RAMUsedGB  float64    `json:"ram_used_gb"`
	RAMTotalGB float64    `json:"ram_total_gb"`
	SwapUsedGB float64    `json:"swap_used_gb"`
	Disks      []Disk     `json:"disks"`
	TempC      float64    `json:"temp_c"`
	UptimeSec  int64      `json:"uptime_s"`
}

// Disk is a single mounted volume.
type Disk struct {
	Device  string  `json:"device"` // 'nvme0n1'
	Mount   string  `json:"mount"`  // '/'
	UsedGB  float64 `json:"used_gb"`
	TotalGB float64 `json:"total_gb"`
	UsedPct float64 `json:"used_pct"`
}

// Service is the state of a systemd unit.
type Service struct {
	Name   string  `json:"name"`
	State  string  `json:"state"` // 'active'|'inactive'|'failed'|'activating'|'deactivating'
	Since  int64   `json:"since"` // unix epoch
	CPUPct float64 `json:"cpu_pct"`
	MemMB  float64 `json:"mem_mb"`
}

// Process is one entry in the top-N table.
type Process struct {
	PID    int     `json:"pid"`
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_pct"`
	MemMB  float64 `json:"mem_mb"`
	Cmd    string  `json:"cmd"`
}

// Connections is a coarse view of what AgentHub is talking to.
type Connections struct {
	WhatsApp  string   `json:"wa"`         // 'connected'|'disconnected'|'pairing'
	WSClients int      `json:"ws_clients"` // populated by ws hub once wired
	Tunnels   []Tunnel `json:"tunnels"`
}

// Tunnel is a tunneling daemon (cloudflared, etc.).
type Tunnel struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// Manager is the entry point.
type Manager struct {
	cfg *config.Config
	log *slog.Logger
}

// New constructs a Manager.
func New(cfg *config.Config, log *slog.Logger) *Manager {
	return &Manager{cfg: cfg, log: log}
}

// Stats returns a fresh snapshot of host resources.
func (m *Manager) Stats(ctx context.Context) (Stats, error) {
	out := Stats{}

	// CPU — non-blocking read (interval=0 → instant since last call).
	if pcts, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pcts) > 0 {
		out.CPUPct = round2(pcts[0])
	}
	if c, err := cpu.CountsWithContext(ctx, true); err == nil {
		out.Cores = c
	}

	// Load average.
	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		out.LoadAvg = [3]float64{round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)}
	}

	// RAM.
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
		out.RAMTotalGB = bytesToGB(vm.Total)
		out.RAMUsedGB = bytesToGB(vm.Used)
	}
	if sm, err := mem.SwapMemoryWithContext(ctx); err == nil && sm != nil {
		out.SwapUsedGB = bytesToGB(sm.Used)
	}

	// Disks — only physical mounts (skip tmpfs/squashfs/etc.).
	if parts, err := disk.PartitionsWithContext(ctx, false); err == nil {
		for _, p := range parts {
			if !isPhysicalFs(p.Fstype) {
				continue
			}
			u, err := disk.UsageWithContext(ctx, p.Mountpoint)
			if err != nil || u == nil {
				continue
			}
			out.Disks = append(out.Disks, Disk{
				Device:  trimDevPrefix(p.Device),
				Mount:   p.Mountpoint,
				UsedGB:  bytesToGB(u.Used),
				TotalGB: bytesToGB(u.Total),
				UsedPct: round2(u.UsedPercent),
			})
		}
	}

	// CPU temperature — pick the first sane sensor (>0, <120°C).
	if temps, err := host.SensorsTemperaturesWithContext(ctx); err == nil {
		for _, t := range temps {
			if t.Temperature > 0 && t.Temperature < 120 {
				if isCPUSensor(t.SensorKey) {
					out.TempC = round2(t.Temperature)
					break
				}
			}
		}
		// Fallback: first non-zero if no labeled CPU sensor.
		if out.TempC == 0 {
			for _, t := range temps {
				if t.Temperature > 0 && t.Temperature < 120 {
					out.TempC = round2(t.Temperature)
					break
				}
			}
		}
	}

	// Uptime.
	if up, err := host.UptimeWithContext(ctx); err == nil {
		out.UptimeSec = int64(up)
	}

	return out, nil
}

// Services returns the state of each whitelisted systemd unit.
func (m *Manager) Services(ctx context.Context) ([]Service, error) {
	out := []Service{}
	for _, name := range m.cfg.ManagedServices {
		s := Service{Name: name}
		s.State = systemctlIsActive(ctx, name)
		s.Since, s.CPUPct, s.MemMB = systemctlMeta(ctx, name)
		out = append(out, s)
	}
	return out, nil
}

// ServiceAction performs start/stop/restart on a whitelisted unit.
//
// Tries plain `systemctl` first; if that fails with a permission error,
// retries with `sudo -n systemctl` (NOPASSWD must be configured for this
// to succeed in production). Errors are surfaced verbatim so the caller
// can decide on status code.
func (m *Manager) ServiceAction(ctx context.Context, name, action string) error {
	if !m.isManaged(name) {
		return fmt.Errorf("service %q not in whitelist", name)
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("invalid action %q (start|stop|restart)", action)
	}
	// Direct attempt — works if the daemon has CAP_SYS_ADMIN or polkit grants the user.
	if err := runCmd(ctx, "systemctl", action, name); err == nil {
		return nil
	} else if !isAuthError(err) {
		return err
	}
	// Fallback: sudo -n (non-interactive). Requires NOPASSWD in /etc/sudoers.d/.
	if err := runCmd(ctx, "sudo", "-n", "systemctl", action, name); err != nil {
		return err
	}
	return nil
}

func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Interactive authentication") ||
		strings.Contains(msg, "Authentication") ||
		strings.Contains(msg, "denied") ||
		strings.Contains(msg, "permission") ||
		strings.Contains(msg, "Permission") ||
		strings.Contains(msg, "polkit") ||
		strings.Contains(msg, "Polkit")
}

// Processes returns the top-N processes by cpu (default) or mem.
func (m *Manager) Processes(ctx context.Context, top int, sortBy string) ([]Process, error) {
	if top <= 0 || top > 200 {
		top = 10
	}
	if sortBy != "mem" {
		sortBy = "cpu"
	}
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("processes: %w", err)
	}
	out := make([]Process, 0, len(procs))
	for _, p := range procs {
		// Skip kernel threads silently (cmdline empty).
		cmd, _ := p.CmdlineWithContext(ctx)
		if cmd == "" {
			// some processes have name only — keep them anyway
		}
		name, _ := p.NameWithContext(ctx)
		cpuPct, _ := p.CPUPercentWithContext(ctx)
		memInfo, _ := p.MemoryInfoWithContext(ctx)
		var memMB float64
		if memInfo != nil {
			memMB = bytesToMB(memInfo.RSS)
		}
		out = append(out, Process{
			PID:    int(p.Pid),
			Name:   name,
			CPUPct: round2(cpuPct),
			MemMB:  round2(memMB),
			Cmd:    cmd,
		})
	}
	switch sortBy {
	case "mem":
		sort.Slice(out, func(i, j int) bool { return out[i].MemMB > out[j].MemMB })
	default:
		sort.Slice(out, func(i, j int) bool { return out[i].CPUPct > out[j].CPUPct })
	}
	if len(out) > top {
		out = out[:top]
	}
	return out, nil
}

// Connections reports WA + WS + tunnels in a single payload.
func (m *Manager) Connections(ctx context.Context) (Connections, error) {
	out := Connections{
		WSClients: 0, // wired by ws hub later
	}
	if m.cfg.WAEnabled {
		out.WhatsApp = "unknown" // wired by wa client later
	} else {
		out.WhatsApp = "disconnected"
	}
	// Best-effort tunnel discovery: cloudflared from the whitelist.
	for _, name := range m.cfg.ManagedServices {
		if !strings.Contains(name, "cloudflared") && !strings.Contains(name, "tunnel") {
			continue
		}
		out.Tunnels = append(out.Tunnels, Tunnel{
			Name:  name,
			State: systemctlIsActive(ctx, name),
		})
	}
	return out, nil
}

// isManaged checks whitelist membership.
func (m *Manager) isManaged(name string) bool {
	for _, s := range m.cfg.ManagedServices {
		if s == name {
			return true
		}
	}
	return false
}

// ----- helpers -----

func systemctlIsActive(ctx context.Context, name string) string {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", name)
	out, _ := cmd.Output() // is-active exits non-zero when inactive; the stdout still holds the state
	state := strings.TrimSpace(string(out))
	if state == "" {
		return "unknown"
	}
	return state
}

// systemctlMeta extracts ActiveEnterTimestamp + MainPID and computes per-pid cpu/mem.
func systemctlMeta(ctx context.Context, name string) (since int64, cpuPct, memMB float64) {
	cmd := exec.CommandContext(ctx, "systemctl", "show", name,
		"--property=ActiveEnterTimestamp",
		"--property=MainPID")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	var pid int32
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "ActiveEnterTimestamp="):
			val := strings.TrimPrefix(line, "ActiveEnterTimestamp=")
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			// Example: "Sun 2026-04-26 14:32:09 UTC"
			if t, err := parseSystemdTime(val); err == nil {
				since = t.Unix()
			}
		case strings.HasPrefix(line, "MainPID="):
			val := strings.TrimPrefix(line, "MainPID=")
			val = strings.TrimSpace(val)
			fmt.Sscanf(val, "%d", &pid)
		}
	}
	if pid > 0 {
		if p, err := process.NewProcess(pid); err == nil {
			if c, err := p.CPUPercentWithContext(ctx); err == nil {
				cpuPct = round2(c)
			}
			if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
				memMB = round2(bytesToMB(mi.RSS))
			}
		}
	}
	return since, cpuPct, memMB
}

// parseSystemdTime parses formats systemd may emit. We accept either with or
// without weekday + tz token.
func parseSystemdTime(s string) (time.Time, error) {
	formats := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized systemd timestamp")
}

func isPhysicalFs(fs string) bool {
	switch fs {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "f2fs", "zfs", "ntfs", "exfat", "vfat", "fat32", "apfs":
		return true
	}
	return false
}

func trimDevPrefix(dev string) string {
	return strings.TrimPrefix(dev, "/dev/")
}

func isCPUSensor(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "core") || strings.Contains(k, "cpu") || strings.Contains(k, "package") || strings.Contains(k, "tctl") || strings.Contains(k, "tdie") || strings.Contains(k, "k10temp")
}

func bytesToGB(b uint64) float64 { return round2(float64(b) / (1 << 30)) }
func bytesToMB(b uint64) float64 { return round2(float64(b) / (1 << 20)) }

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
