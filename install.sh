#!/usr/bin/env bash
# ============================================================
# نگهبان ابری — نصب‌کننده و مدیر (Cloud Guardian)
#   install | update | uninstall | relay | status | check | help
#
# نصب یک‌خطی:
#   bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install.sh)"
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
RELAY_REPO="Aknuun/cloud-guardian-relay"
BRANCH="main"
DIR="${HOME}/.cloud-guardian"
CFG="${DIR}/config.json"
API="https://api.cloudflare.com/client/v4"
CF_TOKENS_URL="https://dash.cloudflare.com/profile/api-tokens"
CF_DASH_URL="https://dash.cloudflare.com"
DOC_TOKEN_URL="https://github.com/Aknuun/cloud-guardian/blob/main/docs/cloudflare-api-token.md"
RELAY_INSTALL_URL="https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh"

# --- رنگ‌ها (اگر خروجی ترمینال نیست، خاموش) ---
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  RST=$'\033[0m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; UL=$'\033[4m'
  BLUE=$'\033[1;34m'; CYAN=$'\033[1;36m'; GREEN=$'\033[1;32m'
  YELLOW=$'\033[1;33m'; RED=$'\033[1;31m'; MAG=$'\033[1;35m'
else
  RST=""; BOLD=""; DIM=""; UL=""; BLUE=""; CYAN=""; GREEN=""; YELLOW=""; RED=""; MAG=""
fi

b()    { printf "${BLUE}[*]${RST} %s\n" "$*"; }
step() { printf "\n${MAG}${BOLD}▶ %s${RST}\n" "$*"; }
ok()   { printf "${GREEN}✅ %s${RST}\n" "$*"; }
warn() { printf "${YELLOW}⚠️  %s${RST}\n" "$*"; }
err()  { printf "${RED}❌ %s${RST}\n" "$*" >&2; }
link() { printf "${BLUE}${UL}%s${RST}" "$1"; }
hr()   { printf "${DIM}──────────────────────────────────────────────────────────${RST}\n"; }

# --- تشخیص پوشهٔ سورس (اگر کنار اسکریپت worker.js باشد از همان استفاده کن) ---
SRC_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ]; then
  _d="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)"
  if [ -n "$_d" ] && [ -f "$_d/worker.js" ] && [ -f "$_d/deploy-tool.py" ]; then SRC_DIR="$_d"; fi
fi

need_tools() {
  command -v curl >/dev/null 2>&1 || { err "curl نصب نیست."; exit 1; }
  command -v python3 >/dev/null 2>&1 || { err "python3 نصب نیست."; exit 1; }
}

# دانلود فایل از مخزن گیت‌هاب (اول API، بعد raw)
gh_raw() { # owner/repo  path
  curl -fsSL --max-time 60 -H 'Accept: application/vnd.github.raw' \
    "https://api.github.com/repos/$1/contents/$2?ref=${BRANCH}" 2>/dev/null \
  || curl -fsSL --max-time 60 "https://raw.githubusercontent.com/$1/${BRANCH}/$2"
}

fetch_files() {
  need_tools
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  if [ -n "$SRC_DIR" ]; then
    b "استفاده از فایل‌های محلی: $SRC_DIR"
    cp "$SRC_DIR/worker.js" "$DIR/worker.js"
    cp "$SRC_DIR/deploy-tool.py" "$DIR/deploy-tool.py"
  else
    b "دانلود worker.js و deploy-tool.py از گیت‌هاب…"
    gh_raw "$REPO" "worker.js" > "$DIR/worker.js"
    gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"
  fi
  chmod +x "$DIR/deploy-tool.py"
}

verify_token() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" "$API/user/tokens/verify" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print('ok' if d.get('success') and (d.get('result') or {}).get('status')=='active' else 'bad')" 2>/dev/null || echo bad
}

detect_account() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" "$API/accounts?per_page=50" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);r=d.get('result') or [];print(r[0]['id'] if d.get('success') and r else '')" 2>/dev/null || true
}

first_account_name() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" "$API/accounts?per_page=50" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);r=d.get('result') or [];print(r[0].get('name','') if d.get('success') and r else '')" 2>/dev/null || true
}

