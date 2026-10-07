#!/usr/bin/env bash
# ============================================================
# Cloud Guardian — installer & manager  (bilingual: فارسی / English)
#   new-install | update | uninstall | status | check | help
#   (install/i kept as aliases of new-install)
#
# one-liner:
#   bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install.sh)"
# force language:  CG_LANG=fa|en   or   --lang fa|en
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
BRANCH="main"
DIR="${HOME}/.cloud-guardian"
CFG="${DIR}/config.json"
API="https://api.cloudflare.com/client/v4"
CF_TOKENS_URL="https://dash.cloudflare.com/profile/api-tokens"
CF_DASH_URL="https://dash.cloudflare.com"
DOC_TOKEN_URL="https://github.com/Aknuun/cloud-guardian/blob/main/docs/cloudflare-api-token.md"

# --- colors (off if not a terminal) ---
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

# ============================================================
# i18n — t <key> prints the message in the current language
# ============================================================
t() {
  case "${CG_L:-fa}:$1" in
    en:e_curl)                 printf '%s' "curl is not installed." ;;
    en:e_python)               printf '%s' "python3 is not installed." ;;
    en:e_download)             printf '%s' "Download failed." ;;
    en:e_bad_token)            printf '%s' "Token is invalid or lacks the required permissions." ;;
    en:e_tries)                printf '%s' "Too many failed attempts." ;;
    en:e_worker_name)          printf '%s' "Invalid worker name (letters/digits/-/_, up to 63 chars, must contain at least one letter — a digits-only name looks like a pasted admin ID)." ;;
    en:e_account)              printf '%s' "Account ID is required." ;;
    en:e_bot_token)            printf '%s' "Invalid token format (should look like 123456:AA...)." ;;
    en:e_admin)                printf '%s' "IDs must be numeric, comma-separated (e.g. 12345,67890)." ;;
    en:e_config_missing)       printf '%s' "config.json not found; run install first." ;;
    en:e_token_missing)        printf '%s' "token is missing from config." ;;
    en:e_nothing_remove)       printf '%s' "config.json not found; nothing to remove." ;;
    en:e_deploy)               printf '%s' "Deploy failed." ;;
    en:e_webhook_nosecret)     printf '%s' "tg_secret is missing from KV (incomplete deploy/API error). Refusing to set a secretless webhook." ;;
    en:e_kv_stale)             printf '%s' "The saved KV namespace id is not in this account (stale config). Delete ~/.cloud-guardian/config.json and run install again." ;;
    en:e_update)               printf '%s' "Update failed." ;;
    en:e_update_fix_kv)        printf '%s' "Fix: the saved KV namespace is not in this account (stale config). Delete ~/.cloud-guardian/config.json and run install again." ;;
    en:e_update_fix_auth)      printf '%s' "Fix: the token is invalid or lacks permissions. Make a new token with the 8 required permissions and run install again." ;;
    en:e_update_fix_retry)     printf '%s' "Fix: probably a temporary Cloudflare API error. Run update again in a minute; if it persists, run: python3 deploy-tool.py check" ;;
    en:e_unknown)              printf '%s' "Unknown command:" ;;
    en:e_systemd)              printf '%s' "systemd is required." ;;

    en:w_empty)                printf '%s' "Token is empty." ;;
    en:w_webhook)              printf '%s' "Failed to set webhook:" ;;
    en:w_account_auto)         printf '%s' "Auto-detection failed." ;;
    en:w_kv_delete)            printf '%s' "Failed to delete KV:" ;;
    en:w_partial)              printf '%s' "Some uninstall steps failed." ;;
    en:w_subdomain)            printf '%s' "workers.dev subdomain not found; set the webhook manually." ;;

    en:ok_token)               printf '%s' "Token is valid." ;;
    en:ok_using_token)         printf '%s' "Using the stored/environment token." ;;
    en:ok_account)             printf '%s' "Found:" ;;
    en:ok_config)              printf '%s' "Configuration saved (mode 600 — private)" ;;
    en:ok_deploy)              printf '%s' "Worker deployed successfully." ;;
    en:ok_webhook)             printf '%s' "Telegram webhook set:" ;;
    en:ok_updated)             printf '%s' "Worker updated to version" ;;
    en:ok_uninstalled)         printf '%s' "Uninstall complete." ;;
    en:ok_cron_removed)        printf '%s' "Auto-update cron removed." ;;
    en:ok_files_removed)       printf '%s' "Local files removed." ;;
    en:w_rename_retry)          printf '%s' "Webhook still failing — redeploying under a fresh worker name and retrying automatically:" ;;
    en:w_old_del_fail)          printf '%s' "Could not delete superseded worker (delete it manually to avoid duplicate crons):" ;;
    en:usage_profile)          printf '%s' "Isolated profile for extra bots on one account (--profile NAME | --dir PATH)." ;;
    en:multi_found)            printf '%s' "This Cloudflare account already runs these workers:" ;;
    en:multi_ask)              printf '%s' "Delete any previous workers FIRST? (sensitive — a deleted worker stops at once; its KV data is kept) [y/N] " ;;
    en:multi_pick)             printf '%s' "Numbers (comma-separated), 'all', or empty = keep everything: " ;;
    en:multi_deleted)          printf '%s' "Worker deleted:" ;;
    en:multi_del_fail)         printf '%s' "Could not delete worker (remove it manually if needed):" ;;
    en:multi_kv_kept)          printf '%s' "KV data was kept and can be re-attached later." ;;
    en:multi_exists)           printf '%s' "That worker name already exists — deploying will REPLACE it and kill that bot. Use it anyway? [y/N] " ;;
    en:w_secret_wait)           printf '%s' "Waiting for the worker to mint its secret (warming it up)…" ;;
    en:w_webhook_pending)       printf '%s' "Webhook not set yet — the worker creates its secret on first run and self-sets the webhook within ~1 minute (cron every minute). If the bot stays silent, run: bash install.sh update" ;;

    en:using_local)            printf '%s' "Using local files:" ;;
    en:downloading)            printf '%s' "Downloading worker.js and deploy-tool.py from GitHub…" ;;
    en:step_token)             printf '%s' "Step 1 of 6 — Cloudflare token" ;;
    en:step_account)           printf '%s' "Step 2 of 6 — Account ID" ;;
    en:step_worker)            printf '%s' "Step 3 of 6 — Worker name" ;;
    en:step_bot)               printf '%s' "Step 4 of 6 — Telegram bot" ;;
    en:step_admin)             printf '%s' "Step 5 of 6 — Admin ID" ;;
    en:step_save)              printf '%s' "Step 6 of 6 — Save configuration" ;;
    en:deploy_step)            printf '%s' "Deploying to Cloudflare (KV + bindings + worker + cron + workers.dev)" ;;
    en:verify_token)           printf '%s' "Verifying token…" ;;
    en:detect_account)         printf '%s' "Auto-detecting Account ID…" ;;
    en:acc_hint)               printf '%s' "From the dashboard: right sidebar, API box → Account ID:" ;;
    en:worker_prompt)          printf '%s' "Worker name [cloud-guardian]: " ;;
    en:bot_hint)               printf '%s' "Get the bot token from @BotFather (command /newbot)." ;;
    en:bot_prompt)             printf '%s' "Bot token: " ;;
    en:admin_hint)             printf '%s' "Get your numeric ID from @userinfobot." ;;
    en:admin_prompt)           printf '%s' "Admin IDs (comma-separated, first is primary): " ;;
    en:done_title)             printf '%s' "Install complete — Cloud Guardian is live" ;;
    en:done_worker)            printf '%s' "Worker:" ;;
    en:done_bot)               printf '%s' "Open Telegram, message this bot and press /start:" ;;
    en:done_sending)           printf '%s' "Sending /start to the customer's bot…" ;;
    en:done_tg_msg)            printf '%s' "✅ Cloud Guardian installed. Press /start to see the menu and buttons." ;;
    en:done_cfg)               printf '%s' "Config (secret):" ;;
    en:done_status)            printf '%s' "Status:" ;;
    en:done_remove)            printf '%s' "Uninstall:" ;;
    en:done_update)            printf '%s' "Update:" ;;

    en:tk_box)                 printf '%s' "🔑 Create a Cloudflare API token" ;;
    en:tk_opens)               printf '%s' "Open this link in your browser:" ;;
    en:tk_create)              printf '%s' "Click Create Token → Create Custom Token" ;;
    en:tk_perms)               printf '%s' "Add these 8 permissions (4 required + 4 optional):" ;;
    en:tk_req)                 printf '%s' "REQUIRED (install stops without these):" ;;
    en:tk_opt)                 printf '%s' "OPTIONAL (install succeeds without these):" ;;
    en:tk_res)                 printf '%s' "Then below: Account Resources = All accounts · Zone Resources = All zones" ;;
    en:tk_copy)                printf '%s' "Continue → Create Token, copy the token and paste it here." ;;
    en:tk_visual)              printf '%s' "Visual guide:" ;;
    en:tk_token_prompt)        printf '%s' "Paste the Cloudflare API token here: " ;;

    en:upd_title)              printf '%s' "Updating Cloud Guardian" ;;
    en:upd_download)           printf '%s' "Downloading the latest worker.js from GitHub…" ;;
    en:upd_versions)           printf '%s' "Current: %s  →  New: %s" ;;

    en:un_title)               printf '%s' "Uninstalling Cloud Guardian" ;;
    en:un_warn)                printf '%s' "Worker «%s» will be deleted from Cloudflare (bot stops completely)." ;;
    en:un_confirm)             printf '%s' "Are you sure? [y/N] " ;;
    en:un_canceled)            printf '%s' "Cancelled." ;;
    en:un_keep_kv)             printf '%s' "Delete the KV namespace too? [Y/n] " ;;
    en:un_removing)            printf '%s' "Deleting worker…" ;;
    en:un_files_q)             printf '%s' "Delete the local files too? [Y/n] " ;;
    en:un_files_kept)          printf '%s' "Files kept:" ;;


    en:gd_title)               printf '%s' "Setting up the guardian agent (server-side offload)" ;;
    en:gd_token)               printf '%s' "Agent token:" ;;
    en:gd_oneliner)            printf '%s' "Run this ONE line on the customer server (as root):" ;;
    en:gd_register_q)          printf '%s' "Register the agent token in the bot now? [y/N] " ;;
    en:gd_registered)          printf '%s' "Agent token registered in the bot." ;;
    en:gd_manual)              printf '%s' "Skipped — register later with: python3 deploy-tool.py guardian-register --token ..." ;;
    en:w_guardian_reg)      printf '%s' "Auto-register failed; do it manually with: python3 deploy-tool.py guardian-register --token ..." ;;

    en:st_title)               printf '%s' "Cloud Guardian status" ;;
    en:ck_title)               printf '%s' "Checking Cloudflare token permissions" ;;

    en:usage_title)            printf '%s' "Cloud Guardian — help" ;;
    en:cmd_install)            printf '%s' "New install (fresh install from scratch)" ;;
    en:cmd_update)             printf '%s' "Update" ;;
    en:cmd_uninstall)          printf '%s' "Uninstall" ;;
    en:cmd_guardian)           printf '%s' "Guardian agent (server offload)" ;;
    en:cmd_status)             printf '%s' "Status" ;;
    en:cmd_check)              printf '%s' "Check token" ;;
    en:cmd_help)               printf '%s' "Help" ;;
    en:usage_cf)               printf '%s' "Cloudflare token:" ;;
    en:usage_oneline)          printf '%s' "One-line install:" ;;
    en:step_perms)             printf '%s' "Checking Cloudflare token permissions" ;;
    en:perm_box)               printf '%s' "Cloudflare token permissions are missing or wrong" ;;
    en:perm_body)              printf '%s' "Grant these 8 permissions on your token, 4 required + 4 optional (Account Resources = All accounts · Zone Resources = All zones):" ;;
    en:ok_perms)               printf '%s' "All required permissions are granted." ;;
    en:perm_opt_warn)          printf '%s' "Optional permissions are missing - install continues; those features will need them later." ;;
    en:perm_fix)               printf '%s' "Edit/create the token here:" ;;
    en:perm_abort)             printf '%s' "Install aborted. Fix the token and run again." ;;
    en:perm_continue)          printf '%s' "Token has errors above. Deploy anyway with limited features? [y/N] " ;;
    en:menu_title)             printf '%s' "Cloud Guardian — main menu" ;;
    en:menu_exit)              printf '%s' "Exit" ;;
    en:menu_choose)            printf '%s' "Choose an option: " ;;
    en:menu_invalid)           printf '%s' "Invalid option." ;;
    en:menu_back)              printf '%s' "Press Enter to return to the menu" ;;
    en:offer_install)          printf '%s' "Start a new install now? [y/N] " ;;
    en:auto_deps)              printf '%s' "Installing missing dependencies automatically:" ;;
    en:existing_found)         printf '%s' "Existing install found" ;;
    en:new_install_backup)     printf '%s' "Existing config found — backing it up and starting a fresh new install." ;;
    en:new_install_title)      printf '%s' "New install — fresh setup from scratch (never updates)" ;;
    en:perm_retry)             printf '%s' "Token fixed? Press Enter to retry the permission check (no restart from scratch) — Ctrl+C to abort" ;;
    en:boot_title)             printf '%s' "Setting up Cloudflare credentials" ;;
    en:boot_saved)             printf '%s' "Credentials saved (mode 600 - private)" ;;
    *)                         printf '%s' "$1" ;;
  esac
}

