#!/usr/bin/env bash
# ============================================================
# Cloud Guardian — one-line installer for Linux servers
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

command -v curl >/dev/null 2>&1 || e "curl is not installed."
command -v python3 >/dev/null 2>&1 || e "python3 is not installed."

b "Downloading project files..."
mkdir -p "$DIR"
curl -fsSL -o "$DIR/worker.js" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/worker.js?ref=${BRANCH}" || e "Failed to download worker.js."
curl -fsSL -o "${DIR}/deploy-tool.py" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/deploy-tool.py?ref=${BRANCH}" || e "Failed to download deploy-tool.py."
chmod +x "${DIR}/deploy-tool.py"

echo ""
echo "=== Required Information ==="
echo ""

read -r -p "Cloudflare API token (Workers Scripts Edit + Workers KV + Zone DNS): " CF_TOKEN
[ -n "$CF_TOKEN" ] || e "Token is required."

echo "   Trying to auto-detect Account ID..."
ACC_ID=$(curl -sS -H "Authorization: Bearer $CF_TOKEN" "$API/accounts?per_page=50" | python3 -c "import sys,json;r=json.load(sys.stdin);print(r['result'][0]['id'] if r.get('success') and r.get('result') else '')" 2>/dev/null || echo "")
if [ -z "$ACC_ID" ]; then
  w "Account ID could not be detected automatically; please enter it manually (Cloudflare dashboard → right sidebar → Account ID)."
  read -r -p "Account ID: " ACC_ID
fi
[ -n "$ACC_ID" ] || e "Account ID is required."

read -r -p "Worker name (default: cloud-guardian): " WK
WK="${WK:-cloud-guardian}"

read -r -p "Telegram bot token (from @BotFather): " BOT_TOKEN
[ -n "$BOT_TOKEN" ] || e "Bot token is required."

read -r -p "Your Telegram numeric ID as admin (from @userinfobot): " ADMIN_ID
[ -n "$ADMIN_ID" ] || e "Admin ID is required."

echo ""
b "Preparing config..."
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

b "Installing/deploying to Cloudflare (KV namespace + bindings + worker + cron)..."
cd "$DIR"
python3 deploy-tool.py install || e "Deploy failed."

b "Connecting Telegram webhook..."
SUB=$(curl -sS -H "Authorization: Bearer $CF_TOKEN" "$API/accounts/$ACC_ID/workers/subdomain" | python3 -c "import sys,json;r=json.load(sys.stdin);print(r['result']['subdomain'] if r.get('success') and r.get('result') and r['result'].get('subdomain') else '')" 2>/dev/null || echo "")
if [ -n "$SUB" ]; then
  WEB_URL="https://$WK.$SUB.workers.dev/tg"
  WH=$(curl -sS "https://api.telegram.org/bot$BOT_TOKEN/setWebhook?url=$WEB_URL&drop_pending_updates=true")
  echo "$WH" | grep -q '"ok":true' && b "Webhook set: $WEB_URL" || w "setWebhook: $WH"
  echo "$WEB_URL" > "$DIR/webhook_url"
else
  w "workers.dev subdomain is disabled for this account; add a custom route/domain to the worker and set the Telegram webhook manually."
fi

b "Installing auto-update (optional cron backup)..."
curl -fsSL -o "$DIR/update.sh" -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/${REPO}/contents/update.sh?ref=${BRANCH}"
chmod +x "$DIR/update.sh"
( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ; echo "*/30 * * * * $DIR/update.sh >> $DIR/update.log 2>&1" ) | crontab -

echo ""
echo "======================================================"
echo "  ✅ Installation complete — Cloud Guardian is live."
echo "======================================================"
echo ""
echo "  • Send /start in Telegram to open the menu."
echo "  • Updates are applied automatically from the GitHub repo inside the worker itself; the cron is only a backup."
echo "  • The /start ad can be changed by the admin with /promoset (no customer update needed)."
echo "  • Config file: $CFG (keep it secret)"
echo ""