workers_subdomain() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" "$API/accounts/$2/workers/subdomain" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print((d.get('result') or {}).get('subdomain',''))" 2>/dev/null || true
}

public_ip() {
  local ip=""
  for u in "https://api.ipify.org" "https://ifconfig.me" "https://ipinfo.io/ip"; do
    ip="$(curl -s4 --max-time 8 "$u" 2>/dev/null | tr -d '[:space:]')" && [ -n "$ip" ] && break
  done
  printf '%s' "$ip"
}

read_worker_version() {
  sed -n 's/^const BOT_VERSION = "\([^"]*\)".*/\1/p' "$DIR/worker.js" 2>/dev/null | head -1
}

set_cfg_version() {
  [ -f "$CFG" ] || return 0
  CFG_VER="$1" python3 - "$CFG" <<'PY'
import json, os, sys
p=sys.argv[1]
try: cfg=json.load(open(p))
except Exception: sys.exit(0)
cfg["version"]=os.environ.get("CFG_VER","")
json.dump(cfg, open(p,"w"), ensure_ascii=False, indent=2)
PY
  chmod 600 "$CFG" 2>/dev/null || true
}

load_cfg() {
  CFG_ACCOUNT=""; CFG_WORKER=""; CFG_TOKEN=""; CFG_BOT=""; CFG_ADMIN=""
  [ -f "$CFG" ] || return 0
  eval "$(python3 - "$CFG" <<'PY'
import json,sys
try: c=json.load(open(sys.argv[1]))
except Exception: sys.exit(0)
def q(v): return "'" + str(v).replace("'", "'\\''") + "'"
print("CFG_ACCOUNT="+q(c.get("account_id","")))
print("CFG_WORKER="+q(c.get("worker","")))
print("CFG_TOKEN="+q(c.get("token","")))
print("CFG_BOT="+q(c.get("bot_token","")))
print("CFG_ADMIN="+q(c.get("admin_id","")))
PY
)"
}

write_cfg() {
  [ -f "$CFG" ] && cp "$CFG" "$CFG.bak.$(date +%s)"
  CFG_ACC="$ACC" CFG_WORKER="$WORKER" CFG_TOKEN="$TOKEN" CFG_BOT="$BOT" CFG_ADMIN="$ADMIN" \
  python3 - "$CFG" <<'PY'
import json, os, sys
p=sys.argv[1]
try: cfg=json.load(open(p))
except Exception: cfg={}
cfg["account_id"]=os.environ["CFG_ACC"]
cfg["worker"]=os.environ["CFG_WORKER"]
cfg["token"]=os.environ["CFG_TOKEN"]
cfg["bot_token"]=os.environ["CFG_BOT"]
try: cfg["admin_id"]=int(str(os.environ["CFG_ADMIN"]).split(",")[0])
except Exception: cfg["admin_id"]=0
cfg["cf_accounts"]=[{"name":"main","token":os.environ["CFG_TOKEN"]}]
cfg.setdefault("compatibility_date","2024-11-01")
cfg.setdefault("usage_model","standard")
cfg.setdefault("version","")
json.dump(cfg, open(p,"w"), ensure_ascii=False, indent=2)
PY
  chmod 600 "$CFG"
}

set_webhook() {
  local sub url out
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  if [ -z "$sub" ]; then warn "زیردامنهٔ workers.dev پیدا نشد؛ وبهوک را دستی ست کن."; return 0; fi
  url="https://$WORKER.$sub.workers.dev/tg"
  out="$(curl -sS --max-time 30 "https://api.telegram.org/bot$BOT/setWebhook?url=$url&drop_pending_updates=true" 2>/dev/null || true)"
  if printf '%s' "$out" | grep -q '"ok":true'; then
    ok "وبهوک تلگرام ست شد: $url"
    printf '%s\n' "$url" > "$DIR/webhook_url"
  else
    warn "ست‌کردن وبهوک ناموفق بود: ${out:0:200}"
  fi
}