# ============================================================
# Argument parsing:  --lang fa|en  then command
# ============================================================
LANG_ARG=""
PROFILE=""
DIR_OVERRIDE=""
POS=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --lang|-l)   LANG_ARG="${2:-}"; shift 2 2>/dev/null || shift ;;
    --lang=*)    LANG_ARG="${1#*=}"; shift ;;
    --profile|-p) PROFILE="${2:-}"; shift 2 2>/dev/null || shift ;;
    --profile=*) PROFILE="${1#*=}"; shift ;;
    --dir)       DIR_OVERRIDE="${2:-}"; shift 2 2>/dev/null || shift ;;
    --dir=*)     DIR_OVERRIDE="${1#*=}"; shift ;;
    *)           POS+=("$1"); shift ;;
  esac
done
if [ "${#POS[@]}" -gt 0 ]; then set -- "${POS[@]}"; else set --; fi
# Profiles isolate configs for multiple bots on one account:
# --profile shop2 -> ~/.cloud-guardian-shop2 (default profile: ~/.cloud-guardian).
if [ -n "$PROFILE" ]; then
  [[ "$PROFILE" =~ ^[a-zA-Z0-9_-]+$ ]] || { printf 'Bad --profile (letters/digits/-/_ only).\n' >&2; exit 1; }
  DIR="${HOME}/.cloud-guardian-${PROFILE}"
