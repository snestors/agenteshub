package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadHarnessFile_MissingReportsExistsFalse(t *testing.T) {
	dir := t.TempDir()
	out := readHarnessFile(filepath.Join(dir, "nope.md"), "nope.md", harnessFileMaxBytes)
	if out["exists"].(bool) {
		t.Errorf("missing file should report exists=false")
	}
	if out["content"].(string) != "" {
		t.Errorf("missing file should have empty content")
	}
	if out["size"].(int64) != 0 {
		t.Errorf("missing file size should be 0")
	}
}

func TestReadHarnessFile_SmallFileFullContent(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "current.md")
	body := "session start: x\n- did A\n- did B\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := readHarnessFile(full, "progress/current.md", harnessFileMaxBytes)
	if !out["exists"].(bool) {
		t.Errorf("file should exist")
	}
	if out["truncated"].(bool) {
		t.Errorf("small file should not be truncated")
	}
	if out["content"].(string) != body {
		t.Errorf("content mismatch: %q vs %q", out["content"], body)
	}
}

func TestReadHarnessFile_TruncationCap(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "history.md")
	body := strings.Repeat("a", 2000)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := readHarnessFile(full, "progress/history.md", 1000)
	if !out["truncated"].(bool) {
		t.Errorf("file larger than cap should be truncated")
	}
	if len(out["content"].(string)) != 1000 {
		t.Errorf("content len = %d, want 1000", len(out["content"].(string)))
	}
	if out["size"].(int64) != 2000 {
		t.Errorf("size = %d, want 2000 (real size, not truncated len)", out["size"])
	}
}

func writeInitScript(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "init.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunHarnessInit_Success(t *testing.T) {
	dir := t.TempDir()
	writeInitScript(t, dir, "#!/usr/bin/env bash\necho hello $AGENTHUB_PROJECT_NAME\nexit 0\n")
	res := runHarnessInit(context.Background(), dir,
		[]string{"AGENTHUB_PROJECT_NAME=alpha"},
		5*time.Second, 256*1024)
	if res.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", res.ExitCode)
	}
	if !strings.Contains(res.Combined, "hello alpha") {
		t.Errorf("combined missing greeting: %q", res.Combined)
	}
	if res.TimedOut {
		t.Errorf("should not have timed out")
	}
	if res.Truncated {
		t.Errorf("output should not be truncated")
	}
}

func TestRunHarnessInit_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	writeInitScript(t, dir, "#!/usr/bin/env bash\necho boom >&2\nexit 7\n")
	res := runHarnessInit(context.Background(), dir, nil, 5*time.Second, 256*1024)
	if res.ExitCode != 7 {
		t.Errorf("exit_code = %d, want 7", res.ExitCode)
	}
	if !strings.Contains(res.Combined, "boom") {
		t.Errorf("combined missing stderr line: %q", res.Combined)
	}
}

func TestRunHarnessInit_Timeout(t *testing.T) {
	dir := t.TempDir()
	writeInitScript(t, dir, "#!/usr/bin/env bash\nsleep 5\n")
	res := runHarnessInit(context.Background(), dir, nil, 200*time.Millisecond, 256*1024)
	if !res.TimedOut {
		t.Errorf("expected timed_out=true")
	}
	if res.ExitCode != -2 {
		t.Errorf("exit_code = %d, want -2 (timeout sentinel)", res.ExitCode)
	}
}

func TestRunHarnessInit_OutputTruncated(t *testing.T) {
	dir := t.TempDir()
	// 4 KiB of output, cap at 1 KiB.
	writeInitScript(t, dir, "#!/usr/bin/env bash\nfor i in $(seq 1 100); do printf '%.0sa' $(seq 1 40); echo; done\n")
	res := runHarnessInit(context.Background(), dir, nil, 5*time.Second, 1024)
	if !res.Truncated {
		t.Errorf("expected truncated=true")
	}
	if len(res.Combined) > 1024 {
		t.Errorf("combined len = %d, want <= 1024", len(res.Combined))
	}
}