install_cron_backup() {
  gh_raw "$REPO" "update.sh" > "$DIR/update.sh" 2>/dev/null || true
  chmod +x "$DIR/update.sh" 2>/dev/null || true
  if command -v crontab >/dev/null 2>&1 && [ -f "$DIR/update.sh" ]; then
    ( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ; echo "*/30 * * * * $DIR/update.sh >> $DIR/update.log 2>&1" ) | crontab - 2>/dev/null \
      && ok "کرون پشتیبانِ آپدیت خودکار نصب شد (هر ۳۰ دقیقه)." \
      || warn "نصب کرون ناموفق بود (اختیاری)."
  fi
}

# ─────────────── راهنمای رنگی توکن کلادفلر ───────────────
cf_token_guide() {
  printf '\n'
  printf "${CYAN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${CYAN}${BOLD}  │${RST}  🔑 ${BOLD}ساخت توکن API کلادفلر${RST}                                     ${CYAN}${BOLD}│${RST}\n"
  printf "${CYAN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n"
  printf '\n'
  printf "  ${BOLD}۱)${RST} این لینک را در مرورگر باز کن:\n\n"
  printf "        %s\n\n" "$(link "$CF_TOKENS_URL")"
  printf "  ${BOLD}۲)${RST} دکمهٔ ${GREEN}Create Token${RST} → ${GREEN}Create Custom Token${RST}\n\n"
  printf "  ${BOLD}۳)${RST} این ${YELLOW}۵ دسترسی${RST} را اضافه کن:\n\n"
  printf "       ${GREEN}1)${RST} Account · ${YELLOW}Workers Scripts${RST}     · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}2)${RST} Account · ${YELLOW}Workers KV Storage${RST}  · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}3)${RST} Zone    · ${YELLOW}DNS${RST}                 · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}4)${RST} Zone    · ${YELLOW}Zone Settings${RST}       · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}5)${RST} Zone    · ${YELLOW}Cache Purge${RST}         · ${GREEN}Purge${RST}\n\n"
  printf "     و در پایین:  ${BOLD}Account Resources${RST} = ${GREEN}All accounts${RST}  ·  ${BOLD}Zone Resources${RST} = ${GREEN}All zones${RST}\n\n"
  printf "  ${BOLD}۴)${RST} ${GREEN}Continue → Create Token${RST} و توکن را کپی کن و ${BOLD}اینجا بچسبان${RST}.\n"
  printf "     ${DIM}راهنمای تصویری: %s${RST}\n\n" "$(link "$DOC_TOKEN_URL")"
  hr
}

