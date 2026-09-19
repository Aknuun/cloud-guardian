#!/usr/bin/env bash
# ============================================================
# به‌روزرسانی خودکار نگهبان ابری — توسط کرون هر ۳۰ دقیقه اجرا می‌شود
# اگر نسخه‌ای جدیدتر روی مخزن گیت‌هاب منتشر شده باشد، دیپلوی می‌کند.
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
BRANCH="main"
DIR="${HOME}/.cloud-guardian"
CFG="${DIR}/config.json"

[ -f "$CFG" ] || exit 0
[ -f "${DIR}/deploy-tool.py" ] || exit 0

command -v curl >/dev/null 2>&1 || exit 0

TMP="${DIR}/worker.js.new"
curl -fsSL --max-time 90 -o "$TMP" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${BRANCH}" || exit 0

NEWV=$(sed -n 's/^const BOT_VERSION = "\([^"]*\)".*/\1/p' "$TMP" | head -1)
OLDV=$(python3 - "${CFG}" <<'PY'
import json,sys
try:
    print(json.load(open(sys.argv[1])).get("version",""))
except Exception:
    print("")
PY
)

if [ -z "$NEWV" ]; then
  rm -f "$TMP"
  exit 0
fi

if [ "$NEWV" = "$OLDV" ]; then
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
  echo "$(date) — به نسخهٔ $NEWV به‌روزرسانی شد."
else
  echo "$(date) — به‌روزرسانی به $NEWV ناموفق بود (لاگ بالا)."
fi