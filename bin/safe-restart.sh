#!/usr/bin/env bash
# safe-restart.sh — signals the daemon to restart once all in-flight turns complete.
#
# Behavior:
#   1. Backs up the currently running binary to bin/agenthub.prev (rollback safety net)
#   2. Calls /api/runs/schedule-restart so the daemon waits for active runs
#   3. Falls back to direct `sudo systemctl restart agenthub` if the API is down
#
# Rollback if the new binary breaks:
#   cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh

set -euo pipefail

cd "$(dirname "$0")/.."

API="http://127.0.0.1:8093/api/runs/schedule-restart"
BIN="bin/agenthub"
PREV="bin/agenthub.prev"

# ─── 1. Backup current binary ───────────────────────────────────────────────
if [ -f "$BIN" ]; then
  cp -p "$BIN" "$PREV"
  echo "[safe-restart] Backup: $PREV ($(./bin/agenthub.prev version 2>/dev/null || echo 'unknown'))"
else
  echo "[safe-restart] WARN: no encuentro $BIN — sigo sin backup"
fi

# ─── 2. Schedule via daemon API ─────────────────────────────────────────────
echo "[safe-restart] Scheduling restart..."
RESPONSE=$(curl -sf -X POST "$API" 2>/dev/null || echo "")

if [ -z "$RESPONSE" ]; then
  echo "[safe-restart] API unreachable. Restarting directly..."
  sudo systemctl restart agenthub
  exit 0
fi

ACTIVE=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('active_runs', 0))" 2>/dev/null || echo "0")

if [ "$ACTIVE" = "0" ]; then
  echo "[safe-restart] No active runs — restart triggered immediately."
else
  echo "[safe-restart] $ACTIVE active run(s) in flight — restart will happen once they complete."
fi
