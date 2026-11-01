#!/usr/bin/env bash
# One-shot release: bump the version everywhere, verify, commit, tag, push.
# Usage: bash scripts/release.sh 4.57.0 "commit message"
# Any failure aborts before commit; nothing is tagged or pushed half-way.
set -euo pipefail

VER="${1:-}"
MSG="${2:-}"
if [[ ! "$VER" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: bash scripts/release.sh X.Y.Z \"message\"" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"


echo "▶ versions → $VER"
echo "$VER" > server/VERSION
sed -i "s/const Version = \"[0-9.]*\"/const Version = \"$VER\"/" server/internal/router/router.go
sed -i "s/BOT_VERSION = \"[0-9.]*\"/BOT_VERSION = \"$VER\"/" worker.js
grep -q "const Version = \"$VER\"" server/internal/router/router.go || { echo "router.go not bumped" >&2; exit 1; }
grep -q "BOT_VERSION = \"$VER\"" worker.js || { echo "worker.js not bumped" >&2; exit 1; }

echo "▶ ui build"
(cd webui-src && npm run lint && npm run build >/dev/null)

echo "▶ go checks"
(cd server && gofmt -l . | (! grep .) && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... >/dev/null)

echo "▶ sign install script"
SIGN_KEY="${CG_SIGN_KEY:-/root/cg-signing.key}"
if [[ -f "$SIGN_KEY" ]]; then
  (cd server && CGO_ENABLED=0 go run ./cmd/cgsign "$SIGN_KEY" ../install-server.sh) > install-server.sh.sig
  (cd server && CGO_ENABLED=0 go run ./cmd/cgsign "$SIGN_KEY" ../worker.js) > worker.js.sig
else
  echo "!! signing key missing at $SIGN_KEY — self-update will refuse this release" >&2
  rm -f install-server.sh.sig worker.js.sig
fi

echo "▶ commit + tag"
git add -A
git -c user.name=Aknuun -c user.email=aknuun@users.noreply.github.com commit -q -m "${MSG:-release v$VER}"
git tag -f "v$VER"

echo "▶ push"
URL="$(git remote get-url origin)"
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  git push -q "https://Aknuun:${GITHUB_TOKEN}@github.com/Aknuun/cloud-guardian.git" HEAD --tags
else
  git push -q origin HEAD --tags
fi

echo "✅ v$VER released"
git --no-pager log --oneline -1
echo "changed:"
git --no-pager show --stat --oneline HEAD | sed -n '2,40p'
