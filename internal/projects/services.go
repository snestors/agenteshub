package projects

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"gopkg.in/yaml.v3"
)

type ServiceManifest struct {
	Services []ServiceEntry `yaml:"services" json:"services"`
}

type ServiceEntry struct {
	Kind        string `yaml:"kind" json:"kind"` // systemd|docker|cloudflare-tunnel|process
	Description string `yaml:"description" json:"description"`
	// systemd
	Unit string `yaml:"unit,omitempty" json:"unit,omitempty"`
	// docker
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
	HealthCmd string `yaml:"health_cmd,omitempty" json:"health_cmd,omitempty"`
	// cloudflare-tunnel
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Target   string `yaml:"target,omitempty" json:"target,omitempty"`
	// process
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
	Cwd     string `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	// shared
	HealthURL string `yaml:"health_url,omitempty" json:"health_url,omitempty"`
	PublicURL string `yaml:"public_url,omitempty" json:"public_url,omitempty"`
}

type ServiceStatus struct {
	ServiceEntry
	Status      string  `json:"status"` // 'active'|'stopped'|'failed'|'unknown'
	Since       int64   `json:"since,omitempty"`
	CPUPct      float64 `json:"cpu_pct,omitempty"`
	MemMB       float64 `json:"mem_mb,omitempty"`
	HealthOK    bool    `json:"health_ok"`
	HealthError string  `json:"health_error,omitempty"`
}

func LoadManifest(projectPath string) (*ServiceManifest, error) {
	raw, err := os.ReadFile(filepath.Join(projectPath, ".agenthub", "services.yaml"))
	if err != nil {
		return nil, err
	}
	var m ServiceManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Services == nil {
		m.Services = []ServiceEntry{}
	}
	return &m, nil
}

func BuildStatus(ctx context.Context, manifest *ServiceManifest) []ServiceStatus {
	if manifest == nil {
		return []ServiceStatus{}
	}
	out := make([]ServiceStatus, 0, len(manifest.Services))
	for _, entry := range manifest.Services {
		st := ServiceStatus{
			ServiceEntry: entry,
			Status:       "unknown",
		}
		switch strings.TrimSpace(entry.Kind) {
		case "systemd":
			st.Status, st.Since, st.CPUPct, st.MemMB = systemdStatus(ctx, entry.Unit)
		case "docker":
			st.Status, st.CPUPct, st.MemMB, st.HealthError = dockerStatus(ctx, entry.Container)
		case "cloudflare-tunnel":
			st.Status, st.Since, st.CPUPct, st.MemMB = systemdStatus(ctx, "cloudflared")
			if entry.HealthURL == "" && entry.Hostname != "" {
				entry.HealthURL = "https://" + entry.Hostname
				st.ServiceEntry.HealthURL = entry.HealthURL
			}
		case "process":
			st.Status, st.CPUPct, st.MemMB = processStatus(ctx, entry.Command)
		default:
			st.HealthError = "unknown service kind"
		}
		healthOK, healthErr := serviceHealth(ctx, entry)
		if healthErr != "" {
			st.HealthOK = false
			if st.HealthError == "" {
				st.HealthError = healthErr
			}
		} else if hasHealthCheck(entry) {
			st.HealthOK = healthOK
		} else {
			st.HealthOK = st.Status == "active"
		}
		out = append(out, st)
	}
	return out
}

func hasHealthCheck(entry ServiceEntry) bool {
	return strings.TrimSpace(entry.HealthURL) != "" || strings.TrimSpace(entry.HealthCmd) != ""
}

func systemdStatus(ctx context.Context, unit string) (status string, since int64, cpuPct, memMB float64) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return "unknown", 0, 0, 0
	}
	state, _ := cmdOutput(ctx, 2*time.Second, "", "systemctl", "is-active", unit)
	state = strings.TrimSpace(state)
	switch state {
	case "active":
		status = "active"
	case "inactive":
		status = "stopped"
	case "failed":
		status = "failed"
	case "":
		status = "unknown"
	default:
		status = state
	}
	since, cpuPct, memMB = systemdMeta(ctx, unit)
	return status, since, cpuPct, memMB
}

func systemdMeta(ctx context.Context, unit string) (since int64, cpuPct, memMB float64) {
	out, err := cmdOutput(ctx, 2*time.Second, "", "systemctl", "show", unit, "--property=ActiveEnterTimestamp", "--property=MainPID")
	if err != nil {
		return 0, 0, 0
	}
	var pid int32
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "ActiveEnterTimestamp="):
			val := strings.TrimSpace(strings.TrimPrefix(line, "ActiveEnterTimestamp="))
			if t, err := parseSystemdTime(val); err == nil {
				since = t.Unix()
			}
		case strings.HasPrefix(line, "MainPID="):
			val := strings.TrimSpace(strings.TrimPrefix(line, "MainPID="))
			n, _ := strconv.Atoi(val)
			pid = int32(n)
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

func dockerStatus(ctx context.Context, container string) (status string, cpuPct, memMB float64, errMsg string) {
	container = strings.TrimSpace(container)
	if container == "" {
		return "unknown", 0, 0, "missing docker container"
	}
	state, err := cmdOutput(ctx, 3*time.Second, "", "docker", "inspect", "--format", "{{.State.Status}}", container)
	if err != nil {
		return "unknown", 0, 0, err.Error()
	}
	switch strings.TrimSpace(state) {
	case "running":
		status = "active"
	case "exited", "created", "paused", "restarting", "removing", "dead":
		status = "stopped"
	default:
		status = strings.TrimSpace(state)
		if status == "" {
			status = "unknown"
		}
	}
	stats, err := cmdOutput(ctx, 3*time.Second, "", "docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}\t{{.MemUsage}}", container)
	if err == nil {
		parts := strings.Split(strings.TrimSpace(stats), "\t")
		if len(parts) >= 1 {
			cpuPct = parsePercent(parts[0])
		}
		if len(parts) >= 2 {
			memMB = parseDockerMemMB(parts[1])
		}
	}
	return status, cpuPct, memMB, ""
}

func processStatus(ctx context.Context, command string) (status string, cpuPct, memMB float64) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "unknown", 0, 0
	}
	out, err := cmdOutput(ctx, 2*time.Second, "", "pgrep", "-f", command)
	if err != nil || strings.TrimSpace(out) == "" {
		return "stopped", 0, 0
	}
	first := strings.Fields(out)[0]
	pid64, _ := strconv.ParseInt(first, 10, 32)
	if pid64 > 0 {
		if p, err := process.NewProcess(int32(pid64)); err == nil {
			if c, err := p.CPUPercentWithContext(ctx); err == nil {
				cpuPct = round2(c)
			}
			if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
				memMB = round2(bytesToMB(mi.RSS))
			}
		}
	}
	return "active", cpuPct, memMB
}

func serviceHealth(ctx context.Context, entry ServiceEntry) (bool, string) {
	if entry.HealthURL != "" {
		return httpHealth(ctx, entry.HealthURL)
	}
	if entry.HealthCmd != "" {
		cwd := strings.TrimSpace(entry.Cwd)
		_, err := cmdOutput(ctx, 5*time.Second, cwd, "bash", "-lc", entry.HealthCmd)
		if err != nil {
			return false, err.Error()
		}
		return true, ""
	}
	return false, ""
}

func httpHealth(ctx context.Context, url string) (bool, string) {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _, _ = io.Copy(io.Discard, res.Body); _ = res.Body.Close() }()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return true, ""
	}
	return false, fmt.Sprintf("health returned HTTP %d", res.StatusCode)
}

func cmdOutput(ctx context.Context, timeout time.Duration, cwd, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return string(out), fmt.Errorf("%s timed out", name)
	}
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseSystemdTime(s string) (time.Time, error) {
	if s == "" || s == "n/a" {
		return time.Time{}, errors.New("empty systemd timestamp")
	}
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

func parsePercent(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	v, _ := strconv.ParseFloat(s, 64)
	return round2(v)
}

func parseDockerMemMB(s string) float64 {
	left := strings.TrimSpace(strings.Split(s, "/")[0])
	if left == "" {
		return 0
	}
	fields := strings.Fields(left)
	if len(fields) == 2 {
		return round2(convertMemToMB(fields[0], fields[1]))
	}
	for _, suffix := range []string{"GiB", "MiB", "KiB", "GB", "MB", "KB", "B"} {
		if strings.HasSuffix(left, suffix) {
			return round2(convertMemToMB(strings.TrimSuffix(left, suffix), suffix))
		}
	}
	return 0
}

func convertMemToMB(num, unit string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(num), 64)
	switch strings.TrimSpace(unit) {
	case "GiB", "GB":
		return v * 1024
	case "MiB", "MB":
		return v
	case "KiB", "KB":
		return v / 1024
	case "B":
		return v / (1024 * 1024)
	default:
		return v
	}
}

func bytesToMB(b uint64) float64 { return float64(b) / (1 << 20) }

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
