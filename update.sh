#!/usr/bin/env bash
# ============================================================
# به‌روزرسانی خودکار نگهبان ابری — توسط کرون هر ۱۰ دقیقه اجرا می‌شود
# فقط وقتی در مخزن گیت‌هاب «تگ نسخهٔ جدید» ساخته شده باشد دیپلوی می‌کند.
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

# آخرین تگ نسخه‌ای مخزن (بزرگ‌ترین نسخه)
LATEST=$(curl -fsSL --max-time 90 -H 'Accept: application/vnd.github+json' \
  "https://api.github.com/repos/${REPO}/tags?per_page=100" 2>/dev/null \
  | python3 -c '
import sys,json,re
try:
    data=json.load(sys.stdin)
except Exception:
    sys.exit(0)
def key(n):
    m=re.match(r"v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$", n.strip())
    return tuple(int(x or 0) for x in m.groups()) if m else None
tags=[t.get("name","") for t in data if isinstance(t,dict)]
tags=[t for t in tags if key(t)]
print(max(tags, key=key) if tags else "")
' 2>/dev/null || true)
[ -n "$LATEST" ] || exit 0

# اگر تگ جدیدی نسبت به نسخهٔ نصب‌شده نبود، کاری نکن
[ "$LATEST" = "$OLDV" ] && exit 0
if [ -n "$OLDV" ]; then
  NEWEST=$(printf '%s\n%s\n' "$OLDV" "$LATEST" | sort -V | tail -1)
  [ "$NEWEST" = "$LATEST" ] || exit 0
fi

TMP="${DIR}/worker.js.new"
curl -fsSL --max-time 90 -o "$TMP" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${LATEST}" || exit 0

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
  echo "$(date) — به نسخهٔ $NEWV (تگ $LATEST) به‌روزرسانی شد."
else
  echo "$(date) — به‌روزرسانی به $NEWV ناموفق بود (لاگ بالا)."
fi
