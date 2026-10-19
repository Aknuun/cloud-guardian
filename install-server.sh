#!/usr/bin/env bash
# ============================================================
#  Cloud Guardian — Server edition (Go)
#  Full VPS installer  (systemd service + static Go build)
#
#  one-liner:
#    bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install-server.sh)"
#
#  commands:  (no arg = interactive menu) | install | update | rollback | \
#             reconfig | status | logs | uninstall | help
# ============================================================
set -euo pipefail

REPO="Aknuun/cloud-guardian"
BRANCH="main"

INSTALL_DIR="/opt/cloud-guardian"
SRC_DIR="${INSTALL_DIR}/server"
BIN="${INSTALL_DIR}/cg-server"
CFG="${INSTALL_DIR}/config.json"
DB="${INSTALL_DIR}/data.db"
ENVFILE="${INSTALL_DIR}/server.env"
UNIT="/etc/systemd/system/cg-server.service"
SERVICE="cg-server"

# ref to build from: a branch (default) or a version tag for rollback/install of a
# specific release, e.g.  REF=v4.5.2 bash install-server.sh update
set_ref() {
  REF="$1"
  if [[ "$REF" =~ ^v?[0-9] ]]; then
    CTAR="https://codeload.github.com/${REPO}/tar.gz/refs/tags/${REF}"
  else
    CTAR="https://codeload.github.com/${REPO}/tar.gz/refs/heads/${REF}"
  fi
}
set_ref "${REF:-$BRANCH}"

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

CMD="${1:-}"

need_root() {
  if [ "$(id -u)" != "0" ]; then
    err "This script must be run as root (sudo -i)"
    exit 1
  fi
}

have() { command -v "$1" >/dev/null 2>&1; }

# ------------------------------------------------------------
# install prerequisites (curl + tar)
# ------------------------------------------------------------
ensure_base() {
  if have curl && have tar && have python3; then return; fi
  step "Installing prerequisites"
  if have apt-get; then
    apt-get update -y >/dev/null 2>&1 || true
    DEBIAN_FRONTEND=noninteractive apt-get install -y curl tar ca-certificates python3 >/dev/null 2>&1 || true
  elif have dnf; then
    dnf install -y curl tar ca-certificates python3 >/dev/null 2>&1 || true
  elif have yum; then
    yum install -y curl tar ca-certificates python3 >/dev/null 2>&1 || true
  fi
  have curl || { err "curl is required"; exit 1; }
  have tar  || { err "tar is required";  exit 1; }
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
    warn "Go is outdated; installing a newer one"
  fi
  step "Installing Go"
  local arch ver url
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) err "Unsupported arch: $(uname -m)"; exit 1 ;;
  esac
  ver="$(curl -fsSL "https://go.dev/VERSION?m=text" | head -n1)"
  [ -n "$ver" ] || { err "Failed to resolve Go version"; exit 1; }
  url="https://go.dev/dl/${ver}.linux-${arch}.tar.gz"
  log "go: ${ver} (${arch})"
  curl -fsSL "$url" -o /tmp/go.tar.gz || { err "Go download failed"; exit 1; }
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
  step "Fetching source from GitHub (ref: ${REF})"
  mkdir -p "$INSTALL_DIR"
  local tmp; tmp="$(mktemp -d)"
  curl -fsSL "$CTAR" -o "${tmp}/src.tar.gz" || { err "Source download failed"; rm -rf "$tmp"; exit 1; }
  tar -C "$tmp" -xzf "${tmp}/src.tar.gz"
  local root; root="$(find "$tmp" -maxdepth 1 -type d -name 'cloud-guardian-*' | head -n1)"
  [ -n "$root" ] && [ -d "${root}/server" ] || { err "Unexpected repo layout"; rm -rf "$tmp"; exit 1; }
  rm -rf "$SRC_DIR"
  cp -r "${root}/server" "$SRC_DIR"
  rm -rf "$tmp"
  ok "Source updated"
}