elif [ -n "$DIR_OVERRIDE" ]; then
  DIR="$DIR_OVERRIDE"
fi
CFG="${DIR}/config.json"

set_language() {
  CG_L=en
}
LANG_SEL="${CG_LANG:-$LANG_ARG}"
set_language
export CG_LANG="$CG_L"

# ============================================================
# helpers
# ============================================================
SRC_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ]; then
  _d="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)"
  if [ -n "$_d" ] && [ -f "$_d/worker.js" ] && [ -f "$_d/deploy-tool.py" ]; then SRC_DIR="$_d"; fi
fi

need_tools() {
  command -v curl >/dev/null 2>&1 || { err "$(t e_curl)"; exit 1; }
  command -v python3 >/dev/null 2>&1 || { err "$(t e_python)"; exit 1; }
}

# Install missing dependencies automatically at start, without asking.
# Falls back to need_tools error if installation is impossible (no root/package manager).
ensure_deps() {
  local missing=()
  command -v curl >/dev/null 2>&1 || missing+=("curl")
  command -v python3 >/dev/null 2>&1 || missing+=("python3")
  [ "${#missing[@]}" -eq 0 ] && return 0
  b "$(t auto_deps) ${missing[*]}"
  local SUDO=""
  if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then SUDO="sudo"; fi
  if command -v apt-get >/dev/null 2>&1; then
    $SUDO apt-get update -qq 2>/dev/null || true
    $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${missing[@]}" >/dev/null 2>&1 || true
  elif command -v dnf >/dev/null 2>&1; then
    $SUDO dnf install -y -q "${missing[@]}" >/dev/null 2>&1 || true
  elif command -v yum >/dev/null 2>&1; then
    $SUDO yum install -y -q "${missing[@]}" >/dev/null 2>&1 || true
  elif command -v apk >/dev/null 2>&1; then
    $SUDO apk add --no-cache "${missing[@]}" >/dev/null 2>&1 || true
  fi
  need_tools
}

gh_raw() {
  curl -fsSL --max-time 60 -H 'Accept: application/vnd.github.raw' \
    "https://api.github.com/repos/$1/contents/$2?ref=${BRANCH}" 2>/dev/null \
  || curl -fsSL --max-time 60 "https://raw.githubusercontent.com/$1/${BRANCH}/$2"
}

