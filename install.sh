#!/usr/bin/env bash
# ============================================================
# Cloud Guardian — installer & manager  (bilingual: فارسی / English)
#   install | update | uninstall | relay | status | check | help
#
# one-liner:
#   bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install.sh)"
# force language:  CG_LANG=fa|en   or   --lang fa|en
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
    fa:e_curl)                 printf '%s' "curl نصب نیست." ;;
    en:e_curl)                 printf '%s' "curl is not installed." ;;
    fa:e_python)               printf '%s' "python3 نصب نیست." ;;
    en:e_python)               printf '%s' "python3 is not installed." ;;
    fa:e_download)             printf '%s' "دانلود فایل ناموفق بود." ;;
    en:e_download)             printf '%s' "Download failed." ;;
    fa:e_bad_token)            printf '%s' "توکن نامعتبر است یا دسترسی لازم را ندارد." ;;
    en:e_bad_token)            printf '%s' "Token is invalid or lacks the required permissions." ;;
    fa:e_tries)                printf '%s' "تلاش‌های ناموفق تمام شد." ;;
    en:e_tries)                printf '%s' "Too many failed attempts." ;;
    fa:e_worker_name)          printf '%s' "نام ورکر نامعتبر است (حروف/عدد/-/_ تا ۶۳ کاراکتر)." ;;
    en:e_worker_name)          printf '%s' "Invalid worker name (letters/digits/-/_, up to 63 chars)." ;;
    fa:e_account)              printf '%s' "Account ID لازم است." ;;
    en:e_account)              printf '%s' "Account ID is required." ;;
    fa:e_bot_token)            printf '%s' "فرمت توکن نامعتبر است (باید شبیه 123456:AA... باشد)." ;;
    en:e_bot_token)            printf '%s' "Invalid token format (should look like 123456:AA...)." ;;
    fa:e_admin)                printf '%s' "شناسه باید فقط عدد باشد." ;;
    en:e_admin)                printf '%s' "The ID must be numeric only." ;;
    fa:e_config_missing)       printf '%s' "config.json پیدا نشد؛ اول نصب کن." ;;
    en:e_config_missing)       printf '%s' "config.json not found; run install first." ;;
    fa:e_token_missing)        printf '%s' "token در config نیست." ;;
    en:e_token_missing)        printf '%s' "token is missing from config." ;;
    fa:e_nothing_remove)       printf '%s' "config.json پیدا نشد؛ چیزی برای حذف نیست." ;;
    en:e_nothing_remove)       printf '%s' "config.json not found; nothing to remove." ;;
    fa:e_deploy)               printf '%s' "دیپلوی ناموفق بود." ;;
    en:e_deploy)               printf '%s' "Deploy failed." ;;
    fa:e_update)               printf '%s' "آپدیت ناموفق بود." ;;
    en:e_update)               printf '%s' "Update failed." ;;
    fa:e_unknown)              printf '%s' "دستور نامشخص:" ;;
    en:e_unknown)              printf '%s' "Unknown command:" ;;
    fa:e_root)                 printf '%s' "برای نصب رله باید با root اجرا کنی:" ;;
    en:e_root)                 printf '%s' "Relay install needs root:" ;;
    fa:e_systemd)              printf '%s' "systemd لازم است." ;;
    en:e_systemd)              printf '%s' "systemd is required." ;;

    fa:w_empty)                printf '%s' "توکن خالی است." ;;
    en:w_empty)                printf '%s' "Token is empty." ;;
    fa:w_webhook)              printf '%s' "ست‌کردن وبهوک ناموفق بود:" ;;
    en:w_webhook)              printf '%s' "Failed to set webhook:" ;;
    fa:w_cron)                 printf '%s' "نصب کرون ناموفق بود (اختیاری)." ;;
    en:w_cron)                 printf '%s' "Failed to install the cron job (optional)." ;;
    fa:w_account_auto)         printf '%s' "تشخیص خودکار نشد." ;;
    en:w_account_auto)         printf '%s' "Auto-detection failed." ;;
    fa:w_kv_delete)            printf '%s' "حذف KV ناموفق:" ;;
    en:w_kv_delete)            printf '%s' "Failed to delete KV:" ;;
    fa:w_partial)              printf '%s' "بعضی مراحل حذف با خطا مواجه شد." ;;
    en:w_partial)              printf '%s' "Some uninstall steps failed." ;;
    fa:w_subdomain)            printf '%s' "زیردامنهٔ workers.dev پیدا نشد؛ وبهوک را دستی ست کن." ;;
    en:w_subdomain)            printf '%s' "workers.dev subdomain not found; set the webhook manually." ;;
    fa:w_manual_relay)         printf '%s' "config.json نیست؛ رله را دستی از منوی ربات («🖥 سرورها ← 🔧 تنظیم رله») ثبت کن." ;;
    en:w_manual_relay)         printf '%s' "No config.json; register the relay manually from the bot («🖥 Servers ← 🔧 Set relay»)." ;;
    fa:w_relay_reg)            printf '%s' "ثبت خودکار ناموفق؛ دستی از منوی «🖥 سرورها ← 🔧 تنظیم رله» انجام بده." ;;
    en:w_relay_reg)            printf '%s' "Auto-register failed; do it manually from «🖥 Servers ← 🔧 Set relay»." ;;

    fa:ok_token)               printf '%s' "توکن معتبر است." ;;
    en:ok_token)               printf '%s' "Token is valid." ;;
    fa:ok_using_token)         printf '%s' "از توکن ذخیره‌شده/محیطی استفاده می‌شود." ;;
    en:ok_using_token)         printf '%s' "Using the stored/environment token." ;;
    fa:ok_account)             printf '%s' "پیدا شد:" ;;
    en:ok_account)             printf '%s' "Found:" ;;
    fa:ok_config)              printf '%s' "تنظیمات ذخیره شد (دسترسی 600 — فقط خودت)" ;;
    en:ok_config)              printf '%s' "Configuration saved (mode 600 — private)" ;;
    fa:ok_deploy)              printf '%s' "ورکر با موفقیت دیپلوی شد." ;;
    en:ok_deploy)              printf '%s' "Worker deployed successfully." ;;
    fa:ok_webhook)             printf '%s' "وبهوک تلگرام ست شد:" ;;
    en:ok_webhook)             printf '%s' "Telegram webhook set:" ;;
    fa:ok_cron)                printf '%s' "کرون پشتیبانِ آپدیت خودکار نصب شد (هر ۳۰ دقیقه)." ;;
    en:ok_cron)                printf '%s' "Auto-update backup cron installed (every 30 min)." ;;
    fa:ok_updated)             printf '%s' "ورکر آپدیت شد به نسخهٔ" ;;
    en:ok_updated)             printf '%s' "Worker updated to version" ;;
    fa:ok_uninstalled)         printf '%s' "حذف کامل شد." ;;
    en:ok_uninstalled)         printf '%s' "Uninstall complete." ;;
    fa:ok_cron_removed)        printf '%s' "کرون آپدیت خودکار حذف شد." ;;
    en:ok_cron_removed)        printf '%s' "Auto-update cron removed." ;;
    fa:ok_files_removed)       printf '%s' "فایل‌های محلی پاک شدند." ;;
    en:ok_files_removed)       printf '%s' "Local files removed." ;;
    fa:ok_relay_installed)     printf '%s' "رله روی این سرور نصب/آپدیت شد." ;;
    en:ok_relay_installed)     printf '%s' "Relay installed/updated on this server." ;;

    fa:install_title)          printf '%s' "نصب نگهبان ابری (Cloud Guardian)" ;;
    en:install_title)          printf '%s' "Installing Cloud Guardian" ;;
    fa:using_local)            printf '%s' "استفاده از فایل‌های محلی:" ;;
    en:using_local)            printf '%s' "Using local files:" ;;
    fa:downloading)            printf '%s' "دانلود worker.js و deploy-tool.py از گیت‌هاب…" ;;
    en:downloading)            printf '%s' "Downloading worker.js and deploy-tool.py from GitHub…" ;;
    fa:step_token)             printf '%s' "مرحلهٔ ۱ از ۶ — توکن کلادفلر" ;;
    en:step_token)             printf '%s' "Step 1 of 6 — Cloudflare token" ;;
    fa:step_account)           printf '%s' "مرحلهٔ ۲ از ۶ — Account ID" ;;
    en:step_account)           printf '%s' "Step 2 of 6 — Account ID" ;;
    fa:step_worker)            printf '%s' "مرحلهٔ ۳ از ۶ — نام ورکر" ;;
    en:step_worker)            printf '%s' "Step 3 of 6 — Worker name" ;;
    fa:step_bot)               printf '%s' "مرحلهٔ ۴ از ۶ — ربات تلگرام" ;;
    en:step_bot)               printf '%s' "Step 4 of 6 — Telegram bot" ;;
    fa:step_admin)             printf '%s' "مرحلهٔ ۵ از ۶ — شناسهٔ مدیر" ;;
    en:step_admin)             printf '%s' "Step 5 of 6 — Admin ID" ;;
    fa:step_save)              printf '%s' "مرحلهٔ ۶ از ۶ — ذخیرهٔ تنظیمات" ;;
    en:step_save)              printf '%s' "Step 6 of 6 — Save configuration" ;;
    fa:deploy_step)            printf '%s' "دیپلوی روی کلادفلر (KV + bindings + worker + cron + workers.dev)" ;;
    en:deploy_step)            printf '%s' "Deploying to Cloudflare (KV + bindings + worker + cron + workers.dev)" ;;
    fa:reuse_token)            printf '%s' "توکن ذخیره‌شدهٔ قبلی استفاده شود؟ [Y/n] " ;;
    en:reuse_token)            printf '%s' "Reuse the previously stored token? [Y/n] " ;;
    fa:verify_token)           printf '%s' "بررسی توکن…" ;;
    en:verify_token)           printf '%s' "Verifying token…" ;;
    fa:detect_account)         printf '%s' "تشخیص خودکار Account ID…" ;;
    en:detect_account)         printf '%s' "Auto-detecting Account ID…" ;;
    fa:acc_hint)               printf '%s' "از داشبورد: سمت راست، باکس API ← Account ID:" ;;
    en:acc_hint)               printf '%s' "From the dashboard: right sidebar, API box → Account ID:" ;;
    fa:worker_prompt)          printf '%s' "نام ورکر [cloud-guardian]: " ;;
    en:worker_prompt)          printf '%s' "Worker name [cloud-guardian]: " ;;
    fa:bot_hint)               printf '%s' "توکن ربات را از @BotFather بگیر (دستور /newbot)." ;;
    en:bot_hint)               printf '%s' "Get the bot token from @BotFather (command /newbot)." ;;
    fa:bot_prompt)             printf '%s' "توکن ربات: " ;;
    en:bot_prompt)             printf '%s' "Bot token: " ;;
    fa:admin_hint)             printf '%s' "شناسهٔ عددی خودت را از @userinfobot بگیر." ;;
    en:admin_hint)             printf '%s' "Get your numeric ID from @userinfobot." ;;
    fa:admin_prompt)           printf '%s' "شناسهٔ عددی مدیر: " ;;
    en:admin_prompt)           printf '%s' "Numeric admin ID: " ;;
    fa:relay_ask)              printf '%s' "رلهٔ SSH را روی همین سرور نصب کنم؟ (برای پایش/اجرای سرورها) [y/N] " ;;
    en:relay_ask)              printf '%s' "Install the SSH relay on this server? (for server monitoring/SSH) [y/N] " ;;
    fa:done_title)             printf '%s' "نصب کامل شد — نگهبان ابری فعال است" ;;
    en:done_title)             printf '%s' "Install complete — Cloud Guardian is live" ;;
    fa:done_worker)            printf '%s' "ورکر:" ;;
    en:done_worker)            printf '%s' "Worker:" ;;
    fa:done_bot)               printf '%s' "از تلگرام به این ربات پیام بده و /start بزن:" ;;
    en:done_bot)               printf '%s' "Open Telegram, message this bot and press /start:" ;;
    fa:done_cfg)               printf '%s' "تنظیمات (محرمانه):" ;;
    en:done_cfg)               printf '%s' "Config (secret):" ;;
    fa:done_status)            printf '%s' "وضعیت:" ;;
    en:done_status)            printf '%s' "Status:" ;;
    fa:done_remove)            printf '%s' "حذف:" ;;
    en:done_remove)            printf '%s' "Uninstall:" ;;
    fa:done_update)            printf '%s' "آپدیت:" ;;
    en:done_update)            printf '%s' "Update:" ;;

    fa:tk_box)                 printf '%s' "🔑 ساخت توکن API کلادفلر" ;;
    en:tk_box)                 printf '%s' "🔑 Create a Cloudflare API token" ;;
    fa:tk_opens)               printf '%s' "این لینک را در مرورگر باز کن:" ;;
    en:tk_opens)               printf '%s' "Open this link in your browser:" ;;
    fa:tk_create)              printf '%s' "دکمهٔ Create Token → Create Custom Token" ;;
    en:tk_create)              printf '%s' "Click Create Token → Create Custom Token" ;;
    fa:tk_perms)               printf '%s' "این ۵ دسترسی را اضافه کن:" ;;
    en:tk_perms)               printf '%s' "Add these 5 permissions:" ;;
    fa:tk_res)                 printf '%s' "و در پایین: Account Resources = All accounts · Zone Resources = All zones" ;;
    en:tk_res)                 printf '%s' "Then below: Account Resources = All accounts · Zone Resources = All zones" ;;
    fa:tk_copy)                printf '%s' "Continue → Create Token، توکن را کپی کن و اینجا بچسبان." ;;
    en:tk_copy)                printf '%s' "Continue → Create Token, copy the token and paste it here." ;;
    fa:tk_visual)              printf '%s' "راهنمای تصویری:" ;;
    en:tk_visual)              printf '%s' "Visual guide:" ;;
    fa:tk_token_prompt)        printf '%s' "توکن API کلادفلر را اینجا بچسبان: " ;;
    en:tk_token_prompt)        printf '%s' "Paste the Cloudflare API token here: " ;;

    fa:upd_title)              printf '%s' "آپدیت نگهبان ابری" ;;
    en:upd_title)              printf '%s' "Updating Cloud Guardian" ;;
    fa:upd_download)           printf '%s' "دانلود آخرین worker.js از گیت‌هاب…" ;;
    en:upd_download)           printf '%s' "Downloading the latest worker.js from GitHub…" ;;
    fa:upd_versions)           printf '%s' "نسخهٔ فعلی: %s  →  نسخهٔ جدید: %s" ;;
    en:upd_versions)           printf '%s' "Current: %s  →  New: %s" ;;

    fa:un_title)               printf '%s' "حذف نگهبان ابری" ;;
    en:un_title)               printf '%s' "Uninstalling Cloud Guardian" ;;
    fa:un_warn)                printf '%s' "ورکر «%s» از کلادفلر حذف می‌شود (توقف کامل ربات)." ;;
    en:un_warn)                printf '%s' "Worker «%s» will be deleted from Cloudflare (bot stops completely)." ;;
    fa:un_confirm)             printf '%s' "مطمئنی؟ [y/N] " ;;
    en:un_confirm)             printf '%s' "Are you sure? [y/N] " ;;
    fa:un_canceled)            printf '%s' "لغو شد." ;;
    en:un_canceled)            printf '%s' "Cancelled." ;;
    fa:un_keep_kv)             printf '%s' "KV namespace هم حذف شود؟ [Y/n] " ;;
    en:un_keep_kv)             printf '%s' "Delete the KV namespace too? [Y/n] " ;;
    fa:un_removing)            printf '%s' "حذف ورکر…" ;;
    en:un_removing)            printf '%s' "Deleting worker…" ;;
    fa:un_files_q)             printf '%s' "فایل‌های محلی هم پاک شوند؟ [Y/n] " ;;
    en:un_files_q)             printf '%s' "Delete the local files too? [Y/n] " ;;
    fa:un_files_kept)          printf '%s' "فایل‌ها نگه داشته شدند:" ;;
    en:un_files_kept)          printf '%s' "Files kept:" ;;

    fa:rl_title)               printf '%s' "نصب رلهٔ SSH (srv-relay)" ;;
    en:rl_title)               printf '%s' "Installing SSH relay (srv-relay)" ;;
    fa:rl_download)            printf '%s' "دانلود اسکریپت نصب رله از مخزن…" ;;
    en:rl_download)            printf '%s' "Downloading the relay installer from the repo…" ;;
    fa:rl_download_fail)       printf '%s' "دانلود اسکریپت رله ناموفق بود." ;;
    en:rl_download_fail)       printf '%s' "Failed to download the relay installer." ;;
    fa:rl_installing)          printf '%s' "اجرای نصب رله (پورت %s)…" ;;
    en:rl_installing)          printf '%s' "Running relay installer (port %s)…" ;;
    fa:rl_addr)                printf '%s' "آدرس رله:" ;;
    en:rl_addr)                printf '%s' "Relay URL:" ;;
    fa:rl_token)               printf '%s' "توکن رله:" ;;
    en:rl_token)               printf '%s' "Relay token:" ;;
    fa:rl_fw)                  printf '%s' "پورت باید از بیرون باز باشد:" ;;
    en:rl_fw)                  printf '%s' "The port must be open externally:" ;;
    fa:rl_register_q)          printf '%s' "رله خودکار در ربات ثبت شود؟ (یک رکورد A روی زون کلادفلر می‌سازد) [y/N] " ;;
    en:rl_register_q)          printf '%s' "Auto-register the relay in the bot? (creates an A record on your Cloudflare zone) [y/N] " ;;

    fa:st_title)               printf '%s' "وضعیت نگهبان ابری" ;;
    en:st_title)               printf '%s' "Cloud Guardian status" ;;
    fa:ck_title)               printf '%s' "بررسی دسترسی‌های توکن کلادفلر" ;;
    en:ck_title)               printf '%s' "Checking Cloudflare token permissions" ;;

    fa:usage_title)            printf '%s' "نگهبان ابری — راهنما" ;;
    en:usage_title)            printf '%s' "Cloud Guardian — help" ;;
    fa:cmd_install)            printf '%s' "نصب/بازنصب کامل روی کلادفلر (پیش‌فرض)" ;;
    en:cmd_install)            printf '%s' "Full install/reinstall on Cloudflare (default)" ;;
    fa:cmd_update)             printf '%s' "آپدیت ورکر به آخرین نسخهٔ گیت‌هاب" ;;
    en:cmd_update)             printf '%s' "Update the worker to the latest GitHub version" ;;
    fa:cmd_uninstall)          printf '%s' "حذف ورکر (و KV و وبهوک)" ;;
    en:cmd_uninstall)          printf '%s' "Delete the worker (and KV and webhook)" ;;
    fa:cmd_relay)              printf '%s' "نصب رلهٔ SSH روی همین سرور (+ ثبت خودکار)" ;;
    en:cmd_relay)              printf '%s' "Install the SSH relay on this server (+ auto-register)" ;;
    fa:cmd_status)             printf '%s' "نمایش وضعیت ورکر، کرون، KV و وبهوک" ;;
    en:cmd_status)             printf '%s' "Show worker, cron, KV and webhook status" ;;
    fa:cmd_check)              printf '%s' "بررسی دسترسی‌های توکن کلادفلر" ;;
    en:cmd_check)              printf '%s' "Check the Cloudflare token permissions" ;;
    fa:cmd_help)               printf '%s' "همین راهنما" ;;
    en:cmd_help)               printf '%s' "This help" ;;
    fa:usage_cf)               printf '%s' "توکن کلادفلر:" ;;
    en:usage_cf)               printf '%s' "Cloudflare token:" ;;
    fa:usage_oneline)          printf '%s' "نصب یک‌خطی:" ;;
    en:usage_oneline)          printf '%s' "One-line install:" ;;
    fa:step_perms)             printf '%s' "بررسی دسترسی‌های توکن کلادفلر" ;;
    en:step_perms)             printf '%s' "Checking Cloudflare token permissions" ;;
    fa:perm_box)               printf '%s' "دسترسی‌های توکن کلادفلر ناقص/اشتباه است" ;;
    en:perm_box)               printf '%s' "Cloudflare token permissions are missing or wrong" ;;
    fa:perm_body)              printf '%s' "این ۵ دسترسی را روی توکن بده (Account Resources = All accounts · Zone Resources = All zones):" ;;
    en:perm_body)              printf '%s' "Grant these 5 permissions on your token (Account Resources = All accounts · Zone Resources = All zones):" ;;
    fa:perm_fix)               printf '%s' "توکن را از این لینک ویرایش/بساز:" ;;
    en:perm_fix)               printf '%s' "Edit/create the token here:" ;;
    fa:perm_continue)          printf '%s' "با این حال ادامه بدهم؟ [y/N] " ;;
    en:perm_continue)          printf '%s' "Continue anyway? [y/N] " ;;
    fa:perm_abort)             printf '%s' "نصب متوقف شد. توکن را درست کن و دوباره اجرا کن." ;;
    en:perm_abort)             printf '%s' "Install aborted. Fix the token and run again." ;;
    fa:menu_title)             printf '%s' "نگهبان ابری — منوی اصلی" ;;
    en:menu_title)             printf '%s' "Cloud Guardian — main menu" ;;
    fa:menu_exit)              printf '%s' "خروج" ;;
    en:menu_exit)              printf '%s' "Exit" ;;
    fa:menu_choose)            printf '%s' "یک گزینه انتخاب کن: " ;;
    en:menu_choose)            printf '%s' "Choose an option: " ;;
    fa:menu_invalid)           printf '%s' "گزینهٔ نامعتبر." ;;
    en:menu_invalid)           printf '%s' "Invalid option." ;;
    fa:menu_back)              printf '%s' "اینتر بزن تا به منو برگردی" ;;
    en:menu_back)              printf '%s' "Press Enter to return to the menu" ;;
    fa:offer_install)          printf '%s' "می‌خواهی الان نصب کامل انجام شود؟ [y/N] " ;;
    en:offer_install)          printf '%s' "Start a full install now? [y/N] " ;;
    *)                         printf '%s' "$1" ;;
  esac
}

