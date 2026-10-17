#!/usr/bin/env bash
# ============================================================
#  نگهبان ابری — نسخهٔ سروری (Go)   |   Cloud Guardian — Server edition
#  نصب‌کنندهٔ کامل روی VPS لینوکسی (systemd + Go build)
#
#  one-liner:
#    bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install-server.sh)"
#
#  commands:  install (default) | update | uninstall | status | logs | help
#  language:  English by default.  Override with  CG_LANG=fa  or  --lang fa
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
BRANCH="main"
CTAR="https://codeload.github.com/${REPO}/tar.gz/refs/heads/${BRANCH}"

INSTALL_DIR="/opt/cloud-guardian"
SRC_DIR="${INSTALL_DIR}/server"
BIN="${INSTALL_DIR}/cg-server"
CFG="${INSTALL_DIR}/config.json"
DB="${INSTALL_DIR}/data.db"
ENVFILE="${INSTALL_DIR}/server.env"
UNIT="/etc/systemd/system/cg-server.service"
SERVICE="cg-server"

# --- colors ---
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  RST=$'\033[0m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; UL=$'\033[4m'
  BLUE=$'\033[1;34m'; CYAN=$'\033[1;36m'; GREEN=$'\033[1;32m'
  YELLOW=$'\033[1;33m'; RED=$'\033[1;31m'; MAG=$'\033[1;35m'
else
  RST=""; BOLD=""; DIM=""; UL=""; BLUE=""; CYAN=""; GREEN=""; YELLOW=""; RED=""; MAG=""
fi

log()  { printf "${BLUE}[*]${RST} %s\n" "$*"; }
step() { printf "\n${MAG}${BOLD}▶ %s${RST}\n" "$*"; }
ok()   { printf "${GREEN}✅ %s${RST}\n" "$*"; }
warn() { printf "${YELLOW}⚠️  %s${RST}\n" "$*"; }
err()  { printf "${RED}❌ %s${RST}\n" "$*" >&2; }
hr()   { printf "${DIM}──────────────────────────────────────────────────────────${RST}\n"; }

banner() {
  echo
  hr
  printf "  ${CYAN}${BOLD}Cloud Guardian${RST} ${DIM}·${RST} ${GREEN}${BOLD}Server edition (Go)${RST}\n"
  printf "  ${DIM}VPS installer — systemd service + static Go build${RST}\n"
  hr
  echo
}

# --- language (English by default) ---
CG_LANG="${CG_LANG:-en}"
args=()
while [ $# -gt 0 ]; do
  case "$1" in
    --lang) CG_LANG="${2:-en}"; shift 2 ;;
    --lang=*) CG_LANG="${1#*=}"; shift ;;
    fa|en) CG_LANG="$1"; shift ;;
    *) args+=("$1"); shift ;;
  esac
done
CMD="${args[0]:-install}"

t() { # t <fa> <en>
  if [ "$CG_LANG" = "en" ]; then printf '%s' "$2"; else printf '%s' "$1"; fi
}

need_root() {
  if [ "$(id -u)" != "0" ]; then
    err "$(t "این اسکریپت باید با root اجرا شود (sudo -i)" "This script must be run as root (sudo -i)")"
    exit 1
  fi
}

have() { command -v "$1" >/dev/null 2>&1; }

# ------------------------------------------------------------
# install prerequisites (curl + tar)
# ------------------------------------------------------------
ensure_base() {
  if have curl && have tar; then return; fi
  step "$(t "نصب پیش‌نیازها" "Installing prerequisites")"
  if have apt-get; then
    apt-get update -y >/dev/null 2>&1 || true
    DEBIAN_FRONTEND=noninteractive apt-get install -y curl tar ca-certificates >/dev/null 2>&1 || true
  elif have dnf; then
    dnf install -y curl tar ca-certificates >/dev/null 2>&1 || true
  elif have yum; then
    yum install -y curl tar ca-certificates >/dev/null 2>&1 || true
  fi
  have curl || { err "curl"; exit 1; }
  have tar  || { err "tar";  exit 1; }
}

# ------------------------------------------------------------
# ensure Go toolchain (>= 1.26)
# ------------------------------------------------------------
go_major_minor() { go version 2>/dev/null | sed -E 's/.*go([0-9]+)\.([0-9]+).*/\1 \2/'; }