fetch_files() {
  need_tools
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  if [ -n "$SRC_DIR" ]; then
    b "$(t using_local) $SRC_DIR"
    cp "$SRC_DIR/worker.js" "$DIR/worker.js"
    cp "$SRC_DIR/deploy-tool.py" "$DIR/deploy-tool.py"
  else
    b "$(t downloading)"
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
workers_subdomain() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" "$API/accounts/$2/workers/subdomain" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print((d.get('result') or {}).get('subdomain',''))" 2>/dev/null || true
}
bot_username() {
  curl -sS --max-time 15 "https://api.telegram.org/bot$1/getMe" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print((d.get('result') or {}).get('username',''))" 2>/dev/null || true
}
# شبیه‌سازی پیام /start از طرف مدیر و فرستادن آن به وب‌هوک ربات
trigger_start() {
  local url="$1" admin="$2" epoch
  epoch="$(date +%s)"
  curl -sS --max-time 20 -X POST "$url" -H 'content-type: application/json' \
    -d "{\"update_id\":1,\"message\":{\"message_id\":1,\"date\":$epoch,\"chat\":{\"id\":$admin,\"type\":\"private\"},\"from\":{\"id\":$admin,\"is_bot\":false,\"first_name\":\"admin\"},\"text\":\"/start\"}}" \
    >/dev/null 2>&1 || true
}
# پیام متنی مستقیم از طرف ربات به مدیر
tg_send_text() {
  local tok="$1" chat="$2" text="$3" payload
  payload="$(CHAT="$chat" TEXT="$text" python3 -c 'import json,os; print(json.dumps({"chat_id": os.environ["CHAT"], "text": os.environ["TEXT"]}, ensure_ascii=False))' 2>/dev/null || true)"
  [ -n "$payload" ] || return 0
  curl -sS --max-time 20 -X POST "https://api.telegram.org/bot$tok/sendMessage" \
    -H 'content-type: application/json' -d "$payload" >/dev/null 2>&1 || true
}
public_ip() {
  local ip=""
  for u in "https://api.ipify.org" "https://ifconfig.me" "https://ipinfo.io/ip"; do
    ip="$(curl -s4 --max-time 8 "$u" 2>/dev/null | tr -d '[:space:]')" && [ -n "$ip" ] && break
  done
  printf '%s' "$ip"
}
read_worker_version() { sed -n 's/^const BOT_VERSION = "\([^"]*\)".*/\1/p' "$DIR/worker.js" 2>/dev/null | head -1; }
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
# Read a raw value from the worker KV (empty string on any failure).
kv_get() {
  python3 - "$TOKEN" "$ACC" "$CFG" "$1" <<'PYINNER' 2>/dev/null || true
import json, sys, urllib.request
tok, acc, cfgp, key = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
try:
    cfg = json.load(open(cfgp))
    kvid = cfg.get("kv_namespace_id") or ""
    if not kvid:
        print(""); raise SystemExit
    req = urllib.request.Request(
        "https://api.cloudflare.com/client/v4/accounts/" + acc + "/storage/kv/namespaces/" + kvid + "/values/" + key,
        headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req, timeout=20) as r:
        print(r.read().decode().strip())
except Exception:
    print("")
PYINNER
}
set_webhook() {
  # Returns 0 only when Telegram confirms the webhook WITH the worker's tg_secret.
  # A secretless webhook makes the worker drop every update, so it is refused.
  local sub url out secret waited
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  if [ -z "$sub" ]; then err "$(t w_subdomain)"; return 1; fi
  url="https://$WORKER.$sub.workers.dev/tg"
  # Fresh deploys have no tg_secret yet — the worker mints it on first run.
  # Warm it (GET plus a harmless POST that reaches the secret-minting path),
  # then poll KV briefly before giving up. Progress is printed so the
  # installer never looks stuck.
  curl -sS --max-time 20 "$url" >/dev/null 2>&1 || true
  curl -sS --max-time 20 -X POST "$url" -H 'Content-Type: application/json' --data '{}' >/dev/null 2>&1 || true
  secret="$(kv_get tg_secret)"
  if [ -z "$secret" ]; then
    b "$(t w_secret_wait)"
  fi
  waited=0
  while [ -z "$secret" ] && [ "$waited" -lt 60 ]; do
    sleep 5; waited=$((waited+5))
    secret="$(kv_get tg_secret)"
  done
  # rc 2: secret never minted — a rename cannot fix this, so the caller
  # must NOT auto-rename on it (only warn).
  if [ -z "$secret" ]; then err "$(t e_webhook_nosecret)"; return 2; fi
  # Telegram sometimes can't resolve a freshly-deployed workers.dev host
  # on the first try (its own DNS cache) — retry with backoff instead of
  # failing a healthy deploy on a transient resolver error.
  local tries=0
  while [ "$tries" -lt 4 ]; do
    out="$(curl -sS --max-time 30 "https://api.telegram.org/bot$BOT/setWebhook?url=$url&drop_pending_updates=true&secret_token=$secret" 2>/dev/null || true)"
    if printf '%s' "$out" | grep -q '"ok":true'; then
      ok "$(t ok_webhook) $url"
      printf '%s\n' "$url" > "$DIR/webhook_url"
      return 0
    fi
    tries=$((tries+1))
    [ "$tries" -lt 4 ] && sleep 15
  done
  err "$(t w_webhook) ${out:0:200}"
  return 1
}
_tk_guide_block() {
  printf "${CYAN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${CYAN}${BOLD}  │${RST}  %s\n" "$(t tk_box)"
  printf "${CYAN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n"
  printf "  ${BOLD}1)${RST} %s\n" "$(t tk_opens)"
  printf "        %s\n" "$(link "$CF_TOKENS_URL")"
  printf "  ${BOLD}2)${RST} %s\n" "$(t tk_create)"
  printf "  ${BOLD}3)${RST} %s\n" "$(t tk_perms)"
  printf "       ${BOLD}%s${RST}\n" "$(t tk_req)"
  printf "       ${GREEN}1)${RST} Account · ${YELLOW}Workers Scripts${RST}     · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}2)${RST} Account · ${YELLOW}Workers KV Storage${RST}  · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}3)${RST} Zone    · ${YELLOW}DNS${RST}                 · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}4)${RST} Zone    · ${YELLOW}Zone Settings${RST}       · ${GREEN}Edit${RST}
"
  printf "       ${BOLD}%s${RST}\n" "$(t tk_opt)"
  printf "       ${GREEN}5)${RST} Zone    · ${YELLOW}Email Routing Rules${RST} · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}6)${RST} Account · ${YELLOW}Email Routing Addresses${RST} · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}7)${RST} Zone    · ${YELLOW}Analytics${RST}           · ${GREEN}Read${RST}
"
  printf "       ${GREEN}8)${RST} Account · ${YELLOW}Account Analytics${RST}   · ${GREEN}Read${RST}
"
  printf "  ${BOLD}4)${RST} %s\n" "$(t tk_copy)"
}
cf_token_guide() {
  CG_L=en; _tk_guide_block
  hr
}

# ─────────────── خطای رنگيِ دسترسی توکن کلادفلر ───────────────
cf_perm_error() {
  printf '\n'
  printf "${RED}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${RED}${BOLD}  │${RST}  ⛔ ${BOLD}%s${RST}\n" "$(t perm_box)"
  printf "${RED}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n\n"
  printf "  %s\n\n" "$(t perm_body)"
  printf "       ${BOLD}%s${RST}\n" "$(t tk_req)"
  printf "       ${GREEN}1)${RST} Account · ${YELLOW}Workers Scripts${RST}     · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}2)${RST} Account · ${YELLOW}Workers KV Storage${RST}  · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}3)${RST} Zone    · ${YELLOW}DNS${RST}                 · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}4)${RST} Zone    · ${YELLOW}Zone Settings${RST}       · ${GREEN}Edit${RST}
"
  printf "       ${BOLD}%s${RST}\n" "$(t tk_opt)"
  printf "       ${GREEN}5)${RST} Zone    · ${YELLOW}Email Routing Rules${RST} · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}6)${RST} Account · ${YELLOW}Email Routing Addresses${RST} · ${GREEN}Edit${RST}
"
  printf "       ${GREEN}7)${RST} Zone    · ${YELLOW}Analytics${RST}           · ${GREEN}Read${RST}
"
  printf "       ${GREEN}8)${RST} Account · ${YELLOW}Account Analytics${RST}   · ${GREEN}Read${RST}
"
  printf "  %s %s\n\n" "$(t perm_fix)" "$(link "$CF_TOKENS_URL")"
}