# ============================================================
# Argument parsing:  --lang fa|en  then command
# ============================================================
LANG_ARG=""
POS=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --lang|-l)   LANG_ARG="${2:-}"; shift 2 2>/dev/null || shift ;;
    --lang=*)    LANG_ARG="${1#*=}"; shift ;;
    *)           POS+=("$1"); shift ;;
  esac
done
if [ "${#POS[@]}" -gt 0 ]; then set -- "${POS[@]}"; else set --; fi

set_language() {
  case "${LANG_SEL:-}" in
    en|EN|En|English|english)       CG_L=en ;;
    fa|FA|fa_ir|Fa|Persian|persian) CG_L=fa ;;
    *)                              CG_L=fa ;;
  esac
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
set_webhook() {
  local sub url out
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  if [ -z "$sub" ]; then warn "$(t w_subdomain)"; return 0; fi
  url="https://$WORKER.$sub.workers.dev/tg"
  out="$(curl -sS --max-time 30 "https://api.telegram.org/bot$BOT/setWebhook?url=$url&drop_pending_updates=true" 2>/dev/null || true)"
  if printf '%s' "$out" | grep -q '"ok":true'; then
    ok "$(t ok_webhook) $url"
    printf '%s\n' "$url" > "$DIR/webhook_url"
  else
    warn "$(t w_webhook) ${out:0:200}"
  fi
}
install_cron_backup() {
  gh_raw "$REPO" "update.sh" > "$DIR/update.sh" 2>/dev/null || true
  chmod +x "$DIR/update.sh" 2>/dev/null || true
  if command -v crontab >/dev/null 2>&1 && [ -f "$DIR/update.sh" ]; then
    ( crontab -l 2>/dev/null | grep -v "cloud-guardian/update.sh" ; echo "*/30 * * * * $DIR/update.sh >> $DIR/update.log 2>&1" ) | crontab - 2>/dev/null \
      && ok "$(t ok_cron)" || warn "$(t w_cron)"
  fi
}

