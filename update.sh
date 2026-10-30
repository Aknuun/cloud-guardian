#!/usr/bin/env bash
# ============================================================
# Cloud Guardian auto-update — run by cron every 10 minutes
# Deploys only when a new tag or release exists in the GitHub repo.
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
DIR="${HOME}/.cloud-guardian"
CFG="${DIR}/config.json"

[ -f "$CFG" ] || exit 0
[ -f "${DIR}/deploy-tool.py" ] || exit 0

command -v curl >/dev/null 2>&1 || exit 0
command -v python3 >/dev/null 2>&1 || exit 0

OLDV=$(python3 - "${CFG}" <<'PY'
import json,sys
try:
    print(json.load(open(sys.argv[1])).get("version",""))
except Exception:
    print("")
PY
)

# Latest repo version (highest among tags and releases) — tags may start with v (e.g. v1.0.3)
# Draft and pre-release entries are ignored (stable releases only)
LATEST_TAG=$(python3 - <<'PY' 2>/dev/null || true
import json, re, urllib.request
REPO = "Aknuun/cloud-guardian"
def fetch(url):
    try:
        req = urllib.request.Request(url, headers={"Accept": "application/vnd.github+json", "User-Agent": "cloud-guardian-updater"})
        with urllib.request.urlopen(req, timeout=90) as r:
            return json.loads(r.read().decode("utf-8", "replace"))
    except Exception:
        return []
def key(n):
    m = re.match(r"v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$", str(n).strip())
    return tuple(int(x or 0) for x in m.groups()) if m else None
tags = [t.get("name", "") for t in fetch(f"https://api.github.com/repos/{REPO}/tags?per_page=100") if isinstance(t, dict)]
tags = [t for t in tags if key(t)]
rels = fetch(f"https://api.github.com/repos/{REPO}/releases?per_page=20")
rel_tags = [r.get("tag_name", "") for r in rels if isinstance(r, dict) and not r.get("draft") and not r.get("prerelease")]
rel_tags = [t for t in rel_tags if key(t)]
all_tags = tags + rel_tags
print(max(all_tags, key=key) if all_tags else "")
PY
)
[ -n "$LATEST_TAG" ] || exit 0

# Numeric version without the v prefix, for comparison
LATEST="${LATEST_TAG#v}"

# No new tag/release compared to the installed version: do nothing
[ "$LATEST" = "$OLDV" ] && exit 0
if [ -n "$OLDV" ]; then
  NEWEST=$(printf '%s\n%s\n' "$OLDV" "$LATEST" | sort -V | tail -1)
  [ "$NEWEST" = "$LATEST" ] || exit 0
fi

TMP="${DIR}/worker.js.new"
DTMP="${DIR}/deploy-tool.py.new"
curl -fsSL --max-time 90 -o "$TMP" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${LATEST_TAG}" || exit 0
# Fetch deploy-tool from the same tag; otherwise an older deploy-tool deploys
# empty bindings and the bot dies. If the download fails, skip this round.
curl -fsSL --max-time 90 -o "$DTMP" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/deploy-tool.py?ref=${LATEST_TAG}" || { rm -f "$TMP"; exit 0; }

NEWV=$(sed -n 's/^const BOT_VERSION = "\([^"]*\)".*/\1/p' "$TMP" | head -1)

if [ -z "$NEWV" ]; then
  rm -f "$TMP" "$DTMP"
  exit 0
fi
# deploy-tool must be healthy
grep -q "def update" "$DTMP" || { rm -f "$TMP" "$DTMP"; exit 0; }
# Do not deploy if the in-code version is not newer than the installed one
# (prevents repeated deploy loops and accidental downgrades)
if [ -n "$OLDV" ]; then
  NEWEST2=$(printf '%s\n%s\n' "$OLDV" "$NEWV" | sort -V | tail -1)
  if [ "$NEWEST2" != "$NEWV" ] || [ "$NEWV" = "$OLDV" ]; then
    rm -f "$TMP" "$DTMP"
    exit 0
  fi
fi

mv "$TMP" "${DIR}/worker.js"
mv "$DTMP" "${DIR}/deploy-tool.py"
chmod +x "${DIR}/deploy-tool.py" 2>/dev/null || true
cd "$DIR"
if python3 deploy-tool.py update; then
  python3 - "${CFG}" "$NEWV" <<'PY'
import json,sys
c=json.load(open(sys.argv[1])); c["version"]=sys.argv[2]
json.dump(c, open(sys.argv[1],"w"), ensure_ascii=False, indent=2)
PY
  echo "$(date) — updated to $NEWV (tag/release $LATEST_TAG)."
else
  echo "$(date) — update to $NEWV failed (see log above)."
fi