# ─────────────── نصب ───────────────
do_install() {
  local arg
  for arg in "$@"; do case "$arg" in --no-relay) NO_RELAY=1 ;; esac; done

  printf "\n${MAG}${BOLD}🛡️  نصب نگهبان ابری (Cloud Guardian)${RST}\n"
  hr
  fetch_files

  step "مرحلهٔ ۱ از ۶ — توکن کلادفلر"
  load_cfg
  local TOKEN="${CF_TOKEN:-}" tries=0
  if [ -z "$TOKEN" ] && [ -n "$CFG_TOKEN" ]; then
    read -rp "  توکن ذخیره‌شدهٔ قبلی استفاده شود؟ [Y/n] " a
    [[ "${a,,}" == "n" ]] || TOKEN="$CFG_TOKEN"
  fi
  if [ -z "$TOKEN" ]; then
    cf_token_guide
    while :; do
      read -rp "  🔑 توکن API کلادفلر را اینجا بچسبان: " TOKEN
      [ -n "$TOKEN" ] || { warn "توکن خالی است."; continue; }
      b "بررسی توکن…"
      if [ "$(verify_token "$TOKEN")" = "ok" ]; then ok "توکن معتبر است."; break; fi
      err "توکن نامعتبر است یا دسترسی لازم را ندارد."
      tries=$((tries+1)); [ "$tries" -ge 3 ] && { err "تلاش‌های ناموفق تمام شد."; exit 1; }
    done
  else
    ok "از توکن ذخیره‌شده/محیطی استفاده می‌شود."
  fi

  step "مرحلهٔ ۲ از ۶ — Account ID"
  local ACC="${ACCOUNT_ID:-}"
  if [ -z "$ACC" ]; then
    b "تشخیص خودکار Account ID…"
    ACC="$(detect_account "$TOKEN")"
    if [ -n "$ACC" ]; then
      ok "پیدا شد: $ACC (${CF_DASH_URL}/${ACC})"
    else
      warn "تشخیص خودکار نشد."
      printf "     از داشبورد: %s — ستون راست، باکس API ← Account ID\n" "$(link "$CF_DASH_URL")"
      read -rp "  Account ID را بچسبان: " ACC
    fi
  fi
  [ -n "$ACC" ] || { err "Account ID لازم است."; exit 1; }

  step "مرحلهٔ ۳ از ۶ — نام ورکر"
  local WORKER="${WORKER_NAME:-}"
  if [ -z "$WORKER" ]; then
    read -rp "  نام ورکر [${GREEN}cloud-guardian${RST}]: " WORKER
    WORKER="${WORKER:-cloud-guardian}"
  fi
  [[ "$WORKER" =~ ^[a-zA-Z0-9_-]{1,63}$ ]] || { err "نام ورکر نامعتبر است (حروف/عدد/-/_ تا ۶۳ کاراکتر)."; exit 1; }

  step "مرحلهٔ ۴ از ۶ — ربات تلگرام"
  printf "  توکن ربات را از ${GREEN}@BotFather${RST} بگیر (دستور ${BOLD}/newbot${RST}).\n"
  local BOT="${BOT_TOKEN:-}"
  while :; do
    [ -n "$BOT" ] || read -rp "  🤖 توکن ربات: " BOT
    [[ "$BOT" =~ ^[0-9]{5,}:[A-Za-z0-9_-]{25,}$ ]] && break
    err "فرمت توکن نامعتبر است (باید شبیه 123456:AA... باشد)."; BOT=""
  done

  step "مرحلهٔ ۵ از ۶ — شناسهٔ مدیر"
  printf "  شناسهٔ عددی خودت را از ${GREEN}@userinfobot${RST} بگیر.\n"
  local ADMIN="${ADMIN_ID:-}"
  while :; do
    [ -n "$ADMIN" ] || read -rp "  🆔 شناسهٔ عددی مدیر: " ADMIN
    [[ "$ADMIN" =~ ^[0-9]+$ ]] && break
    err "شناسه باید فقط عدد باشد."; ADMIN=""
  done

  step "مرحلهٔ ۶ از ۶ — ذخیرهٔ تنظیمات"
  write_cfg
  chmod 700 "$DIR" 2>/dev/null || true
  ok "تنظیمات ذخیره شد: $CFG (دسترسی 600 — فقط خودت)"

  step "دیپلوی روی کلادفلر (KV + bindings + worker + cron + workers.dev)"
  if ! ( cd "$DIR" && python3 deploy-tool.py install ); then
    err "دیپلوی ناموفق بود."
    exit 1
  fi
  ok "ورکر با موفقیت دیپلوی شد."
  set_cfg_version "$(read_worker_version)"

  b "ست‌کردن وبهوک تلگرام…"
  set_webhook

  b "نصب آپدیت خودکار (کرون پشتیبان)…"
  install_cron_backup

  local sub url
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  url="https://$WORKER.$sub.workers.dev"

  # رلهٔ SSH (اختیاری)
  if [ -z "${NO_RELAY:-}" ] && [ "$(id -u)" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
    printf '\n'
    read -rp "  🖥 رلهٔ SSH را روی همین سرور نصب کنم؟ (برای پایش/اجرای سرورها) [y/N] " r
    if [[ "${r,,}" == "y" ]]; then do_relay; fi
  fi

  printf '\n'
  printf "${GREEN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${GREEN}${BOLD}  │${RST}  🎉 ${GREEN}${BOLD}نصب کامل شد — نگهبان ابری فعال است${RST}                    ${GREEN}${BOLD}│${RST}\n"
  printf "${GREEN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n"
  printf '\n'
  printf "  • ورکر:      %s\n" "$(link "$url")"
  printf "  • ربات:      از تلگرام به ربات پیام بده و ${BOLD}/start${RST} بزن.\n"
  printf "  • تنظیمات:   %s (محرمانه)\n" "$CFG"
  printf "  • وضعیت:     %s\n" "${BOLD}bash install.sh status${RST}"
  printf "  • حذف:       %s\n" "${BOLD}bash install.sh uninstall${RST}"
  printf "  • آپدیت:     %s\n" "${BOLD}bash install.sh update${RST}"
  printf '\n'
}

