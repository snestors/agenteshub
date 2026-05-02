#!/usr/bin/env bash
# setup-githooks.sh — point git at the versioned hooks under .githooks/
#
# Run once per clone of the repo. Idempotent — safe to re-run.

set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -d .githooks ]; then
  echo "error: no existe .githooks/ — ¿estás en la raíz del repo?" >&2
  exit 1
fi

# Make all hooks executable
chmod +x .githooks/*

git config --local core.hooksPath .githooks

echo "✓ git core.hooksPath = .githooks"
echo "✓ Hooks activos:"
ls -1 .githooks/ | sed 's/^/  /'
echo ""
echo "Para desactivar (volver al default):"
echo "  git config --local --unset core.hooksPath"
