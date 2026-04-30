package sysman

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CronJob is one cron entry discovered on the host.
type CronJob struct {
	Kind     string `json:"kind"` // 'user' | 'system' | 'periodic'
	Source   string `json:"source"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Schedule string `json:"schedule"`
	User     string `json:"user,omitempty"`
	Command  string `json:"command"`
}

// CronListing is the read-only snapshot returned to the UI/tools.
type CronListing struct {
	Jobs      []CronJob `json:"jobs"`
	Warnings  []string  `json:"warnings,omitempty"`
	ScannedAt int64     `json:"scanned_at"`
}

var (
	cronEnvLineRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	runPartsNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// CronJobs lists cron entries from the current user's crontab, /etc/crontab,
// /etc/cron.d/* and run-parts directories. Read-only by design.
func (m *Manager) CronJobs(ctx context.Context) (CronListing, error) {
	out := CronListing{ScannedAt: time.Now().Unix()}

	if jobs, warns := readUserCrontab(ctx); len(jobs) > 0 || len(warns) > 0 {
		out.Jobs = append(out.Jobs, jobs...)
		out.Warnings = append(out.Warnings, warns...)
	}

	if jobs, warns := readCronFile("/etc/crontab", true); len(jobs) > 0 || len(warns) > 0 {
		out.Jobs = append(out.Jobs, jobs...)
		out.Warnings = append(out.Warnings, warns...)
	}

	if jobs, warns := readCronDir("/etc/cron.d"); len(jobs) > 0 || len(warns) > 0 {
		out.Jobs = append(out.Jobs, jobs...)
		out.Warnings = append(out.Warnings, warns...)
	}

	periodicDirs := []struct {
		path     string
		schedule string
	}{
		{path: "/etc/cron.hourly", schedule: "@hourly"},
		{path: "/etc/cron.daily", schedule: "@daily"},
		{path: "/etc/cron.weekly", schedule: "@weekly"},
		{path: "/etc/cron.monthly", schedule: "@monthly"},
	}
	for _, dir := range periodicDirs {
		jobs, warns := readPeriodicDir(dir.path, dir.schedule)
		out.Jobs = append(out.Jobs, jobs...)
		out.Warnings = append(out.Warnings, warns...)
	}

	sort.Slice(out.Jobs, func(i, j int) bool {
		if out.Jobs[i].Source != out.Jobs[j].Source {
			return out.Jobs[i].Source < out.Jobs[j].Source
		}
		if out.Jobs[i].Line != out.Jobs[j].Line {
			return out.Jobs[i].Line < out.Jobs[j].Line
		}
		if out.Jobs[i].Schedule != out.Jobs[j].Schedule {
			return out.Jobs[i].Schedule < out.Jobs[j].Schedule
		}
		return out.Jobs[i].Command < out.Jobs[j].Command
	})

	return out, nil
}

func readUserCrontab(ctx context.Context) ([]CronJob, []string) {
	cmd := exec.CommandContext(ctx, "crontab", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = strings.TrimSpace(err.Error())
		}
		if strings.Contains(strings.ToLower(msg), "no crontab for") {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("user crontab: %s", msg)}
	}
	return parseCronContent(string(out), CronParseOpts{
		Kind:        "user",
		Source:      "user crontab",
		File:        "crontab -l",
		HasUser:     false,
		DefaultUser: "",
	}), nil
}

func readCronDir(dir string) ([]CronJob, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("%s: %v", dir, err)}
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var jobs []CronJob
	var warns []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		fileJobs, fileWarns := readCronFile(path, true)
		jobs = append(jobs, fileJobs...)
		warns = append(warns, fileWarns...)
	}
	return jobs, warns
}

func readCronFile(path string, hasUser bool) ([]CronJob, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	return parseCronContent(string(raw), CronParseOpts{
		Kind:        "system",
		Source:      path,
		File:        path,
		HasUser:     hasUser,
		DefaultUser: "root",
	}), nil
}

func readPeriodicDir(dir, schedule string) ([]CronJob, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("%s: %v", dir, err)}
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isRunPartsCandidate(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	jobs := make([]CronJob, 0, len(names))
	for _, name := range names {
		jobs = append(jobs, CronJob{
			Kind:     "periodic",
			Source:   dir,
			File:     filepath.Join(dir, name),
			Schedule: schedule,
			User:     "root",
			Command:  filepath.Join(dir, name),
		})
	}
	return jobs, nil
}

type CronParseOpts struct {
	Kind        string
	Source      string
	File        string
	HasUser     bool
	DefaultUser string
}

func parseCronContent(raw string, opts CronParseOpts) []CronJob {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	out := []CronJob{}
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawLine := strings.TrimSpace(scanner.Text())
		if rawLine == "" || strings.HasPrefix(rawLine, "#") || cronEnvLineRe.MatchString(rawLine) {
			continue
		}

		fields := strings.Fields(rawLine)
		if len(fields) == 0 {
			continue
		}

		job := CronJob{
			Kind:   opts.Kind,
			Source: opts.Source,
			File:   opts.File,
			Line:   lineNo,
			User:   opts.DefaultUser,
		}

		if strings.HasPrefix(fields[0], "@") {
			job.Schedule = fields[0]
			if opts.HasUser {
				if len(fields) < 3 {
					continue
				}
				job.User = fields[1]
				job.Command = strings.Join(fields[2:], " ")
			} else {
				if len(fields) < 2 {
					continue
				}
				job.Command = strings.Join(fields[1:], " ")
			}
			out = append(out, job)
			continue
		}

		if opts.HasUser {
			if len(fields) < 7 {
				continue
			}
			job.Schedule = strings.Join(fields[:5], " ")
			job.User = fields[5]
			job.Command = strings.Join(fields[6:], " ")
		} else {
			if len(fields) < 6 {
				continue
			}
			job.Schedule = strings.Join(fields[:5], " ")
			job.Command = strings.Join(fields[5:], " ")
		}
		out = append(out, job)
	}
	return out
}

func isRunPartsCandidate(name string) bool {
	return runPartsNameRe.MatchString(name)
}
