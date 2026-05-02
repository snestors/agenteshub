package harness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

// Default and ceiling for init.sh wall-clock; kept here so HTTP and MCP
// surface the same numbers without re-deriving.
const (
	InitDefaultTimeout = 300 * time.Second
	InitMaxTimeout     = 30 * time.Minute
	InitOutputCap      = 256 * 1024
)

// InitResult captures the outcome of RunInit. Sentinel exit codes:
//
//	-1 spawn failed (e.g. missing interpreter)
//	-2 killed by timeout
//
// Combined holds stdout+stderr merged, capped at outputCap.
type InitResult struct {
	ExitCode   int    `json:"exit_code"`
	Combined   string `json:"combined"`
	Truncated  bool   `json:"truncated"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timeout"`
}

// RunInit runs `bash init.sh` in dir with the given extra env, capping
// wall-clock time at timeout and combined output at outputCap bytes.
//
// Caveat: when init.sh spawns long-running child processes (e.g. a literal
// `sleep 60`), CommandContext kills bash on timeout but the orphans run to
// completion. The call returns once bash exits, which can be later than
// `timeout` suggests. Acceptable for 0.4.0 — init.sh is meant to be a quick
// validator. If we hit a real-world stall we'll switch to a process group +
// kill -KILL on the pgid.
func RunInit(parent context.Context, dir string, extraEnv []string, timeout time.Duration, outputCap int) InitResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "init.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	duration := time.Since(start)

	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)

	truncated := false
	if outputCap > 0 && len(out) > outputCap {
		out = out[:outputCap]
		truncated = true
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case timedOut:
			exitCode = -2
		case errors.As(runErr, &exitErr):
			exitCode = exitErr.ExitCode()
		default:
			exitCode = -1
		}
	}

	return InitResult{
		ExitCode:   exitCode,
		Combined:   string(out),
		Truncated:  truncated,
		DurationMs: duration.Milliseconds(),
		TimedOut:   timedOut,
	}
}
