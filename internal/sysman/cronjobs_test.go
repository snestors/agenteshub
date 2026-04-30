package sysman

import "testing"

func TestParseCronContentUserCrontab(t *testing.T) {
	raw := `
SHELL=/bin/bash
PATH=/usr/local/bin:/usr/bin
# comentario
*/5 * * * * cd /home/nestor/grid-bot && ./run_grid.py >> logs/cron_grid.log 2>&1
0 13 * * * /home/nestor/finanzas/run_monitor.sh
@reboot /usr/local/bin/boot-task
`

	got := parseCronContent(raw, CronParseOpts{
		Kind:   "user",
		Source: "user crontab",
		File:   "crontab -l",
	})

	if len(got) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(got))
	}
	if got[0].Schedule != "*/5 * * * *" {
		t.Fatalf("unexpected first schedule: %q", got[0].Schedule)
	}
	if got[0].Command != "cd /home/nestor/grid-bot && ./run_grid.py >> logs/cron_grid.log 2>&1" {
		t.Fatalf("unexpected first command: %q", got[0].Command)
	}
	if got[1].Schedule != "0 13 * * *" {
		t.Fatalf("unexpected second schedule: %q", got[1].Schedule)
	}
	if got[2].Schedule != "@reboot" {
		t.Fatalf("unexpected macro schedule: %q", got[2].Schedule)
	}
}

func TestParseCronContentSystemCrontab(t *testing.T) {
	raw := `
# /etc/crontab
17 * * * * root cd / && run-parts --report /etc/cron.hourly
@daily root /usr/local/bin/nightly
MAILTO=root
`

	got := parseCronContent(raw, CronParseOpts{
		Kind:        "system",
		Source:      "/etc/crontab",
		File:        "/etc/crontab",
		HasUser:     true,
		DefaultUser: "root",
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got))
	}
	if got[0].User != "root" || got[0].Schedule != "17 * * * *" {
		t.Fatalf("unexpected first job: %+v", got[0])
	}
	if got[1].Schedule != "@daily" || got[1].Command != "/usr/local/bin/nightly" {
		t.Fatalf("unexpected macro job: %+v", got[1])
	}
}

func TestIsRunPartsCandidate(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{name: "google-chrome", ok: true},
		{name: "apt_compat", ok: true},
		{name: ".placeholder", ok: false},
		{name: "name.with.dot", ok: false},
		{name: "with space", ok: false},
	}
	for _, tc := range cases {
		if got := isRunPartsCandidate(tc.name); got != tc.ok {
			t.Fatalf("candidate(%q) = %v, want %v", tc.name, got, tc.ok)
		}
	}
}