# ------------------------------------------------------------
# build static binary
# ------------------------------------------------------------
build_binary() {
  step "Building server"
  cd "$SRC_DIR"
  export GOCACHE="${INSTALL_DIR}/.gocache" GOPATH="${INSTALL_DIR}/.gopath" GOFLAGS="-mod=mod"
  CGO_ENABLED=0 "$GO_BIN" build -trimpath -ldflags "-s -w" -o "$BIN" ./cmd/cg-server
  chmod +x "$BIN"
  ok "Built"
}

# ------------------------------------------------------------
# config prompts
# ------------------------------------------------------------
json_escape() { printf '%s' "$1" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read())[1:-1])' 2>/dev/null || printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

# read a key from the existing config.json (empty if missing)
cfg_get() {
  [ -f "$CFG" ] || return 0
  python3 - "$1" "$CFG" <<'PY' 2>/dev/null || true
import json,sys
k,p=sys.argv[1],sys.argv[2]
try:
    d=json.load(open(p))
except Exception:
    sys.exit(0)
v=d.get(k,"")
print(v if isinstance(v,str) else ("" if v is None else v))
PY
}

# extract the :port from a url (empty if none)
port_of_url() {
  printf '%s' "$1" | sed -nE 's#^[a-zA-Z][a-zA-Z0-9+.-]*://[^/]*:([0-9]{2,5})(/.*)?$#\1#p'
}

# pick a random free 5-digit TCP port (checks bindability before returning)
pick_free_port() {
  python3 - <<'PY'
import socket, random, sys
def free(p):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        s.bind(("0.0.0.0", p))
        return True
    except OSError:
        return False
    finally:
        s.close()
cands = list(range(10000, 65536))
random.shuffle(cands)
for p in cands[:300]:
    if free(p):
        print(p); sys.exit(0)
for p in cands[300:]:
    if free(p):
        print(p); sys.exit(0)
print(10000)
PY
}

ask() { # ask <prompt> [default] -> REPLY
  local p="$1" d="${2:-}"
  if [ -n "$d" ]; then
    read -r -p "$(printf "${CYAN}?${RST} %s [%s]: " "$p" "$d")" REPLY || true
    REPLY="${REPLY:-$d}"
  else
    read -r -p "$(printf "${CYAN}?${RST} %s: " "$p")" REPLY || true
  fi
}

write_config() {
  step "Server configuration"
  local token admin cf pub acct hz
  token="$(cfg_get bot_token)"; admin="$(cfg_get admin_id)"
  cf="$(cfg_get cf_token)"; acct="$(cfg_get cf_account_id)"; hz="$(cfg_get hz_token)"
  if [ -f "$CFG" ]; then
    warn "config.json exists — press Enter to keep the current value"
  fi

  # bot token
  while :; do
    ask "Telegram bot token (from @BotFather)" "$token"
    token="$REPLY"
    if [[ "$token" =~ ^[0-9]+:[A-Za-z0-9_-]{20,}$ ]]; then break; fi
    err "Invalid token format"
  done

  # admin id
  while :; do
    ask "Your numeric Telegram ID (main admin)" "$admin"
    admin="${REPLY%%,*}"
    if [[ "$admin" =~ ^[0-9]+$ ]]; then break; fi
    err "Must be numeric"
  done

  # cf token (optional)
  ask "Cloudflare API token (optional, Enter to keep/skip)" "$cf"
  cf="$REPLY"

  # public url — auto-detected silently (no prompt); used only by the optional failover.
  # The health/webhook port is random per install (5 digits, verified free) to avoid
  # clashing with other services on the host (e.g. a check-host relay on 8787).
  pub="$(cfg_get public_url)"
  CG_PORT="$(port_of_url "$pub")"
  if [ -z "$pub" ]; then
    [ -n "$CG_PORT" ] || CG_PORT="$(pick_free_port)"
    local ip; ip="$(curl -fsSL --max-time 8 https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || true)"
    if [ -n "$ip" ]; then pub="http://${ip}:${CG_PORT}"; fi
  fi
  [ -n "$CG_PORT" ] || CG_PORT="$(pick_free_port)"
  log "public_url (auto): ${pub:-none}   port: ${CG_PORT}"

  cat > "$CFG" <<EOF
{
  "bot_token": "$(json_escape "$token")",
  "admin_id": ${admin},
  "cf_token": "$(json_escape "$cf")",
  "cf_account_id": "$(json_escape "$acct")",
  "public_url": "$(json_escape "$pub")",
  "hz_token": "$(json_escape "$hz")"
}
EOF
  chmod 600 "$CFG"
  ok "config.json written"
}

