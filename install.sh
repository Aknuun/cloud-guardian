#!/usr/bin/env bash
# ============================================================
# نگهبان ابری — نصب تک‌خطی روی سرور لینوکسی
# bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install.sh)"
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
BRANCH="main"
API="https://api.cloudflare.com/client/v4"
DIR="${HOME}/.cloud-guardian"
CFG="${DIR}/config.json"

b(){ printf '\033[1;34m[*]\033[0m %s\n' "$*"; }
w(){ printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
e(){ printf '\033[1;31m[✗]\033[0m %s\n' "$*"; exit 1; }

command -v curl >/dev/null 2>&1 || e "curl نصب نیست."
command -v python3 >/dev/null 2>&1 || e "python3 نصب نیست."

b "دانلود فایل‌های پروژه…"
mkdir -p "$DIR"
curl -fsSL -o "$DIR/worker.js" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${BRANCH}" || e "دانلود worker.js ناموفق بود."
curl -fsSL -o "${DIR}/deploy-tool.py" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/deploy-tool.py?ref=${BRANCH}" || e "دانلود deploy-tool.py ناموفق بود."
chmod +x "${DIR}/deploy-tool.py"

echo ""
echo "═══ اطلاعات مورد نیاز ═══"
echo ""

read -r -p "توکن API کلادفلر (Workers Scripts Edit + Workers KV + Zone DNS): " CF_TOKEN
[ -n "$CF_TOKEN" ] || e "توکن لازم است."

echo "   تلاش برای یافتن خودکار Account ID…"
ACC_ID=$(curl -sS -H "Authorization: Bearer $CF_TOKEN" "$API/accounts?per_page=50" | python3 -c "import sys,json;r=json.load(sys.stdin);print(r['result'][0]['id'] if r.get('success') and r.get('result') else '')" 2>/dev/null || echo "")
if [ -z "$ACC_ID" ]; then
  w "Account ID خودکار پیدا نشد؛ دستی وارد کنید (از دشبورد کلادفلر → حساب → Account ID)."
  read -r -p "Account ID: " ACC_ID
fi
[ -n "$ACC_ID" ] || e "Account ID لازم است."

read -r -p "نام ورکر (پیش‌فرض cloud-guardian): " WK
WK="${WK:-cloud-guardian}"

read -r -p "توکن ربات تلگرام (از @BotFather): " BOT_TOKEN
[ -n "$BOT_TOKEN" ] || e "توکن ربات لازم است."

read -r -p "شناسهٔ عددی تلگرام مدیر (عدد، از /myid): " ADMIN_ID
[ -n "$ADMIN_ID" ] || e "شناسهٔ مدیر لازم است."

echo ""
b "آماده‌سازی config…"
python3 - "$CFG" "$ACC_ID" "$WK" "$CF_TOKEN" "$BOT_TOKEN" "$ADMIN_ID" <<'PY'
import json,sys
cfg_file, acc, wk, tok, btok, aid = sys.argv[1:7]
cfg = {
  "account_id": acc,
  "worker": wk,
  "token": tok,
  "bot_token": btok,
  "admin_id": int(aid),
  "cf_accounts": [{"name": "main", "token": tok}],
  "compatibility_date": "2024-11-01",
  "usage_model": "standard",
  "version": "",
}
open(cfg_file, "w").write(json.dumps(cfg, ensure_ascii=False, indent=2))
PY
chmod 700 "$DIR"
chmod 600 "$CFG"

b "نصب/دیپلوی روی کلادفلر (ساخت KV + همینگ + ورکر + کرون)…"
cd "$DIR"
python3 deploy-tool.py install || e "دیپلوی ناموفق بود."

b "اتصال وب‌هوک تلگرام…"
SUB=$(curl -sS -H "Authorization: Bearer $CF_TOKEN" "$API/accounts/$ACC_ID/workers/subdomain" | python3 -c "import sys,json;r=json.load(sys.stdin);print(r['result']['subdomain'] if r.get('success') and r.get('result') and r['result'].get('subdomain') else '')" 2>/dev/null || echo "")
if [ -n "$SUB" ]; then
  WEB_URL="https://$WK.$SUB.workers.dev/tg"
  WH=$(curl -sS "https://api.telegram.org/bot$BOT_TOKEN/setWebhook?url=$WEB_URL&drop_pending_updates=true")
  echo "$WH" | grep -q '"ok":true' && b "وب‌هوک تنظیم شد: $WEB_URL" || w "setWebhook: $WH"
  echo "$WEB_URL" > "$DIR/webhook_url"
else
  w "workers.dev برای این اکانت غیرفعال است؛ یک Route/دامنهٔ سفارشی به ورکر اضافه کنید و وب‌هوک تلگرام را دستی روی آن ببندید."
fi

b "نصب آپدیت خودکار (کرون اختیاری، پشتیبان)…"
curl -fsSL -o "$DIR/update.sh" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/update.sh?ref=${BRANCH}"
chmod +x "$DIR/update.sh"
( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ; echo "*/30 * * * * $DIR/update.sh >> $DIR/update.log 2>&1" ) | crontab -

echo ""
echo "════════════════════════════════════════════════════"
echo "  ✅ نصب کامل شد — نگهبان ابری فعال است."
echo "════════════════════════════════════════════════════"
echo ""
echo "  • در تلگرام /start بزنید تا منو بیاید."
echo "  • آپدیت‌ها به‌صورت خودکار داخل خودِ ورکر از مخزن گیت‌هاب انجام می‌شود؛ کرون نصب‌شده فقط پشتیبان است."
echo "  • تبلیغ روی /start را مدیر با /promoset عوض می‌کند (بدون نیاز به آپدیت مشتری)."
echo "  • فایل تنظیمات: $CFG  (خودتان نگه دارید)"
echo ""