# Next free worker name for auto-heal renames: goooo -> goooo2 -> goooo3.
next_worker_name() {
  if [[ "$1" =~ ^(.*[^0-9])([0-9]+)$ ]]; then
    printf '%s%s' "${BASH_REMATCH[1]}" "$((BASH_REMATCH[2]+1))"
  else
    printf '%s2' "$1"
  fi
}
# Worker names on an account, one per line (empty on any failure).
account_workers() {
  curl -sS --max-time 30 -H "Authorization: Bearer $1" \
    "$API/accounts/$2/workers/scripts" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print(chr(10).join([str((w or {}).get('id') or '') for w in (d.get('result') or []) if (w or {}).get('id')]))" 2>/dev/null || true
}
# Best-effort removal of a superseded worker (kills its duplicate crons).
# Caller warns on failure; uses do_install/update TOKEN+ACC via dynamic scope.
delete_worker_script() {
  curl -sS --max-time 30 -X DELETE -H "Authorization: Bearer $TOKEN" \
    "$API/accounts/$ACC/workers/scripts/$1" >/dev/null 2>&1
}
# ============================================================
# install
# ============================================================
do_install() {
  local arg
  for arg in "$@"; do case "$arg" in --force|-y) FORCE=1 ;; esac; done

  # NEW INSTALL contract: always a fresh install from scratch.
  # It never auto-updates, never reuses the old flow — existing config
  # is backed up (config.json.bak.<timestamp>) and setup starts at step 1.
  if [ -f "$CFG" ] && [ -z "${FORCE:-}" ]; then
    cp "$CFG" "$CFG.bak.$(date +%s)" 2>/dev/null || true
    warn "$(t new_install_backup): $CFG"
  fi

  printf "\n${MAG}${BOLD}🛡️  %s${RST}\n" "$(t new_install_title)"
  hr
  fetch_files

  step "$(t step_token)"
  load_cfg
  # NEW INSTALL: never reuse the stored token — always start clean.
  # (Automation can still inject via CF_TOKEN env.)
  local TOKEN="${CF_TOKEN:-}" tries=0
  if [ -z "$TOKEN" ]; then
    cf_token_guide
    while :; do
      read -rp "  🔑 $(t tk_token_prompt)" TOKEN
      [ -n "$TOKEN" ] || { warn "$(t w_empty)"; continue; }
      b "$(t verify_token)"
      if [ "$(verify_token "$TOKEN")" = "ok" ]; then ok "$(t ok_token)"; break; fi
      err "$(t e_bad_token)"
      tries=$((tries+1)); [ "$tries" -ge 3 ] && { err "$(t e_tries)"; exit 1; }
    done
  else
    ok "$(t ok_using_token)"
  fi

  step "$(t step_account)"
  local ACC="${ACCOUNT_ID:-}"
  if [ -z "$ACC" ]; then
    b "$(t detect_account)"
    ACC="$(detect_account "$TOKEN")"
    if [ -n "$ACC" ]; then
      ok "$(t ok_account) $ACC (${CF_DASH_URL}/${ACC})"
    else
      warn "$(t w_account_auto)"
      printf "     %s %s\n" "$(t acc_hint)" "$(link "$CF_DASH_URL")"
      read -rp "  Account ID: " ACC
    fi
  fi
  [ -n "$ACC" ] || { err "$(t e_account)"; exit 1; }

  # MULTI-BOT: other workers may already live on this account. List them
  # and offer (in RED — deletion is sensitive) to remove previous ones
  # first. KV data of deleted workers is kept.
  EXISTING="$(account_workers "$TOKEN" "$ACC")"
  if [ -n "$EXISTING" ]; then
    b "$(t multi_found)"
    i=0
    while IFS= read -r _w; do
      [ -n "$_w" ] || continue
      i=$((i+1))
      printf "    ${GREEN}%s)${RST} %s\n" "$i" "$_w"
    done <<< "$EXISTING"
    read -rp "  ${RED}$(t multi_ask)${RST}" _ans || _ans=""
    if [[ "${_ans,,}" == "y" ]]; then
      read -rp "  $(t multi_pick)" _sel || _sel=""
      _del=""
      if [ "${_sel,,}" = "all" ]; then
        _del="$EXISTING"
      elif [ -n "$_sel" ]; then
        while IFS= read -r _n; do
          _n="$(printf '%s' "$_n" | tr -d ' ')"
          [[ "$_n" =~ ^[0-9]+$ ]] && [ "$_n" -ge 1 ] && [ "$_n" -le "$i" ] || continue
          _name="$(printf '%s\n' "$EXISTING" | sed -n "${_n}p")"
          [ -n "$_name" ] && _del="${_del}${_del:+$'\n'}${_name}"
        done <<< "$(printf '%s' "$_sel" | tr ',' '\n')"
      fi
      while IFS= read -r _d; do
        [ -n "$_d" ] || continue
        if delete_worker_script "$_d"; then
          ok "$(t multi_deleted) $_d"
        else
          warn "$(t multi_del_fail) $_d"
        fi
      done <<< "$_del"
      b "$(t multi_kv_kept)"
      EXISTING="$(account_workers "$TOKEN" "$ACC")"
    fi
  fi

  step "$(t step_worker)"
  # Profile installs default to a profiled worker name so a second bot
  # never silently overwrites the first one.
  DEFW="cloud-guardian"
  [ -n "${PROFILE:-}" ] && DEFW="cloud-guardian-${PROFILE}"
  local WORKER="${WORKER_NAME:-}"
  if [ -z "$WORKER" ]; then
    read -rp "  $(t worker_prompt)[${DEFW}]: " WORKER
    WORKER="${WORKER:-$DEFW}"
  fi
  [[ "$WORKER" =~ ^[a-zA-Z0-9_-]{1,63}$ && "$WORKER" =~ [a-zA-Z] ]] || { err "$(t e_worker_name)"; exit 1; }
  # Collision guard: deploying over an existing name REPLACES that worker
  # (kills the other bot). Ask explicitly, in red like deletions.
  if printf '%s\n' "$EXISTING" | grep -qx "$WORKER"; then
    read -rp "  ${RED}$(t multi_exists)${RST}" _yn || exit 1
    if [[ "${_yn,,}" != "y" ]]; then
      err "$(t e_worker_name)"; exit 1
    fi
  fi

  step "$(t step_bot)"
  printf "  %s\n" "$(t bot_hint)"
  local BOT="${BOT_TOKEN:-}"
  while :; do
    [ -n "$BOT" ] || read -rp "  🤖 $(t bot_prompt)" BOT
    [[ "$BOT" =~ ^[0-9]{5,}:[A-Za-z0-9_-]{25,}$ ]] && break
    err "$(t e_bot_token)"; BOT=""
  done

  step "$(t step_admin)"
  printf "  %s\n" "$(t admin_hint)"
  local ADMIN="${ADMIN_ID:-}"
  while :; do
    [ -n "$ADMIN" ] || read -rp "  🆔 $(t admin_prompt)" ADMIN
    ADMIN="$(printf '%s' "$ADMIN" | tr -d ' ')"
    [[ "$ADMIN" =~ ^[0-9]+(,[0-9]+)*$ ]] && break
    err "$(t e_admin)"; ADMIN=""
  done

  step "$(t step_save)"
  write_cfg
  chmod 700 "$DIR" 2>/dev/null || true
  ok "$(t ok_config): $CFG"

  step "$(t step_perms)"
  # Standard recovery: on required-permission failure, stay on this step
  # and offer Enter-to-retry (no restart from step 1). --force keeps the
  # old non-interactive behaviour (warn and continue).
  while :; do
    set +e
    ( cd "$DIR" && CG_LANG="$CG_L" python3 deploy-tool.py check --no-box )
    rc=$?
    set -e
    if [ "$rc" -eq 0 ]; then
      ok "$(t ok_perms)"; break
    elif [ "$rc" -eq 2 ]; then
      warn "$(t perm_opt_warn)"
      if [ -n "${FORCE:-}" ]; then break; fi
      read -rp "  $(t perm_continue)" yn || exit 1
      if [[ "${yn,,}" == "y" ]]; then break; fi
      read -rp "  ⏎ $(t perm_retry): " _ || exit 1
    else
      cf_perm_error
      if [ -n "${FORCE:-}" ]; then
        warn "$(t perm_abort)"; break
      fi
      err "$(t perm_abort)"
      read -rp "  ⏎ $(t perm_retry): " _ || exit 1
    fi
  done

  step "$(t deploy_step)"
  # Standard recovery: retry deploy in place (no restart). Webhook is
  # NEVER set on a failed/incomplete deploy — set_webhook runs only
  # after a successful install below (it also refuses secretless webhooks).
  while :; do
    dout=""
    if dout=$(cd "$DIR" && python3 deploy-tool.py install 2>&1); then
      break
    fi
    printf "%s\n" "$dout"
    if printf "%s" "$dout" | grep -q "10041"; then
      err "$(t e_kv_stale)"
    else
      cf_perm_error
    fi
    err "$(t e_deploy)"
    if [ -n "${FORCE:-}" ]; then exit 1; fi
    read -rp "  ⏎ $(t perm_retry): " _ || exit 1
  done
  ok "$(t ok_deploy)"
  set_cfg_version "$(read_worker_version)"

  b "$(t ok_webhook)"
  # AUTO-HEAL (no AI needed): Telegram sometimes cannot resolve a freshly
  # deployed workers.dev host (its own negative DNS cache). The automatic
  # fix is a fresh hostname — redeploy under an incremented worker name
  # and retry the webhook, bounded attempts. Superseded workers are
  # deleted so no duplicate crons keep running. Deploy already succeeded,
  # so a final miss only warns (worker self-heals hourly; `update` too).
  # A secretless webhook is still never set (see set_webhook).
  wh_tries=0
  wh_rc=0
  while ! set_webhook; do
    wh_rc=$?
    if [ "$wh_rc" -eq 2 ]; then
      warn "$(t w_webhook_pending)"
      break
    fi
    wh_tries=$((wh_tries+1))
    if [ "$wh_tries" -ge 3 ]; then
      warn "$(t w_webhook_pending)"
      break
    fi
    OLD_W="$WORKER"
    WORKER="$(next_worker_name "$WORKER")"
    warn "$(t w_rename_retry) $OLD_W → $WORKER"
    write_cfg
    if ! dout=$(cd "$DIR" && python3 deploy-tool.py install 2>&1); then
      printf "%s\n" "$dout"
      err "$(t e_deploy)"
      exit 1
    fi
    ok "$(t ok_deploy)"
    set_cfg_version "$(read_worker_version)"
    delete_worker_script "$OLD_W" || warn "$(t w_old_del_fail) $OLD_W"
  done

  local sub url
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  url="https://$WORKER.$sub.workers.dev"

  b "$(t done_sending)"
  tg_send_text "$BOT" "${ADMIN%%,*}" "$(t done_tg_msg)"
  trigger_start "$url/tg" "${ADMIN%%,*}"

  # ایجنت سرور: قلب تپندهٔ نصب تازه — بدون آن همهٔ کارها روی ورکر می‌ماند و سقف‌ها برمی‌گردند
  do_guardian_agent "$url"

  printf '\n'
  printf "${GREEN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${GREEN}${BOLD}  │${RST}  🎉 ${GREEN}${BOLD}%s${RST}\n" "$(t done_title)"
  printf "${GREEN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n\n"
  printf "  • %s %s\n" "$(t done_worker)" "$(link "$url")"
  local BU; BU="$(bot_username "$BOT")"
  if [ -n "$BU" ]; then
    printf "  • %s @%s\n" "$(t done_bot)" "$(link "$BU")"
  else
    printf "  • %s\n" "$(t done_bot)"
  fi
  printf "  • %s %s\n" "$(t done_cfg)" "$CFG"
  printf "  • %s bash install.sh status\n" "$(t done_status)"
  printf "  • %s bash install.sh uninstall\n" "$(t done_remove)"
  printf "  • %s bash install.sh update\n\n" "$(t done_update)"
}

