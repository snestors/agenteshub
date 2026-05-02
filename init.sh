#!/usr/bin/env bash
# init.sh — BettaTech harness validator.
#
# Detects the project's stack and runs the standard checks. Designed to be
# called from `POST /api/projects/{id}/harness/init` (synchronous, with
# timeout) and locally. Exit 0 = green light, anything else = stop.
#
# Conventions:
#   - Each check prints its own header so failures are easy to find in the
#     combined output.
#   - Slow checks (full test suites) run last so quick lints fail fast.
#   - When AGENTHUB_HARNESS=1 we know we're being called by the daemon and can
#     skip prompts / colour codes.

set -euo pipefail

cd "$(dirname "$0")"

ROOT="$(pwd)"
HEADER='==='
FAIL=0

step() { printf "\n%s %s %s\n" "$HEADER" "$1" "$HEADER"; }
fail() { printf "FAIL: %s\n" "$1" >&2; FAIL=$((FAIL+1)); }

# ─── 1. Sanity ────────────────────────────────────────────────────────────
step "sanity"
[ -f "feature_list.json" ] || fail "feature_list.json missing — is the harness scaffolded?"
[ -d ".claude/agents" ]    || fail ".claude/agents missing"
[ -d "progress" ]          || fail "progress/ missing"
echo "ok"

# ─── 2. Stack detection + checks ──────────────────────────────────────────
step "stack"
DETECTED=()

if [ -f "go.mod" ]; then
  DETECTED+=("go")
  echo "→ go.mod detected"
  if command -v go > /dev/null 2>&1; then
    step "go test"
    if ! go test ./... 2>&1; then
      fail "go test failed"
    fi
  else
    fail "go.mod present but 'go' not on PATH"
  fi
fi

if [ -f "package.json" ]; then
  DETECTED+=("node")
  echo "→ package.json detected"
  PM=""
  if [ -f "pnpm-lock.yaml" ]; then PM="pnpm"
  elif [ -f "yarn.lock" ];     then PM="yarn"
  elif [ -f "package-lock.json" ]; then PM="npm"
  fi
  if [ -n "$PM" ] && command -v "$PM" > /dev/null 2>&1; then
    step "$PM build"
    if ! "$PM" run build 2>&1; then
      fail "$PM run build failed"
    fi
  else
    echo "skip: no recognized package manager on PATH (pnpm/yarn/npm)"
  fi
fi

if [ -f "pyproject.toml" ] || [ -f "requirements.txt" ]; then
  DETECTED+=("python")
  echo "→ python project detected"
  if command -v pytest > /dev/null 2>&1 && [ -d "tests" ]; then
    step "pytest"
    if ! pytest -q 2>&1; then
      fail "pytest failed"
    fi
  fi
fi

if [ -f "Cargo.toml" ]; then
  DETECTED+=("rust")
  echo "→ Cargo.toml detected"
  if command -v cargo > /dev/null 2>&1; then
    step "cargo test"
    if ! cargo test 2>&1; then
      fail "cargo test failed"
    fi
  fi
fi

if [ ${#DETECTED[@]} -eq 0 ]; then
  echo "WARN: no recognized stack (go/node/python/rust). Add project-specific checks here."
fi

# ─── 3. Result ────────────────────────────────────────────────────────────
step "result"
if [ "$FAIL" -eq 0 ]; then
  echo "OK — all checks passed"
  exit 0
fi
echo "FAILED with $FAIL error(s)"
exit 1