# ─────────────── آپدیت ───────────────
do_update() {
  printf "\n${MAG}${BOLD}🔄 آپدیت نگهبان ابری${RST}\n"; hr
  need_tools
  load_cfg
  [ -f "$CFG" ] || { err "اول نصب کن (config.json نیست)."; exit 1; }
  [ -n "$CFG_TOKEN" ] || { err "token در config نیست."; exit 1; }
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  b "دانلود آخرین worker.js از گیت‌هاب…"
  if [ -n "$SRC_DIR" ] && [ -f "$SRC_DIR/worker.js" ]; then
    cp "$SRC_DIR/worker.js" "$DIR/worker.js"
  else
    gh_raw "$REPO" "worker.js" > "$DIR/worker.js"
    gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"
    chmod +x "$DIR/deploy-tool.py"
  fi
  local newv oldv; newv="$(read_worker_version)"; oldv="$(python3 -c "import json;print(json.load(open('$CFG')).get('version',''))" 2>/dev/null || true)"
  b "نسخهٔ فعلی: ${oldv:-?}  →  نسخهٔ جدید: ${newv:-?}"
  if ! ( cd "$DIR" && python3 deploy-tool.py update ); then err "آپدیت ناموفق بود."; exit 1; fi
  set_cfg_version "$newv"
  ok "ورکر به نسخهٔ ${newv:-?} آپدیت شد."
}

# ─────────────── حذف ───────────────
do_uninstall() {
  printf "\n${MAG}${BOLD}🗑️  حذف نگهبان ابری${RST}\n"; hr
  need_tools
  load_cfg
  [ -f "$CFG" ] || { err "config.json پیدا نشد؛ چیزی برای حذف نیست."; exit 1; }
  warn "ورکر «${BOLD}$CFG_WORKER${RST}» از کلادفلر حذف می‌شود (توقف کامل ربات)."
  local a; read -rp "  مطمئنی؟ [y/N] " a
  [[ "${a,,}" == "y" ]] || { echo "لغو شد."; exit 0; }
  local keep_kv=""
  read -rp "  KV namespace هم حذف شود؟ [Y/n] " kv
  [[ "${kv,,}" == "n" ]] && keep_kv="--keep-kv"
  b "حذف ورکر${keep_kv:+ (KV نگه داشته می‌شود)}…"
  ( cd "$DIR" && python3 deploy-tool.py uninstall $keep_kv ) || warn "بعضی مراحل حذف با خطا مواجه شد."
  if command -v crontab >/dev/null 2>&1; then
    ( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ) | crontab - 2>/dev/null && ok "کرون آپدیت خودکار حذف شد."
  fi
  local a2; read -rp "  فایل‌های محلی $DIR هم پاک شوند؟ [Y/n] " a2
  if [[ "${a2,,}" != "n" ]]; then
    rm -rf "$DIR" && ok "فایل‌های محلی پاک شدند."
  else
    warn "فایل‌ها نگه داشته شدند: $DIR"
  fi
  ok "حذف کامل شد."
}