# ============================================================
# update
# ============================================================
do_update() {
  printf "\n${MAG}${BOLD}🔄 %s${RST}\n" "$(t upd_title)"; hr
  need_tools
  ensure_ctx full || return 0
  [ -n "$CFG_TOKEN" ] || { err "$(t e_token_missing)"; exit 1; }
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  b "$(t upd_download)"
  if [ -n "$SRC_DIR" ] && [ -f "$SRC_DIR/worker.js" ]; then
    cp "$SRC_DIR/worker.js" "$DIR/worker.js"
    # ابزار دیپلوی را هم هم‌نسخه کن؛ وگرنه نسخهٔ قدیمیِ deploy-tool بایندینگ‌ها را
    # خالی دیپلوی می‌کند و ربات می‌میرد.
    if [ -f "$SRC_DIR/deploy-tool.py" ]; then
      cp "$SRC_DIR/deploy-tool.py" "$DIR/deploy-tool.py"
      chmod +x "$DIR/deploy-tool.py" 2>/dev/null || true
    fi
    if [ -f "$SRC_DIR/update.sh" ]; then
      cp "$SRC_DIR/update.sh" "$DIR/update.sh"
      chmod +x "$DIR/update.sh" 2>/dev/null || true
    fi
  else
    gh_raw "$REPO" "worker.js" > "$DIR/worker.js"
    gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"
    chmod +x "$DIR/deploy-tool.py"
  fi
  local newv oldv
  newv="$(read_worker_version)"
  oldv="$(python3 -c "import json;print(json.load(open('$CFG')).get('version',''))" 2>/dev/null || true)"
  printf "$(t upd_versions)\n" "${oldv:-?}" "${newv:-?}"
  local upd_out
  if ! upd_out=$(cd "$DIR" && python3 deploy-tool.py update 2>&1); then
    printf "%s\n" "$upd_out"
    err "$(t e_update)"
    if printf "%s" "$upd_out" | grep -q "10041"; then
      err "$(t e_update_fix_kv)"
    elif printf "%s" "$upd_out" | grep -qiE "unauthor|401|10000|token|permission"; then
      err "$(t e_update_fix_auth)"
    else
      err "$(t e_update_fix_retry)"
    fi
    exit 1
  fi
  # ترمیم وبهوک بعد از هر آپدیت موفق: اگر نصب قبلی با وبهوک بی‌secret (دیپلوی ناقص)
  # گیر کرده بود، الان با secret درست بازسازی می‌شود. خرابی‌اش کشنده نیست چون
  # وبهوک سالم قبلی سر جایش می‌ماند.
  set_webhook || warn "$(t w_webhook) (update continues; old webhook untouched)"
  set_cfg_version "$newv"
  ok "$(t ok_updated) ${newv:-?}."
}

