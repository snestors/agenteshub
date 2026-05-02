#!/usr/bin/env bash
# release.sh — bump version atomically across the 3 sources of truth + tag.
#
# Two-phase usage (the in-between is for editing RELEASE_NOTES manually):
#
#   bin/release.sh patch          # 0.2.65 -> 0.2.66 (prepare bump)
#   bin/release.sh minor          # 0.2.65 -> 0.3.0  (prepare bump)
#   bin/release.sh major          # 0.2.65 -> 1.0.0  (prepare bump)
#   bin/release.sh 0.4.2          # explicit version (prepare bump)
#
#   <edit RELEASE_NOTES.md by hand to fill the new entry>
#
#   bin/release.sh continue       # commit + git tag once notes are filled
#   bin/release.sh abort          # roll back the bump (no commit/tag yet)
#
# After 'continue', the deploy-safe-restart skill takes over (build + smoke).

set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[1;34m'
NC='\033[0m'

err()  { printf "${RED}error:${NC} %s\n" "$1" >&2; exit 1; }
ok()   { printf "${GREEN}\xE2\x9C\x93${NC} %s\n" "$1"; }
info() { printf "${BLUE}\xE2\x96\xB6${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}\xE2\x9A\xA0${NC} %s\n" "$1"; }

IN_FLIGHT=".release-in-flight"

# ────────────────────────────────────────────────────────────────────────────
# Subcommand: continue (after RELEASE_NOTES has been hand-edited)
# ────────────────────────────────────────────────────────────────────────────
if [ "${1:-}" = "continue" ]; then
  [ -f "$IN_FLIGHT" ] || err "no hay release in-flight (no existe .release-in-flight)"
  NEW_VERSION="$(cat "$IN_FLIGHT")"

  # Validate that RELEASE_NOTES top entry is no longer just placeholders
  TOP_BLOCK="$(awk "/^## v${NEW_VERSION} —/,/^---$/" RELEASE_NOTES.md)"
  if echo "$TOP_BLOCK" | grep -qE "_\(complete this entry"; then
    err "RELEASE_NOTES.md aún tiene placeholders en v${NEW_VERSION} — completá la entrada y reintentá"
  fi
  if echo "$TOP_BLOCK" | grep -qE "_\(or remove this section"; then
    warn "Quedaron secciones placeholder ('### Changed/### Fixed' con _'or remove this section'_)."
    warn "Si la entrada es solo Added, borrá las otras secciones a mano antes de continuar."
    err "abortando — limpia placeholders"
  fi

  git add VERSION frontend/package.json RELEASE_NOTES.md

  COMMIT_MSG="chore(release): v${NEW_VERSION}"
  git commit -m "$COMMIT_MSG"
  git tag "v${NEW_VERSION}"

  rm -f "$IN_FLIGHT"

  ok "Commit creado: $(git rev-parse --short HEAD) — $COMMIT_MSG"
  ok "Tag creado: v${NEW_VERSION}"
  echo ""
  info "Próximo paso: build + smoke + promote (ver skill deploy-safe-restart)"
  exit 0
fi

# ────────────────────────────────────────────────────────────────────────────
# Subcommand: abort (rollback prepared bump)
# ────────────────────────────────────────────────────────────────────────────
if [ "${1:-}" = "abort" ]; then
  [ -f "$IN_FLIGHT" ] || err "no hay release in-flight"
  git restore --staged VERSION frontend/package.json RELEASE_NOTES.md 2>/dev/null || true
  git checkout -- VERSION frontend/package.json RELEASE_NOTES.md
  rm -f "$IN_FLIGHT"
  ok "Bump abortado y archivos revertidos"
  exit 0
fi

# ────────────────────────────────────────────────────────────────────────────
# Default: prepare bump
# ────────────────────────────────────────────────────────────────────────────
[ $# -eq 1 ] || err "usage: $0 {patch|minor|major|X.Y.Z|continue|abort}"
BUMP="$1"

if [ -f "$IN_FLIGHT" ]; then
  err "ya hay un release in-flight para v$(cat "$IN_FLIGHT") — corré 'continue' o 'abort'"
fi

if ! git diff-index --quiet HEAD -- ; then
  err "hay cambios tracked sin commitear — commit o stash primero"
fi

CUR_VERSION="$(cat VERSION)"
CUR_FRONTEND="$(grep -E '"version"' frontend/package.json | head -1 | sed -E 's/.*"version": "([^"]+)".*/\1/')"
CUR_NOTES="$(grep -E '^## v[0-9]' RELEASE_NOTES.md | head -1 | sed -E 's/^## v([0-9.]+).*/\1/')"

info "Versiones actuales:"
printf "  VERSION:                %s\n" "$CUR_VERSION"
printf "  frontend/package.json:  %s\n" "$CUR_FRONTEND"
printf "  RELEASE_NOTES top:      %s\n" "$CUR_NOTES"

[ "$CUR_VERSION" = "$CUR_FRONTEND" ] || err "VERSION != frontend/package.json — alineá manual antes"
[ "$CUR_VERSION" = "$CUR_NOTES"    ] || err "VERSION != RELEASE_NOTES top — alineá manual antes"
ok "Las 3 fuentes alineadas en $CUR_VERSION"

IFS='.' read -r CUR_MAJ CUR_MIN CUR_PATCH <<< "$CUR_VERSION"
case "$BUMP" in
  patch) NEW_VERSION="${CUR_MAJ}.${CUR_MIN}.$((CUR_PATCH + 1))" ;;
  minor) NEW_VERSION="${CUR_MAJ}.$((CUR_MIN + 1)).0" ;;
  major) NEW_VERSION="$((CUR_MAJ + 1)).0.0" ;;
  [0-9]*.[0-9]*.[0-9]*) NEW_VERSION="$BUMP" ;;
  *) err "BUMP debe ser patch|minor|major o X.Y.Z, recibí: $BUMP" ;;
esac

info "Nueva versión: $CUR_VERSION → $NEW_VERSION"

echo "$NEW_VERSION" > VERSION
sed -i "s/\"version\": \"$CUR_VERSION\"/\"version\": \"$NEW_VERSION\"/" frontend/package.json
ok "VERSION + frontend/package.json bumpeados"

TODAY="$(date +%Y-%m-%d)"
TMP=$(mktemp)
cat > "$TMP" <<EOF
## v${NEW_VERSION} — ${TODAY}

### Added

- _(complete this entry before continuing the release)_

### Changed

- _(or remove this section if not applicable)_

### Fixed

- _(or remove this section if not applicable)_

---

EOF

awk -v insert_file="$TMP" '
  BEGIN { inserted = 0 }
  /^## v[0-9]/ && !inserted {
    while ((getline line < insert_file) > 0) print line
    close(insert_file)
    inserted = 1
  }
  { print }
' RELEASE_NOTES.md > RELEASE_NOTES.md.new
mv RELEASE_NOTES.md.new RELEASE_NOTES.md
rm -f "$TMP"
ok "RELEASE_NOTES.md skeleton agregado para v${NEW_VERSION}"

git add VERSION frontend/package.json RELEASE_NOTES.md

echo "$NEW_VERSION" > "$IN_FLIGHT"

printf "\n${YELLOW}Cambios staged:${NC}\n"
git diff --cached --stat
echo ""
warn "Editá RELEASE_NOTES.md, completá la entrada de v${NEW_VERSION}, y luego:"
echo ""
echo "  bin/release.sh continue   # commit + tag"
echo "  bin/release.sh abort      # rollback"
echo ""