# ─────────────── نصب رله ───────────────
do_relay() {
  printf "\n${MAG}${BOLD}🖥️  نصب رلهٔ SSH (srv-relay)${RST}\n"; hr
  need_tools
  if [ "$(id -u)" -ne 0 ]; then err "برای نصب رله باید با root اجرا کنی:  sudo bash install.sh relay"; exit 1; fi
  command -v systemctl >/dev/null 2>&1 || { err "systemd لازم است."; exit 1; }
  load_cfg

  local PORT="${RELAY_PORT:-8788}"
  local TOK="${SRV_RELAY_TOKEN:-}"
  [ -n "$TOK" ] || TOK="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
  local PUBIP; PUBIP="$(public_ip)"
  [ -n "$PUBIP" ] || PUBIP="<آیپی-عمومی-سرور>"

  b "دانلود اسکریپت نصب رله از مخزن…"
  local TMP; TMP="$(mktemp)"
  gh_raw "$RELAY_REPO" "srv-relay-install.sh" > "$TMP"
  if ! grep -q "srv-relay" "$TMP" 2>/dev/null; then err "دانلود اسکریپت رله ناموفق بود."; rm -f "$TMP"; exit 1; fi
  b "اجرای نصب رله (پورت $PORT)…"
  SRV_RELAY_TOKEN="$TOK" SRV_RELAY_PORT="$PORT" bash "$TMP"
  rm -f "$TMP"
  ok "رله روی این سرور نصب/آپدیت شد."

  printf '\n'
  printf "  • آدرس رله:  %s\n" "$(link "http://$PUBIP:$PORT")"
  printf "  • توکن رله:  %s\n" "${YELLOW}${BOLD}$TOK${RST}"
  printf "  • پورت باید از بیرون باز باشد:  ${BOLD}sudo ufw allow %s/tcp${RST}\n" "$PORT"

  if [ -f "$CFG" ] && [ -n "$CFG_TOKEN" ]; then
    printf '\n'
    read -rp "  🔗 رله خودکار در ربات ثبت شود؟ (یک رکورد A روی زون کلادفلر می‌سازد) [y/N] " a
    if [[ "${a,,}" == "y" ]]; then
      ( cd "$DIR" && SRV_RELAY_TOKEN="$TOK" python3 deploy-tool.py relay-register "$PUBIP" --token "$TOK" --port "$PORT" ) \
        || warn "ثبت خودکار ناموفق؛ دستی از منوی «🖥 سرورها ← 🔧 تنظیم رله» انجام بده."
    fi
  else
    warn "config.json نیست؛ رله را دستی از منوی ربات («🖥 سرورها ← 🔧 تنظیم رله») ثبت کن."
  fi
  printf '\n'
}

# ─────────────── وضعیت و بررسی ───────────────
ensure_tool_only() {
  need_tools
  if [ ! -f "$DIR/deploy-tool.py" ]; then mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true; gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"; chmod +x "$DIR/deploy-tool.py"; fi
  [ -f "$CFG" ] || { err "config.json پیدا نشد؛ اول نصب کن."; exit 1; }
}

do_status() { printf "\n${MAG}${BOLD}📊 وضعیت نگهبان ابری${RST}\n"; hr; ensure_tool_only; ( cd "$DIR" && python3 deploy-tool.py status ); printf '\n'; }

do_check()  { printf "\n${MAG}${BOLD}🔎 بررسی دسترسی‌های توکن کلادفلر${RST}\n"; hr; ensure_tool_only; ( cd "$DIR" && python3 deploy-tool.py check ); printf '\n'; }

usage() {
  printf "\n${MAG}${BOLD}🛡️  نگهبان ابری — راهنما${RST}\n"; hr
  printf "  ${GREEN}install${RST}      نصب/بازنصب کامل روی کلادفلر (پیش‌فرض)\n"
  printf "  ${GREEN}update${RST}       آپدیت ورکر به آخرین نسخهٔ گیت‌هاب\n"
  printf "  ${GREEN}uninstall${RST}    حذف ورکر (و KV و وبهوک)\n"
  printf "  ${GREEN}relay${RST}        نصب رلهٔ SSH روی همین سرور (+ ثبت خودکار)\n"
  printf "  ${GREEN}status${RST}       نمایش وضعیت ورکر، کرون، KV و وبهوک\n"
  printf "  ${GREEN}check${RST}        بررسی دسترسی‌های توکن کلادفلر\n"
  printf "  ${GREEN}help${RST}         همین راهنما\n"
  hr
  printf "  توکن کلادفلر: %s\n" "$(link "$CF_TOKENS_URL")"
  printf "  نصب یک‌خطی:   ${DIM}bash -c \"\$(curl -sL %s/main/install.sh)\"${RST}\n" "https://raw.githubusercontent.com/$REPO"
  printf '\n'
}

# ─────────────── دیسپچ ───────────────
cmd="install"
if [ "$#" -gt 0 ]; then cmd="$1"; shift; fi
case "$cmd" in
  install|i)         do_install "$@" ;;
  update|u|deploy)   do_update ;;
  uninstall|remove|rm) do_uninstall "$@" ;;
  relay|r)           do_relay "$@" ;;
  status|s)          do_status ;;
  check|c)           do_check ;;
  help|-h|--help)    usage ;;
  *) err "دستور نامشخص: $cmd"; usage; exit 1 ;;
esac