# ------------------------------------------------------------
# systemd unit
# ------------------------------------------------------------
write_unit() {
  step "Creating systemd service"
  local port="${CG_PORT:-}"
  [ -n "$port" ] || port="$(port_of_url "$(cfg_get public_url)")"
  [ -n "$port" ] || port="$(pick_free_port)"
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
Environment=CG_HEALTH=:${port}
ExecStart=${BIN}
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$SERVICE" >/dev/null 2>&1 || true
  ok "Service ready"
}

start_service() {
  step "Starting"
  systemctl restart "$SERVICE"
  sleep 2
  if systemctl is-active --quiet "$SERVICE"; then
    ok "Server is running"
  else
    err "Server failed to start; check logs"
    journalctl -u "$SERVICE" -n 30 --no-pager || true
    exit 1
  fi
}

summary() {
  echo
  hr
  printf "${GREEN}${BOLD}%s${RST}\n" "Installation complete 🎉"
  hr
  echo "Commands:"
  echo "  systemctl status  ${SERVICE}"
  echo "  systemctl restart ${SERVICE}"
  echo "  journalctl -u ${SERVICE} -f"
  echo
  echo "Manage:  bash install-server.sh {update|status|logs|uninstall}"
  echo "Now send /start to your bot in Telegram."
  hr
}

is_installed() { [ -x "$BIN" ] || [ -d "$SRC_DIR" ]; }

installed_version() {
  if [ -f "${SRC_DIR}/VERSION" ]; then
    tr -d '[:space:]' < "${SRC_DIR}/VERSION"
  else
    echo "unknown"
  fi
}

# list published version tags (newest first)
list_tags() {
  curl -fsSL -H 'User-Agent: cg-installer' \
    "https://api.github.com/repos/${REPO}/tags?per_page=30" \
  | python3 -c 'import json,sys,re
try:
    d=json.load(sys.stdin)
except Exception:
    sys.exit(1)
t=[x.get("name","") for x in d if isinstance(x,dict)]
t=[v for v in t if re.match(r"^v?\d+(\.\d+){1,2}$",v)]
print("\n".join(t))'
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
  is_installed || { err "Not installed; run install first"; exit 1; }
  fetch_source
  build_binary
  systemctl restart "$SERVICE"
  ok "Updated to $(installed_version)"
}

do_reconfig() {
  banner
  need_root
  [ -f "$CFG" ] || { err "Not installed; run install first"; exit 1; }
  write_config
  if systemctl is-enabled --quiet "$SERVICE" 2>/dev/null; then
    systemctl restart "$SERVICE" >/dev/null 2>&1 || true
    ok "Config saved; service restarted"
  fi
}

do_rollback() {
  banner
  need_root
  ensure_base
  is_installed || { err "Not installed; run install first"; exit 1; }
  step "Available versions   ${DIM}(current: $(installed_version))${RST}"
  local tags; tags="$(list_tags)" || true
  if [ -z "$tags" ]; then err "Could not fetch versions from GitHub"; return 1; fi
  local -a arr=(); local i=1 t
  while IFS= read -r t; do
    [ -n "$t" ] || continue
    arr+=("$t"); printf "  ${CYAN}%2d)${RST} %s\n" "$i" "$t"; i=$((i+1))
  done <<< "$tags"
  printf "  ${CYAN}%2d)${RST} Cancel\n" "0"
  echo
  local a; read -r -p "$(printf "${CYAN}?${RST} Choose a version to install: ")" a || true
  if [ -z "${a:-}" ] || [ "$a" = "0" ]; then warn "Cancelled"; return 0; fi
  if ! [[ "$a" =~ ^[0-9]+$ ]] || [ "$a" -lt 1 ] || [ "$a" -gt "${#arr[@]}" ]; then
    err "Invalid choice"; return 1
  fi
  local tag="${arr[$((a-1))]}"
  echo
  read -r -p "$(printf "${YELLOW}?${RST} Install %s and restart the service? [y/N]: " "$tag")" c || true
  [[ "${c:-}" =~ ^[Yy]$ ]] || { warn "Cancelled"; return 0; }
  ensure_go
  set_ref "$tag"
  fetch_source
  build_binary
  systemctl restart "$SERVICE"
  ok "Now running $(installed_version)"
}

