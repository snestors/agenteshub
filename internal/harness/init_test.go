package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeInit(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "init.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunInit_Success(t *testing.T) {
	dir := t.TempDir()
	writeInit(t, dir, "#!/usr/bin/env bash\necho hello $AGENTHUB_PROJECT_NAME\nexit 0\n")
	res := RunInit(context.Background(), dir,
		[]string{"AGENTHUB_PROJECT_NAME=alpha"},
		5*time.Second, InitOutputCap)
	if res.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", res.ExitCode)
	}
	if !strings.Contains(res.Combined, "hello alpha") {
		t.Errorf("combined missing greeting: %q", res.Combined)
	}
	if res.TimedOut || res.Truncated {
		t.Errorf("unexpected flags: %+v", res)
	}
}

func TestRunInit_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	writeInit(t, dir, "#!/usr/bin/env bash\necho boom >&2\nexit 7\n")
	res := RunInit(context.Background(), dir, nil, 5*time.Second, InitOutputCap)
	if res.ExitCode != 7 {
		t.Errorf("exit_code = %d, want 7", res.ExitCode)
	}
	if !strings.Contains(res.Combined, "boom") {
		t.Errorf("combined missing stderr line: %q", res.Combined)
	}
}

func TestRunInit_Timeout(t *testing.T) {
	dir := t.TempDir()
	writeInit(t, dir, "#!/usr/bin/env bash\nsleep 5\n")
	res := RunInit(context.Background(), dir, nil, 200*time.Millisecond, InitOutputCap)
	if !res.TimedOut {
		t.Errorf("expected timed_out=true")
	}
	if res.ExitCode != -2 {
		t.Errorf("exit_code = %d, want -2 (timeout sentinel)", res.ExitCode)
	}
}

func TestRunInit_Truncated(t *testing.T) {
	dir := t.TempDir()
	writeInit(t, dir, "#!/usr/bin/env bash\nfor i in $(seq 1 100); do printf '%.0sa' $(seq 1 40); echo; done\n")
	res := RunInit(context.Background(), dir, nil, 5*time.Second, 1024)
	if !res.Truncated {
		t.Errorf("expected truncated=true")
	}
	if len(res.Combined) > 1024 {
		t.Errorf("combined len = %d, want <= 1024", len(res.Combined))
	}
}