ensure_go() {
  local need_major=1 need_minor=26
  if have go; then
    read -r gm gn <<<"$(go_major_minor)"
    if [ "${gm:-0}" -gt "$need_major" ] || { [ "${gm:-0}" -eq "$need_major" ] && [ "${gn:-0}" -ge "$need_minor" ]; }; then
      GO_BIN="$(command -v go)"; return
    fi
    warn "$(t "نسخهٔ Go قدیمی است؛ نسخهٔ جدید نصب می‌شود" "Go is outdated; installing a newer one")"
  fi
  step "$(t "نصب Go" "Installing Go")"
  local arch ver url
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) err "$(t "معماری پشتیبانی‌نشده: $(uname -m)" "Unsupported arch: $(uname -m)")"; exit 1 ;;
  esac
  ver="$(curl -fsSL "https://go.dev/VERSION?m=text" | head -n1)"
  [ -n "$ver" ] || { err "$(t "دریافت نسخهٔ Go ناموفق" "Failed to resolve Go version")"; exit 1; }
  url="https://go.dev/dl/${ver}.linux-${arch}.tar.gz"
  log "go: ${ver} (${arch})"
  curl -fsSL "$url" -o /tmp/go.tar.gz || { err "$(t "دانلود Go ناموفق" "Go download failed")"; exit 1; }
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz
  GO_BIN="/usr/local/go/bin/go"
  export PATH="/usr/local/go/bin:${PATH}"
}

# ------------------------------------------------------------
# fetch server source from GitHub
# ------------------------------------------------------------
fetch_source() {
  step "$(t "دریافت سورس از گیت‌هاب" "Fetching source from GitHub")"
  mkdir -p "$INSTALL_DIR"
  local tmp; tmp="$(mktemp -d)"
  curl -fsSL "$CTAR" -o "${tmp}/src.tar.gz" || { err "$(t "دانلود سورس ناموفق" "Source download failed")"; rm -rf "$tmp"; exit 1; }
  tar -C "$tmp" -xzf "${tmp}/src.tar.gz"
  local root; root="$(find "$tmp" -maxdepth 1 -type d -name 'cloud-guardian-*' | head -n1)"
  [ -n "$root" ] && [ -d "${root}/server" ] || { err "$(t "ساختار مخزن نامعتبر" "Unexpected repo layout")"; rm -rf "$tmp"; exit 1; }
  rm -rf "$SRC_DIR"
  cp -r "${root}/server" "$SRC_DIR"
  rm -rf "$tmp"
  ok "$(t "سورس به‌روز شد" "Source updated")"
}

# ------------------------------------------------------------
# build static binary
# ------------------------------------------------------------
build_binary() {
  step "$(t "کامپایل سرور" "Building server")"
  cd "$SRC_DIR"
  export GOCACHE="${INSTALL_DIR}/.gocache" GOPATH="${INSTALL_DIR}/.gopath" GOFLAGS="-mod=mod"
  CGO_ENABLED=0 "$GO_BIN" build -trimpath -ldflags "-s -w" -o "$BIN" ./cmd/cg-server
  chmod +x "$BIN"
  ok "$(t "کامپایل شد" "Built")"
}

# ------------------------------------------------------------
# config prompts
# ------------------------------------------------------------
json_escape() { printf '%s' "$1" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read())[1:-1])' 2>/dev/null || printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

ask() { # ask <prompt> <default> -> REPLY
  local p="$1" d="${2:-}"
  if [ -n "$d" ]; then
    read -r -p "$(printf "${CYAN}?${RST} %s [%s]: " "$p" "$d")" REPLY || true
    REPLY="${REPLY:-$d}"
  else
    read -r -p "$(printf "${CYAN}?${RST} %s: " "$p")" REPLY || true
  fi
}

write_config() {
  step "$(t "تنظیمات سرور" "Server configuration")"
  local token admin cf ip pub refresh
  if [ -f "$CFG" ]; then
    warn "$(t "config.json موجود است؛ Enter برای حفظ مقادیر فعلی" "config.json exists; press Enter to keep current values")"
  fi

  # bot token
  while :; do
    ask "$(t "توکن ربات تلگرام (از @BotFather)" "Telegram bot token (from @BotFather)")" ""
    token="$REPLY"
    if [[ "$token" =~ ^[0-9]+:[A-Za-z0-9_-]{20,}$ ]]; then break; fi
    err "$(t "فرمت توکن نامعتبر است" "Invalid token format")"
  done

  # admin id
  while :; do
    ask "$(t "آیدی عددی تلگرام شما (ادمین اصلی)" "Your numeric Telegram ID (main admin)")" ""
    admin="${REPLY%%,*}"
    if [[ "$admin" =~ ^[0-9]+$ ]]; then break; fi
    err "$(t "باید عددی باشد" "Must be numeric")"
  done

  # cf token (optional)
  ask "$(t "توکن API کلادفلر (اختیاری، Enter برای رد)" "Cloudflare API token (optional, Enter to skip)")" ""
  cf="$REPLY"

  # public url — auto-detected silently (no prompt); used only by the optional failover
  ip="$(curl -fsSL --max-time 8 https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || true)"
  if [ -n "$ip" ]; then pub="http://${ip}:8787"; else pub=""; fi
  log "$(t "آدرس عمومی (خودکار): ${pub:-—}" "public_url (auto): ${pub:-none}")"

  cat > "$CFG" <<EOF
{
  "bot_token": "$(json_escape "$token")",
  "admin_id": ${admin},
  "cf_token": "$(json_escape "$cf")",
  "cf_account_id": "",
  "public_url": "$(json_escape "$pub")",
  "hz_token": ""
}
EOF
  chmod 600 "$CFG"
  ok "$(t "config.json نوشته شد" "config.json written")"
}

