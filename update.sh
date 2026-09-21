#!/usr/bin/env bash
# ============================================================
# به‌روزرسانی خودکار نگهبان ابری — توسط کرون هر ۱۰ دقیقه اجرا می‌شود
# فقط وقتی در مخزن گیت‌هاب «تگ نسخهٔ جدید یا ریلیز جدید» ساخته شده باشد دیپلوی می‌کند.
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

# آخرین نسخهٔ مخزن (بزرگ‌ترین نسخه از بین تگ‌ها و ریلیزها) — تگ می‌تواند با v شروع شود (مثل v1.0.3)
# ریلیزهای درفت و پری‌ریلیز نادیده گرفته می‌شوند (فقط ریلیز پایدار)
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

# نسخهٔ عددی بدون پیشوند v برای مقایسه
LATEST="${LATEST_TAG#v}"

# اگر تگ/ریلیز جدیدی نسبت به نسخهٔ نصب‌شده نبود، کاری نکن
[ "$LATEST" = "$OLDV" ] && exit 0
if [ -n "$OLDV" ]; then
  NEWEST=$(printf '%s\n%s\n' "$OLDV" "$LATEST" | sort -V | tail -1)
  [ "$NEWEST" = "$LATEST" ] || exit 0
fi

TMP="${DIR}/worker.js.new"
curl -fsSL --max-time 90 -o "$TMP" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${LATEST_TAG}" || exit 0

NEWV=$(sed -n 's/^const BOT_VERSION = "\([^"]*\)".*/\1/p' "$TMP" | head -1)

if [ -z "$NEWV" ]; then
  rm -f "$TMP"
  exit 0
fi

mv "$TMP" "${DIR}/worker.js"
cd "$DIR"
if python3 deploy-tool.py update; then
  python3 - "${CFG}" "$NEWV" <<'PY'
import json,sys
c=json.load(open(sys.argv[1])); c["version"]=sys.argv[2]
json.dump(c, open(sys.argv[1],"w"), ensure_ascii=False, indent=2)
PY
  echo "$(date) — به نسخهٔ $NEWV (تگ/ریلیز $LATEST_TAG) به‌روزرسانی شد."
else
  echo "$(date) — به‌روزرسانی به $NEWV ناموفق بود (لاگ بالا)."
fi