_tk_guide_block() {
  printf "${CYAN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${CYAN}${BOLD}  │${RST}  %s\n" "$(t tk_box)"
  printf "${CYAN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n"
  printf "  ${BOLD}1)${RST} %s\n" "$(t tk_opens)"
  printf "        %s\n" "$(link "$CF_TOKENS_URL")"
  printf "  ${BOLD}2)${RST} %s\n" "$(t tk_create)"
  printf "  ${BOLD}3)${RST} %s\n" "$(t tk_perms)"
  printf "       ${GREEN}1)${RST} Account · ${YELLOW}Workers Scripts${RST}     · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}2)${RST} Account · ${YELLOW}Workers KV Storage${RST}  · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}3)${RST} Zone    · ${YELLOW}DNS${RST}                 · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}4)${RST} Zone    · ${YELLOW}Zone Settings${RST}       · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}5)${RST} Zone    · ${YELLOW}Cache Purge${RST}         · ${GREEN}Purge${RST}\n"
  printf "     %s\n" "$(t tk_res)"
  printf "  ${BOLD}4)${RST} %s\n" "$(t tk_copy)"
  printf "     ${DIM}%s %s${RST}\n" "$(t tk_visual)" "$(link "$DOC_TOKEN_URL")"
}
cf_token_guide() {
  local save="$CG_L"
  printf '\n'
  CG_L=fa; _tk_guide_block
  hr
  CG_L=en; _tk_guide_block
  hr
  CG_L="$save"
}

