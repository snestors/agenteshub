#!/usr/bin/env bash
# promote.sh — atomic promote of bin/agenthub.next to bin/agenthub.
#
# Order matters:
#   1. cp bin/agenthub      bin/agenthub.prev   (backup the binary CURRENTLY running)
#   2. mv bin/agenthub.next bin/agenthub        (promote)
#   3. bin/safe-restart.sh "$@"                 (graceful restart, optional --wait)
#
# Why the dedicated script: if you skip step 1 and let safe-restart.sh do it after
# step 2, the "previous" backup ends up identical to the new binary — there is no
# rollback target. promote.sh runs the backup BEFORE the mv so .prev keeps the
# version that is still running on the daemon.
#
# Usage:
#   bin/promote.sh           # promote + restart (returns when restart is scheduled)
#   bin/promote.sh --wait    # promote + restart + poll /healthz post-restart
#
# Rollback (if the new binary breaks):
#   cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh

set -euo pipefail

cd "$(dirname "$0")/.."

BIN="bin/agenthub"
NEXT="bin/agenthub.next"
PREV="bin/agenthub.prev"

[ -f "$NEXT" ] || { echo "[promote] error: $NEXT no existe — corré el build primero" >&2; exit 1; }
[ -x "$NEXT" ] || { echo "[promote] error: $NEXT no es ejecutable" >&2; exit 1; }

# ─── 1. Backup the binary that is currently running ────────────────────────
if [ -f "$BIN" ]; then
  cp -p "$BIN" "$PREV"
  PREV_VER="$(./"$PREV" version 2>/dev/null || echo 'unknown')"
  echo "[promote] Backup: $PREV ($PREV_VER)"
else
  echo "[promote] WARN: no encuentro $BIN — no hay backup, continúo igual"
fi

# ─── 2. Promote next → prod ────────────────────────────────────────────────
mv "$NEXT" "$BIN"
NEW_VER="$(./"$BIN" version 2>/dev/null || echo 'unknown')"
echo "[promote] Promoted: $BIN ($NEW_VER)"

# ─── 3. Hand off to safe-restart ───────────────────────────────────────────
exec bin/safe-restart.sh "$@"