# ------------------------------------------------------------
# systemd unit
# ------------------------------------------------------------
write_unit() {
  step "$(t "ساخت سرویس systemd" "Creating systemd service")"
  cat > "$UNIT" <<EOF
[Unit]
Description=Cloud Guardian Server (Go)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
Environment=CG_CONFIG=${CFG}
Environment=CG_DB=${DB}
Environment=CG_HEALTH=:8787
ExecStart=${BIN}
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$SERVICE" >/dev/null 2>&1 || true
  ok "$(t "سرویس آماده شد" "Service ready")"
}

start_service() {
  step "$(t "راه‌اندازی" "Starting")"
  systemctl restart "$SERVICE"
  sleep 2
  if systemctl is-active --quiet "$SERVICE"; then
    ok "$(t "سرور فعال شد" "Server is running")"
  else
    err "$(t "سرور بالا نیامد؛ لاگ را ببین" "Server failed to start; check logs")"
    journalctl -u "$SERVICE" -n 30 --no-pager || true
    exit 1
  fi
}

summary() {
  echo
  hr
  printf "${GREEN}${BOLD}%s${RST}\n" "$(t "نصب کامل شد 🎉" "Installation complete 🎉")"
  hr
  echo "$(t "دستورها:" "Commands:")"
  echo "  systemctl status  ${SERVICE}"
  echo "  systemctl restart ${SERVICE}"
  echo "  journalctl -u ${SERVICE} -f"
  echo
  echo "$(t "مدیریت:" "Manage:")  bash install-server.sh {update|status|logs|uninstall}"
  echo "$(t "حالا در تلگرام به ربات /start بده." "Now send /start to your bot in Telegram.")"
  hr
}

do_install() {
  banner
  need_root
  ensure_base
  ensure_go
  fetch_source
  build_binary
  write_config
  write_unit
  start_service
  summary
}

do_update() {
  banner
  need_root
  ensure_base
  ensure_go
  [ -d "$SRC_DIR" ] || { err "$(t "نصب نیست؛ اول install" "Not installed; run install first")"; exit 1; }
  fetch_source
  build_binary
  systemctl restart "$SERVICE"
  ok "$(t "به‌روزرسانی شد" "Updated")"
}

do_uninstall() {
  need_root
  systemctl stop "$SERVICE" >/dev/null 2>&1 || true
  systemctl disable "$SERVICE" >/dev/null 2>&1 || true
  rm -f "$UNIT"; systemctl daemon-reload || true
  read -r -p "$(printf "${RED}?${RST} %s [y/N]: " "$(t "دادهها و تنظیمات هم حذف شوند؟" "Also delete data & config?")")" a || true
  if [[ "${a:-}" =~ ^[Yy]$ ]]; then rm -rf "$INSTALL_DIR"; ok "$(t "همه حذف شد" "All removed")"; else ok "$(t "سرویس حذف شد (داده‌ها ماند)" "Service removed (data kept)")"; fi
}

do_status() {
  need_root
  systemctl status "$SERVICE" --no-pager || true
}

do_logs() {
  need_root
  journalctl -u "$SERVICE" -n 80 --no-pager || true
}

case "$CMD" in
  install|i|new-install) do_install ;;
  update|u) do_update ;;
  uninstall|remove|rm) do_uninstall ;;
  status|st) do_status ;;
  logs|log) do_logs ;;
  help|-h|--help)
    cat <<EOF
$(t "نگهبان ابری — نصب سروری" "Cloud Guardian — Server installer")
  install     $(t "نصب کامل" "full install")
  update      $(t "به‌روزرسانی" "update")
  uninstall   $(t "حذف" "uninstall")
  status      $(t "وضعیت سرویس" "service status")
  logs        $(t "لاگ‌ها" "logs")
EOF
    ;;
  *) err "$(t "دستور ناشناخته" "Unknown command"): $CMD"; exit 1 ;;
esac