# ─────────────── خطای رنگيِ دسترسی توکن کلادفلر ───────────────
cf_perm_error() {
  printf '\n'
  printf "${RED}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${RED}${BOLD}  │${RST}  ⛔ ${BOLD}%s${RST}\n" "$(t perm_box)"
  printf "${RED}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n\n"
  printf "  %s\n\n" "$(t perm_body)"
  printf "       ${GREEN}1)${RST} Account · ${YELLOW}Workers Scripts${RST}     · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}2)${RST} Account · ${YELLOW}Workers KV Storage${RST}  · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}3)${RST} Zone    · ${YELLOW}DNS${RST}                 · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}4)${RST} Zone    · ${YELLOW}Zone Settings${RST}       · ${GREEN}Edit${RST}\n"
  printf "       ${GREEN}5)${RST} Zone    · ${YELLOW}Cache Purge${RST}         · ${GREEN}Purge${RST}\n\n"
  printf "  %s %s\n\n" "$(t perm_fix)" "$(link "$CF_TOKENS_URL")"
}

# ============================================================
# install
# ============================================================
do_install() {
  local arg
  for arg in "$@"; do case "$arg" in --no-relay) NO_RELAY=1 ;; --force|-y) FORCE=1 ;; esac; done

  printf "\n${MAG}${BOLD}🛡️  %s${RST}\n" "$(t install_title)"
  hr
  fetch_files

  step "$(t step_token)"
  load_cfg
  local TOKEN="${CF_TOKEN:-}" tries=0
  if [ -z "$TOKEN" ] && [ -n "$CFG_TOKEN" ]; then
    read -rp "  $(t reuse_token)" a
    [[ "${a,,}" == "n" ]] || TOKEN="$CFG_TOKEN"
  fi
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

  step "$(t step_worker)"
  local WORKER="${WORKER_NAME:-}"
  if [ -z "$WORKER" ]; then
    read -rp "  $(t worker_prompt)" WORKER
    WORKER="${WORKER:-cloud-guardian}"
  fi
  [[ "$WORKER" =~ ^[a-zA-Z0-9_-]{1,63}$ ]] || { err "$(t e_worker_name)"; exit 1; }

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
    [[ "$ADMIN" =~ ^[0-9]+$ ]] && break
    err "$(t e_admin)"; ADMIN=""
  done

  step "$(t step_save)"
  write_cfg
  chmod 700 "$DIR" 2>/dev/null || true
  ok "$(t ok_config): $CFG"

  step "$(t step_perms)"
  if ! ( cd "$DIR" && CG_LANG="$CG_L" python3 deploy-tool.py check --no-box ); then
    cf_perm_error
    if [ -n "${FORCE:-}" ]; then
      warn "$(t perm_abort)"
    elif [ -t 0 ]; then
      read -rp "  $(t perm_continue)" a
      [[ "${a,,}" == "y" ]] || { err "$(t perm_abort)"; exit 1; }
    else
      err "$(t perm_abort)"; exit 1
    fi
  fi

  step "$(t deploy_step)"
  if ! ( cd "$DIR" && python3 deploy-tool.py install ); then
    cf_perm_error
    err "$(t e_deploy)"
    exit 1
  fi
  ok "$(t ok_deploy)"
  set_cfg_version "$(read_worker_version)"

  b "$(t ok_webhook)"
  set_webhook
  install_cron_backup

  local sub url
  sub="$(workers_subdomain "$TOKEN" "$ACC")"
  url="https://$WORKER.$sub.workers.dev"

  printf '\n'
  printf "${GREEN}${BOLD}  ╭──────────────────────────────────────────────────────────────╮${RST}\n"
  printf "${GREEN}${BOLD}  │${RST}  🎉 ${GREEN}${BOLD}%s${RST}\n" "$(t done_title)"
  printf "${GREEN}${BOLD}  ╰──────────────────────────────────────────────────────────────╯${RST}\n\n"
  printf "  • %s %s\n" "$(t done_worker)" "$(link "$url")"
  local BU; BU="$(bot_username "$BOT")"
  if [ -n "$BU" ]; then
    printf "  • %s %s\n" "$(t done_bot)" "$(link "https://t.me/$BU")"
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
  load_cfg
  need_config_or_install || return 0
  [ -n "$CFG_TOKEN" ] || { err "$(t e_token_missing)"; exit 1; }
  mkdir -p "$DIR"; chmod 700 "$DIR" 2>/dev/null || true
  b "$(t upd_download)"
  if [ -n "$SRC_DIR" ] && [ -f "$SRC_DIR/worker.js" ]; then
    cp "$SRC_DIR/worker.js" "$DIR/worker.js"
  else
    gh_raw "$REPO" "worker.js" > "$DIR/worker.js"
    gh_raw "$REPO" "deploy-tool.py" > "$DIR/deploy-tool.py"
    chmod +x "$DIR/deploy-tool.py"
  fi
  local newv oldv
  newv="$(read_worker_version)"
  oldv="$(python3 -c "import json;print(json.load(open('$CFG')).get('version',''))" 2>/dev/null || true)"
  printf "$(t upd_versions)\n" "${oldv:-?}" "${newv:-?}"
  if ! ( cd "$DIR" && python3 deploy-tool.py update ); then err "$(t e_update)"; exit 1; fi
  set_cfg_version "$newv"
  ok "$(t ok_updated) ${newv:-?}."
}