# ============================================================
# uninstall
# ============================================================
do_uninstall() {
  printf "\n${MAG}${BOLD}🗑️  %s${RST}\n" "$(t un_title)"; hr
  need_tools
  ensure_ctx full || return 0
  warn "$(printf "$(t un_warn)" "$CFG_WORKER")"
  local a; read -rp "  $(t un_confirm)" a
  [[ "${a,,}" == "y" ]] || { echo "$(t un_canceled)"; exit 0; }
  local keep_kv=""
  read -rp "  $(t un_keep_kv)" kv
  [[ "${kv,,}" == "n" ]] && keep_kv="--keep-kv"
  b "$(t un_removing)"
  ( cd "$DIR" && python3 deploy-tool.py uninstall $keep_kv ) || warn "$(t w_partial)"
  if command -v crontab >/dev/null 2>&1; then
    ( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ) | crontab - 2>/dev/null && ok "$(t ok_cron_removed)"
  fi
  local a2; read -rp "  $(t un_files_q)" a2
  if [[ "${a2,,}" != "n" ]]; then rm -rf "$DIR" && ok "$(t ok_files_removed)"; else warn "$(t un_files_kept) $DIR"; fi
  ok "$(t ok_uninstalled)"
}

# ============================================================
# guardian agent
# ============================================================

do_guardian_agent() {
  # نصب تازه و مستقل: توکن می‌سازد، دستور یک‌خطی سرور مشتری را چاپ می‌کند و
  # (با تأیید) توکن را در KV ورکر ثبت می‌کند تا heartbeat ایجنت معتبر باشد.
  # آرگومان اول اختیاری: آدرس عمومی ورکر (در do_install موجود است).
  local WORKER_URL_IN="${1:-}"
  printf "\n${MAG}${BOLD}🖥️  %s${RST}\n" "$(t gd_title)"; hr
  local TOK="${GUARDIAN_TOKEN:-}"
  [ -n "$TOK" ] || TOK="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
  local WURL="$WORKER_URL_IN"
  [ -n "$WURL" ] || WURL="https://YOUR-WORKER.workers.dev"
  printf '\n'
  printf "  • %s\n" "$(t gd_oneliner)"
  printf "    ${YELLOW}${BOLD}sudo GUARDIAN_TOKEN=\"%s\" WORKER_URL=\"%s\" bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent-install.sh)\"${RST}\n" "$TOK" "$WURL"
  printf "  • %s ${YELLOW}${BOLD}%s${RST}\n" "$(t gd_token)" "$TOK"
  printf '\n'
  local reg="n"
  if [ -f "$CFG" ] && [ -n "$CFG_TOKEN" ]; then
    if [ -t 0 ]; then
      read -rp "  🔗 $(t gd_register_q)" reg || reg="n"
    fi
    if [[ "${reg,,}" == "y" ]]; then
      ( cd "$DIR" && python3 deploy-tool.py guardian-register --token "$TOK" ) \
        && ok "$(t gd_registered)" \
        || warn "$(t w_guardian_reg)"
    else
      printf "  • %s\n" "$(t gd_manual)"
    fi
  else
    printf "  • %s\n" "$(t gd_manual)"
  fi
  printf '\n'
}

# ============================================================
# status / check
# ============================================================
ensure_tool_only() {
  need_tools
  if [ ! -f "$DIR/deploy-tool.py" ]; then
    mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
    gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"; chmod +x "$DIR/deploy-tool.py"
  fi
  return 0
}
# Standard credential bootstrap: when no usable local config exists, ask for the
# Cloudflare token up front (like full install does), auto-detect the account,
# ask the worker name (+ bot/admin for deploy commands) and save config.json —
# so update/uninstall/status/check work without a prior install.
# Mode "basic" stops after the worker name (enough for status/check).
bootstrap_creds() {
  local mode="${1:-full}" tries=0
  printf "\n${MAG}${BOLD}🔑 %s${RST}\n" "$(t boot_title)"; hr
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  load_cfg
  TOKEN="${CF_TOKEN:-$CFG_TOKEN}"
  if [ -z "$TOKEN" ]; then
    cf_token_guide
    while :; do
      read -rp "  🔑 $(t tk_token_prompt)" TOKEN || TOKEN=""
      [ -n "$TOKEN" ] || { warn "$(t w_empty)"; continue; }
      b "$(t verify_token)"
      if [ "$(verify_token "$TOKEN")" = "ok" ]; then ok "$(t ok_token)"; break; fi
      err "$(t e_bad_token)"
      tries=$((tries+1)); [ "$tries" -ge 3 ] && { err "$(t e_tries)"; return 1; }
    done
  else
    ok "$(t ok_using_token)"
  fi
  ACC="${ACCOUNT_ID:-$CFG_ACCOUNT}"
  if [ -z "$ACC" ]; then
    b "$(t detect_account)"
    ACC="$(detect_account "$TOKEN")"
    if [ -n "$ACC" ]; then
      ok "$(t ok_account) $ACC (${CF_DASH_URL}/${ACC})"
    else
      warn "$(t w_account_auto)"
      printf "     %s %s\n" "$(t acc_hint)" "$(link "$CF_DASH_URL")"
      read -rp "  Account ID: " ACC || ACC=""
    fi
  fi
  [ -n "$ACC" ] || { err "$(t e_account)"; return 1; }
  WORKER="${WORKER_NAME:-${CFG_WORKER:-}}"
  if [ -z "$WORKER" ]; then
    read -rp "  $(t worker_prompt)" WORKER || WORKER=""
    WORKER="${WORKER:-cloud-guardian}"
  fi
  [[ "$WORKER" =~ ^[a-zA-Z0-9_-]{1,63}$ && "$WORKER" =~ [a-zA-Z] ]] || { err "$(t e_worker_name)"; return 1; }
  BOT="${BOT_TOKEN:-$CFG_BOT}"
  ADMIN="${ADMIN_ID:-$CFG_ADMIN}"
  if [ "$mode" = "full" ]; then
    if [ -z "$BOT" ]; then
      printf "  %s\n" "$(t bot_hint)"
      while :; do
        read -rp "  🤖 $(t bot_prompt)" BOT || BOT=""
        [[ "$BOT" =~ ^[0-9]{5,}:[A-Za-z0-9_-]{25,}$ ]] && break
        err "$(t e_bot_token)"; BOT=""
      done
    fi
    ADMIN="$(printf '%s' "${ADMIN:-}" | tr -d ' ')"
    if [[ ! "$ADMIN" =~ ^[0-9]+(,[0-9]+)*$ ]] || [ "${ADMIN%%,*}" = "0" ]; then
      ADMIN=""
      printf "  %s\n" "$(t admin_hint)"
      while :; do
        read -rp "  🆔 $(t admin_prompt)" ADMIN || ADMIN=""
        ADMIN="$(printf '%s' "$ADMIN" | tr -d ' ')"
        [[ "$ADMIN" =~ ^[0-9]+(,[0-9]+)*$ ]] && break
        err "$(t e_admin)"; ADMIN=""
      done
    fi
  fi
  write_cfg
  chmod 700 "$DIR" 2>/dev/null || true
  ok "$(t boot_saved): $CFG"
}
ctx_complete() {
  [ -n "$CFG_TOKEN" ] && [ -n "$CFG_ACCOUNT" ] && [ -n "$CFG_WORKER" ] || return 1
  [ "$1" = "basic" ] && return 0
  [ -n "$CFG_BOT" ] || return 1
  [[ "$CFG_ADMIN" =~ ^[0-9]+(,[0-9]+)*$ ]] && [ "${CFG_ADMIN%%,*}" != "0" ] || return 1
  return 0
}
# Ensure a usable config for management commands: stored config first, then a
# silent fill from env (automation friendly), then interactive bootstrap.
ensure_ctx() {
  local mode="${1:-full}"
  load_cfg
  if ctx_complete "$mode"; then return 0; fi
  if [ -n "${CF_TOKEN:-}" ] && [ -n "${ACCOUNT_ID:-}" ] && [ -n "${WORKER_NAME:-}" ]; then
    if [ "$mode" = "basic" ] || { [ -n "${BOT_TOKEN:-}" ] && [ -n "${ADMIN_ID:-}" ]; }; then
      TOKEN="$CF_TOKEN" ACC="$ACCOUNT_ID" WORKER="$WORKER_NAME"
      BOT="${BOT_TOKEN:-}" ADMIN="${ADMIN_ID:-0}"
      mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
      write_cfg
      load_cfg
      if ctx_complete "$mode"; then return 0; fi
    fi
  fi
  if [ -t 0 ]; then
    bootstrap_creds "$mode" || return 1
    load_cfg
    return 0
  fi
  warn "$(t e_config_missing)"
  return 1
}
do_status() { printf "\n${MAG}${BOLD}📊 %s${RST}\n" "$(t st_title)"; hr; ensure_tool_only; ensure_ctx basic || return 0; ( cd "$DIR" && python3 deploy-tool.py status ); printf '\n'; }
do_check()  { printf "\n${MAG}${BOLD}🔎 %s${RST}\n" "$(t ck_title)"; hr; ensure_tool_only; ensure_ctx basic || return 0; ( cd "$DIR" && python3 deploy-tool.py check ) || true; printf '\n'; }

