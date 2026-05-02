#!/usr/bin/env bash
# safe-restart.sh — signals the daemon to restart once all in-flight turns complete.
#
# Behavior:
#   1. Backs up the currently running binary to bin/agenthub.prev (rollback safety net)
#   2. Calls /api/runs/schedule-restart so the daemon waits for active runs
#   3. Falls back to direct `sudo systemctl restart agenthub` if the API is down
#   4. If --wait is passed, polls /healthz post-restart and notifies on failure
#
# Rollback if the new binary breaks:
#   cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh

set -euo pipefail

cd "$(dirname "$0")/.."

API="http://127.0.0.1:8093"
SCHEDULE_URL="$API/api/runs/schedule-restart"
RUNS_URL="$API/api/runs"
HEALTH_URL="$API/healthz"
BIN="bin/agenthub"
PREV="bin/agenthub.prev"

WAIT_FOR_RESTART=0
if [ "${1:-}" = "--wait" ]; then
  WAIT_FOR_RESTART=1
fi

# ─── 1. Backup current binary ───────────────────────────────────────────────
if [ -f "$BIN" ]; then
  cp -p "$BIN" "$PREV"
  echo "[safe-restart] Backup: $PREV ($(./bin/agenthub.prev version 2>/dev/null || echo 'unknown'))"
else
  echo "[safe-restart] WARN: no encuentro $BIN — sigo sin backup"
fi

# ─── 2. Schedule via daemon API ─────────────────────────────────────────────
echo "[safe-restart] Scheduling restart..."
RESPONSE=$(curl -sf -X POST "$SCHEDULE_URL" 2>/dev/null || echo "")

if [ -z "$RESPONSE" ]; then
  echo "[safe-restart] API unreachable. Restarting directly..."
  sudo systemctl restart agenthub
else
  ACTIVE=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('active_runs', 0))" 2>/dev/null || echo "0")
  if [ "$ACTIVE" = "0" ]; then
    echo "[safe-restart] No active runs — restart triggered immediately."
  else
    echo "[safe-restart] $ACTIVE active run(s) in flight — restart will happen once they complete."
  fi
fi

# ─── 3. Optional: wait for restart to complete + verify ────────────────────
if [ "$WAIT_FOR_RESTART" = "1" ]; then
  echo "[safe-restart] --wait: polling for restart completion (max 5 min)"

  # Phase 3a — wait for pending_restart=false (means the daemon actually restarted)
  WAIT_OK=0
  for i in $(seq 1 60); do  # 60 polls × 5s = 5 min
    RUNS_JSON=$(curl -sf "$RUNS_URL" 2>/dev/null || echo "")
    if [ -n "$RUNS_JSON" ]; then
      PENDING=$(echo "$RUNS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('pending_restart', True))" 2>/dev/null || echo "True")
      if [ "$PENDING" = "False" ]; then
        WAIT_OK=1
        echo "[safe-restart] Restart completed after ${i}×5s polls"
        break
      fi
    fi
    sleep 5
  done

  if [ "$WAIT_OK" != "1" ]; then
    echo "[safe-restart] WARN: restart did not complete within 5 min — daemon may still be busy or stuck"
    exit 2
  fi

  # Phase 3b — verify /healthz is OK with new version
  sleep 2  # let the daemon settle
  HEALTH=$(curl -sf "$HEALTH_URL" 2>/dev/null || echo "")
  if [ -z "$HEALTH" ] || ! echo "$HEALTH" | grep -q '"ok":true'; then
    echo "[safe-restart] FAIL: post-restart /healthz NOT OK"
    echo "$HEALTH" | head -20
    echo "[safe-restart] Rollback hint:"
    echo "  cp $PREV $BIN && sudo systemctl restart agenthub"
    exit 3
  fi

  NEW_V=$(echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"{d.get('version','?')} ({d.get('git_commit','?')})\")" 2>/dev/null)
  echo "[safe-restart] OK: daemon healthy at $NEW_V"
fi
