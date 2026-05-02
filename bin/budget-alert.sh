#!/usr/bin/env bash
# budget-alert.sh — checks Anthropic API spend against a monthly budget and
# pings the user via WhatsApp when a threshold is crossed.
#
# Pulls /api/internal/usage (loopback-only, no JWT) and posts to
# /api/internal/notify when month-to-date cost crosses 50/75/90/100/125% of the
# configured monthly budget. Idempotent per month: state in
# data/budget-alert.state remembers the highest threshold already alerted, so a
# cron run every N minutes won't spam.
#
# Env:
#   AGENTHUB_BUDGET_USD   — monthly cap in USD (default: 50)
#   AGENTHUB_API          — daemon base URL (default: http://127.0.0.1:8093)
#
# Usage:
#   bin/budget-alert.sh           # run once, exit non-zero on infrastructure errors
#   bin/budget-alert.sh --status  # print current totals + state, no alerts
#
# Cron suggestion (every 30 min):
#   */30 * * * * cd /home/nestor/agenthub && bin/budget-alert.sh >> logs/budget-alert.log 2>&1

set -euo pipefail

cd "$(dirname "$0")/.."

API="${AGENTHUB_API:-http://127.0.0.1:8093}"
BUDGET="${AGENTHUB_BUDGET_USD:-50}"
STATE="data/budget-alert.state"
USAGE_URL="$API/api/internal/usage"
NOTIFY_URL="$API/api/internal/notify"
THRESHOLDS=(50 75 90 100 125)

mkdir -p data logs

# ─── 1. Pull usage ─────────────────────────────────────────────────────────
USAGE_JSON="$(curl -sf --max-time 5 "$USAGE_URL" || true)"
if [ -z "$USAGE_JSON" ]; then
  echo "[budget-alert] $(date -Is) ERROR: $USAGE_URL unreachable" >&2
  exit 1
fi

MONTH_COST="$(echo "$USAGE_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['month']['cost_usd'])")"
TODAY_COST="$(echo "$USAGE_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['today']['cost_usd'])")"
NOW_TS="$(echo "$USAGE_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['now'])")"
MONTH_KEY="$(date -u -d "@$NOW_TS" +%Y-%m)"

# Percentage spent (integer, rounded down).
PCT="$(python3 -c "print(int($MONTH_COST / $BUDGET * 100))" 2>/dev/null || echo 0)"

if [ "${1:-}" = "--status" ]; then
  echo "month=$MONTH_KEY cost=\$$MONTH_COST today=\$$TODAY_COST budget=\$$BUDGET pct=$PCT%"
  if [ -f "$STATE" ]; then
    echo "state: $(cat "$STATE")"
  else
    echo "state: (none)"
  fi
  exit 0
fi

# ─── 2. Determine highest threshold reached ────────────────────────────────
HIT=0
for t in "${THRESHOLDS[@]}"; do
  if [ "$PCT" -ge "$t" ]; then
    HIT="$t"
  fi
done

if [ "$HIT" -eq 0 ]; then
  echo "[budget-alert] $(date -Is) OK month=$MONTH_KEY pct=${PCT}% (\$$MONTH_COST / \$$BUDGET) — below 50%"
  exit 0
fi

# ─── 3. Compare against state, alert only on a new (higher) threshold ──────
PREV_MONTH=""
PREV_HIT=0
if [ -f "$STATE" ]; then
  PREV_MONTH="$(awk -F= '/^month=/ {print $2}' "$STATE" 2>/dev/null || true)"
  PREV_HIT="$(awk -F= '/^max_threshold=/ {print $2}' "$STATE" 2>/dev/null || echo 0)"
fi

# Reset on month rollover.
if [ "$PREV_MONTH" != "$MONTH_KEY" ]; then
  PREV_HIT=0
fi

if [ "$HIT" -le "$PREV_HIT" ]; then
  echo "[budget-alert] $(date -Is) already alerted ${PREV_HIT}% for $MONTH_KEY — current=${HIT}% (no-op)"
  exit 0
fi

# ─── 4. Send WhatsApp notification ─────────────────────────────────────────
MSG="⚠ Anthropic API budget alert
Month $MONTH_KEY: \$${MONTH_COST} / \$${BUDGET} (${PCT}%) — crossed ${HIT}% threshold
Today so far: \$${TODAY_COST}"

PAYLOAD="$(python3 -c "import json,sys; print(json.dumps({'body': sys.argv[1]}))" "$MSG")"

NOTIFY_RESP="$(curl -sf --max-time 5 \
  -H 'Content-Type: application/json' \
  -X POST -d "$PAYLOAD" \
  "$NOTIFY_URL" || true)"

if [ -z "$NOTIFY_RESP" ]; then
  echo "[budget-alert] $(date -Is) ERROR: notify endpoint failed (msg not delivered, state NOT updated)" >&2
  exit 2
fi

# ─── 5. Persist new state so we don't spam on the next tick ────────────────
{
  echo "month=$MONTH_KEY"
  echo "max_threshold=$HIT"
  echo "last_cost_usd=$MONTH_COST"
  echo "last_check=$(date -Is)"
} > "$STATE"

echo "[budget-alert] $(date -Is) ALERTED month=$MONTH_KEY pct=${PCT}% threshold=${HIT}% cost=\$${MONTH_COST}"
