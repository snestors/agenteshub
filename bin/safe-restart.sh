#!/usr/bin/env bash
# safe-restart.sh — signals the daemon to restart once all in-flight turns complete.
#
# Behavior:
#   1. Calls /api/runs/schedule-restart so the daemon waits for active runs
#   2. Falls back to direct `sudo systemctl restart agenthub` if the API is down
#   3. If --wait is passed, polls /healthz post-restart and notifies on failure
#
# This script does NOT back up the binary. Backup is the responsibility of
# bin/promote.sh, which runs the cp BEFORE swapping bin/agenthub.next into place.
# Restarting without a fresh promote (e.g. after editing config) is fine — there
# is no "new" binary to back up.
#
# Rollback if the new binary breaks:
#   cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh

set -euo pipefail

cd "$(dirname "$0")/.."

API="http://127.0.0.1:8093"
SCHEDULE_URL="$API/api/runs/schedule-restart"
RUNS_URL="$API/api/runs"
HEALTH_URL="$API/healthz"

WAIT_FOR_RESTART=0
if [ "${1:-}" = "--wait" ]; then
  WAIT_FOR_RESTART=1
fi

# ─── 1. Schedule via daemon API ─────────────────────────────────────────────
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

# ─── 2. Optional: wait for restart to complete + verify ────────────────────
if [ "$WAIT_FOR_RESTART" = "1" ]; then
  echo "[safe-restart] --wait: polling for restart completion (max 5 min)"

  # Phase 2a — wait for pending_restart=false (means the daemon actually restarted)
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

  # Phase 2b — verify /healthz is OK with new version
  sleep 2  # let the daemon settle
  HEALTH=$(curl -sf "$HEALTH_URL" 2>/dev/null || echo "")
  if [ -z "$HEALTH" ] || ! echo "$HEALTH" | grep -q '"ok":true'; then
    echo "[safe-restart] FAIL: post-restart /healthz NOT OK"
    echo "$HEALTH" | head -20
    echo "[safe-restart] Rollback hint:"
    echo "  cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh"
    exit 3
  fi

  NEW_V=$(echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"{d.get('version','?')} ({d.get('git_commit','?')})\")" 2>/dev/null)
  echo "[safe-restart] OK: daemon healthy at $NEW_V"
fi