usage() {
  printf "\n${MAG}${BOLD}🛡️  %s${RST}\n" "$(t usage_title)"; hr
  printf "  ${GREEN}new-install${RST}  %s\n" "$(t cmd_install)"
  printf "  ${GREEN}update${RST}       %s\n" "$(t cmd_update)"
  printf "  ${GREEN}uninstall${RST}    %s\n" "$(t cmd_uninstall)"
  printf "  ${GREEN}guardian${RST}     %s\n" "$(t cmd_guardian)"
  printf "  ${GREEN}status${RST}       %s\n" "$(t cmd_status)"
  printf "  ${GREEN}check${RST}        %s\n" "$(t cmd_check)"
  printf "  ${GREEN}help${RST}         %s\n" "$(t cmd_help)"
  hr
  printf "  %s %s\n" "$(t usage_cf)" "$(link "$CF_TOKENS_URL")"
  printf "  %s ${DIM}bash -c \"\$(curl -sL https://raw.githubusercontent.com/$REPO/main/install.sh)\"${RST}\n" "$(t usage_oneline)"
  printf "  %s\n" "$(t usage_profile)"
  printf '\n'
}

# ============================================================
# main menu
# ============================================================
main_menu() {
  while :; do
    printf '\n'
    printf "${CYAN}${BOLD}  ╭──────────────────────────────────────────────────────────╮${RST}\n"
    printf "${CYAN}${BOLD}  │${RST}  🛡️  ${BOLD}%s${RST}\n" "$(t menu_title)"
    printf "${CYAN}${BOLD}  ╰──────────────────────────────────────────────────────────╯${RST}\n"
    printf "    ${GREEN}1)${RST} %s\n" "$(t cmd_install)"
    printf "    ${GREEN}2)${RST} %s\n" "$(t cmd_update)"
    printf "    ${GREEN}3)${RST} %s\n" "$(t cmd_uninstall)"
    printf "    ${GREEN}4)${RST} %s\n" "$(t cmd_guardian)"
    printf "    ${GREEN}5)${RST} %s\n" "$(t cmd_status)"
    printf "    ${GREEN}6)${RST} %s\n" "$(t cmd_check)"
    printf "    ${GREEN}7)${RST} %s\n" "$(t cmd_help)"
    printf "    ${RED}0)${RST} %s\n\n" "$(t menu_exit)"
    local c
    read -rp "  $(t menu_choose)" c || { printf '\n'; exit 0; }
    case "$c" in
      1) do_install ;;
      2) do_update ;;
      3) do_uninstall ;;
      4) do_guardian_agent ;;
      5) do_status ;;
      6) do_check ;;
      7) usage ;;
      0|q|exit|خروج) printf '\n'; exit 0 ;;
      *) err "$(t menu_invalid)"; continue ;;
    esac
    printf '\n'
    printf "${DIM}  ──────────────────────────── ${RST}"
    printf "${CYAN}${BOLD}⏎ %s${RST}" "$(t menu_back)"
    printf "${DIM} ────────────────────────────${RST}\n"
    read -rp '' _ 2>/dev/null || exit 0
  done
}

# ============================================================
# dispatch
# ============================================================
cmd=""
if [ "$#" -gt 0 ]; then cmd="$1"; shift; fi
ensure_deps
if [ -z "$cmd" ]; then
  if [ -t 0 ] && [ -t 1 ]; then
    main_menu
    exit 0
  fi
  if [ -f "$CFG" ]; then cmd="update"; else cmd="install"; fi
fi
case "$cmd" in
  new-install|new|fresh|install|i) do_install "$@" ;;
  update|u|deploy)     do_update ;;
  uninstall|remove|rm) do_uninstall "$@" ;;
  guardian|g)          do_guardian_agent "$@" ;;
  status|s)            do_status ;;
  check|c)             do_check ;;
  help|-h|--help)      usage ;;
  menu|m)                main_menu ;;
  *) err "$(t e_unknown) $cmd"; usage; exit 1 ;;
esac