# ============================================================
# uninstall
# ============================================================
do_uninstall() {
  printf "\n${MAG}${BOLD}🗑️  %s${RST}\n" "$(t un_title)"; hr
  need_tools
  load_cfg
  [ -f "$CFG" ] || { err "$(t e_nothing_remove)"; exit 1; }
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
# relay
# ============================================================
do_relay() {
  printf "\n${MAG}${BOLD}🖥️  %s${RST}\n" "$(t rl_title)"; hr
  need_tools
  if [ "$(id -u)" -ne 0 ]; then err "$(t e_root)  sudo bash install.sh relay"; exit 1; fi
  command -v systemctl >/dev/null 2>&1 || { err "$(t e_systemd)"; exit 1; }
  load_cfg

  local PORT="${RELAY_PORT:-8788}"
  local TOK="${SRV_RELAY_TOKEN:-}"
  [ -n "$TOK" ] || TOK="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
  local PUBIP; PUBIP="$(public_ip)"
  [ -n "$PUBIP" ] || PUBIP="<server-public-ip>"

  b "$(t rl_download)"
  local TMP; TMP="$(mktemp)"
  gh_raw "$RELAY_REPO" "srv-relay-install.sh" > "$TMP"
  if ! grep -q "srv-relay" "$TMP" 2>/dev/null; then err "$(t rl_download_fail)"; rm -f "$TMP"; exit 1; fi
  printf "$(t rl_installing)\n" "$PORT"
  SRV_RELAY_TOKEN="$TOK" SRV_RELAY_PORT="$PORT" bash "$TMP"
  rm -f "$TMP"
  ok "$(t ok_relay_installed)"

  printf '\n'
  printf "  • %s %s\n" "$(t rl_addr)" "$(link "http://$PUBIP:$PORT")"
  printf "  • %s ${YELLOW}${BOLD}%s${RST}\n" "$(t rl_token)" "$TOK"
  printf "  • %s ${BOLD}sudo ufw allow %s/tcp${RST}\n" "$(t rl_fw)" "$PORT"

  if [ -f "$CFG" ] && [ -n "$CFG_TOKEN" ]; then
    printf '\n'
    read -rp "  🔗 $(t rl_register_q)" a
    if [[ "${a,,}" == "y" ]]; then
      ( cd "$DIR" && SRV_RELAY_TOKEN="$TOK" python3 deploy-tool.py relay-register "$PUBIP" --token "$TOK" --port "$PORT" ) \
        || warn "$(t w_relay_reg)"
    fi
  else
    warn "$(t w_manual_relay)"
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
# اگر config نیست، به‌جای خطا، نصب کامل را پیشنهاد بده
need_config_or_install() {
  if [ -f "$CFG" ]; then return 0; fi
  warn "$(t e_config_missing)"
  if [ -t 0 ]; then
    local a; read -rp "  $(t offer_install)" a
    if [[ "${a,,}" == "y" ]]; then do_install; else return 1; fi
  else
    return 1
  fi
}
do_status() { printf "\n${MAG}${BOLD}📊 %s${RST}\n" "$(t st_title)"; hr; ensure_tool_only; need_config_or_install || return 0; ( cd "$DIR" && python3 deploy-tool.py status ); printf '\n'; }
do_check()  { printf "\n${MAG}${BOLD}🔎 %s${RST}\n" "$(t ck_title)"; hr; ensure_tool_only; need_config_or_install || return 0; ( cd "$DIR" && python3 deploy-tool.py check ); printf '\n'; }

usage() {
  printf "\n${MAG}${BOLD}🛡️  %s${RST}\n" "$(t usage_title)"; hr
  printf "  ${GREEN}install${RST}      %s\n" "$(t cmd_install)"
  printf "  ${GREEN}update${RST}       %s\n" "$(t cmd_update)"
  printf "  ${GREEN}uninstall${RST}    %s\n" "$(t cmd_uninstall)"
  printf "  ${GREEN}relay${RST}        %s\n" "$(t cmd_relay)"
  printf "  ${GREEN}status${RST}       %s\n" "$(t cmd_status)"
  printf "  ${GREEN}check${RST}        %s\n" "$(t cmd_check)"
  printf "  ${GREEN}help${RST}         %s\n" "$(t cmd_help)"
  hr
  printf "  %s %s\n" "$(t usage_cf)" "$(link "$CF_TOKENS_URL")"
  printf "  %s ${DIM}bash -c \"\$(curl -sL https://raw.githubusercontent.com/$REPO/main/install.sh)\"${RST}\n" "$(t usage_oneline)"
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
    printf "    ${GREEN}4)${RST} %s\n" "$(t cmd_relay)"
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
      4) do_relay ;;
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
if [ -z "$cmd" ]; then
  if [ -t 0 ] && [ -t 1 ]; then
    main_menu
    exit 0
  fi
  cmd="install"
fi
case "$cmd" in
  install|i)           do_install "$@" ;;
  update|u|deploy)     do_update ;;
  uninstall|remove|rm) do_uninstall "$@" ;;
  relay|r)             do_relay "$@" ;;
  status|s)            do_status ;;
  check|c)             do_check ;;
  help|-h|--help)      usage ;;
  *) err "$(t e_unknown) $cmd"; usage; exit 1 ;;
esac