menu() {
  banner
  while :; do
    if is_installed; then
      printf "  status: ${GREEN}installed${RST} ${DIM}(version $(installed_version))${RST}\n"
    else
      printf "  status: ${YELLOW}not installed${RST}\n"
    fi
    echo
    echo "  ${BOLD}1)${RST} Install / reinstall        ${DIM}(fresh install)${RST}"
    echo "  ${BOLD}2)${RST} Update to latest           ${DIM}(build from main)${RST}"
    echo "  ${BOLD}3)${RST} Rollback / pick a version  ${DIM}(install a tag)${RST}"
    echo "  ${BOLD}4)${RST} Reconfigure                ${DIM}(token / admin / CF)${RST}"
    echo "  ${BOLD}5)${RST} Status"
    echo "  ${BOLD}6)${RST} Logs"
    echo "  ${BOLD}7)${RST} Uninstall"
    echo "  ${BOLD}0)${RST} Exit"
    echo
    local c; read -r -p "$(printf "${CYAN}?${RST} Choose an option: ")" c || true
    echo
    case "${c:-}" in
      1) do_install; return ;;
      2) do_update; return ;;
      3) do_rollback; return ;;
      4) do_reconfig; return ;;
      5) do_status ;;
      6) do_logs ;;
      7) do_uninstall; return ;;
      0|"") return ;;
      *) err "Invalid option" ;;
    esac
    echo
  done
}

do_uninstall() {
  need_root
  systemctl stop "$SERVICE" >/dev/null 2>&1 || true
  systemctl disable "$SERVICE" >/dev/null 2>&1 || true
  rm -f "$UNIT"; systemctl daemon-reload || true
  read -r -p "$(printf "${RED}?${RST} %s [y/N]: " "Also delete data & config?")" a || true
  if [[ "${a:-}" =~ ^[Yy]$ ]]; then rm -rf "$INSTALL_DIR"; ok "All removed"; else ok "Service removed (data kept)"; fi
}

do_status() {
  need_root
  systemctl status "$SERVICE" --no-pager || true
}

do_logs() {
  need_root
  journalctl -u "$SERVICE" -n 80 --no-pager || true
}

usage() {
  cat <<EOF
Cloud Guardian — Server installer (Go)

  bash install-server.sh                 interactive menu
  bash install-server.sh install         full install
  bash install-server.sh update          update to latest
  bash install-server.sh rollback        pick & install a version
  bash install-server.sh reconfig        edit config only
  bash install-server.sh status          service status
  bash install-server.sh logs            service logs
  bash install-server.sh uninstall       remove service (asks about data)

  REF=<tag|branch> bash install-server.sh install|update   build a specific ref
EOF
}

case "$CMD" in
  install|i|new-install) do_install ;;
  update|u|upgrade)      do_update ;;
  rollback|rb|downgrade|version) do_rollback ;;
  reconfig|config|cfg)   do_reconfig ;;
  uninstall|remove|rm)   do_uninstall ;;
  status|st)             do_status ;;
  logs|log)              do_logs ;;
  menu|"")               
    if [ -t 0 ]; then
      need_root; menu
    elif is_installed; then
      do_update
    else
      do_install
    fi
    ;;
  help|-h|--help) usage ;;
  *) err "Unknown command: $CMD"; echo; usage; exit 1 ;;
esac
