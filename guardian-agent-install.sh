#!/usr/bin/env bash
# =====================================================================
# guardian-agent-install.sh — نصب ایجنت «نگهبان ابری» روی سرور مشتری
# ---------------------------------------------------------------------
# Ubuntu/Debian — نیاز به root. این ایجنت جایگزین srv-relay قدیمی است:
# هم جاب‌های دوره‌ای را روی همین سرور اجرا می‌کند (تا ورکر به سقف
# نوشتن/س‌ی‌پی‌یو کلادفلر نخورد) و هم اجرای دستور برای منوی 🖥 سرورها.
#
# روش ۱ — توکن از ربات (توصیه می‌شود؛ دکمه «🖥 نصب ایجنت روی سرور»):
#   sudo GUARDIAN_TOKEN="..." WORKER_URL="https://....workers.dev" \
#     bash -c "$(curl -fsSL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent-install.sh)"
#
# روش ۲ — دستی (توکن همین‌جا ساخته می‌شود؛ بعد در ربات دکمه
# «🔑 ثبت توکن ایجنت» را بزن و همین توکن را بده):
#   sudo WORKER_URL="https://....workers.dev" \
#     bash -c "$(curl -fsSL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent-install.sh)"
#
# متغیرهای اختیاری: PORT (پیش‌فرض 8789، فقط محلی) · AGENT_SRC_URL (منبع دانلود فایل ایجنت)
# مدل 3.0: نه آدرس عمومی لازم است نه پورت باز — ایجنت خودش به ورکر وصل می‌شود.
# =====================================================================
set -u

# colors for terminal output (disabled when piped)
if [ -t 1 ]; then TCOL='\033[1;33m'; RST='\033[0m'; else TCOL=''; RST=''; fi

REPO_RAW="${AGENT_SRC_URL:-https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent.js}"
# رانر همیشه از همان پایهٔ ایجنت می‌آید (AGENT_SRC_URL سفارشی هم پوشش داده می‌شود)
RUNNER_RAW="${REPO_RAW%/*}/guardian-runner.js"
INSTALL_DIR="/opt/guardian-agent"
CONF_DIR="/etc/guardian-agent"
CONF_FILE="$CONF_DIR/agent.conf"
STATE_DIR="/var/lib/guardian-agent"
UNIT="/etc/systemd/system/guardian-agent.service"
PORT="${PORT:-8789}"
WORKER_URL="${WORKER_URL:-}"
PUBLIC_URL="${PUBLIC_URL:-}"
TOKEN_IN="${GUARDIAN_TOKEN:-}"

info() { echo "[guardian] $*"; }
fail() { echo "[guardian] ❌ $*" >&2; exit 1; }

[ "$(id -u)" = "0" ] || fail "Run as root (use sudo)."
command -v systemctl >/dev/null 2>&1 || fail "systemd not found (Ubuntu/Debian with systemd only)."
command -v curl >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq curl ca-certificates; } || fail "Failed to install curl."

# ---- swap: افزودن swapfile بر اساس ظرفیت موجود (اگر سواپ فعال نباشد) ----
swap_kb="$(awk '/^SwapTotal:/{print $2}' /proc/meminfo 2>/dev/null || echo 0)"
if [ "${swap_kb:-0}" -gt 0 ]; then
  info "Swap already active ($(( swap_kb / 1024 )) MB) - skipping."
else
  mem_mb="$(awk '/^MemTotal:/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)"
  # اندازه: رم ≤۲گیگ → ۲برابر، ≤۸گیگ → برابر، بیشتر → ۴گیگ؛ محدود ۵۱۲مگ..۸گیگ
  if   [ "$mem_mb" -le 2048 ]; then swap_mb=$(( mem_mb * 2 ))
  elif [ "$mem_mb" -le 8192 ]; then swap_mb="$mem_mb"
  else swap_mb=4096
  fi
  [ "$swap_mb" -lt 512 ]  && swap_mb=512
  [ "$swap_mb" -gt 8192 ] && swap_mb=8192
  # هرگز بیشتر از فضای خالی دیسک منهای ۱گیگ حاشیه امن
  avail_mb="$(df -Pm / 2>/dev/null | awk 'NR==2{print $4}' || echo 0)"
  max_mb=$(( ${avail_mb:-0} - 1024 ))
  [ "$swap_mb" -gt "$max_mb" ] && swap_mb="$max_mb"
  if [ "${swap_mb:-0}" -lt 512 ]; then
    info "Not enough free disk for a swapfile - skipping swap."
  else
    info "Adding ${swap_mb} MB swapfile ..."
    if fallocate -l "${swap_mb}M" /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count="$swap_mb" status=none 2>/dev/null; then
      chmod 600 /swapfile
      if mkswap /swapfile >/dev/null 2>&1 && swapon /swapfile 2>/dev/null; then
        grep -q '^/swapfile ' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
        info "Swap enabled: ${swap_mb} MB at /swapfile"
      else
        info "Warning: could not activate swap on this host - continuing."
        rm -f /swapfile
      fi
    else
      info "Warning: could not create swapfile - continuing."
    fi
  fi
fi

# ---- Node ≥ 18 (مثل نصاب رله) ----
need_node=0
if command -v node >/dev/null 2>&1; then
  major="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0)"
  [ "${major:-0}" -ge 18 ] 2>/dev/null || need_node=1
else
  need_node=1
fi
if [ "$need_node" = "1" ]; then
  info "Installing Node.js 22 ..."
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null 2>&1 || fail "Failed to add NodeSource repository."
  apt-get install -y -qq nodejs || fail "Failed to install nodejs."
fi
node --version || fail "node is not available."
NODE_BIN="$(command -v node)"

# ---- SSH client for proxy jobs (exec/stats on managed servers) ----
command -v ssh >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq openssh-client; } || fail "Failed to install openssh-client."
command -v sshpass >/dev/null 2>&1 || { apt-get install -y -qq sshpass; } || fail "Failed to install sshpass."

# ---- پورت: اگر چیزی غیر از خودمان رویش است، با پیام تمیز بایست ----
_port_open=0
if (echo > /dev/tcp/127.0.0.1/"${PORT}") 2>/dev/null; then _port_open=1; fi
if [ "$_port_open" = "1" ]; then
  _pp="$(curl -fsSL --max-time 5 "http://127.0.0.1:${PORT}/ping" 2>/dev/null || true)"
  if ! echo "$_pp" | grep -q '"agent":"guardian-agent"'; then
    fail "Port $PORT is busy (another service). Pass a different one with PORT=..."
  fi
fi

# ---- توکن: از env یا ساخت تازه ----
TOKEN="$TOKEN_IN"
TOKEN_SRC="env"
if [ -z "$TOKEN" ] && [ -f "$CONF_FILE" ]; then
  # نصب مجدد: توکن قبلی حفظ می‌شود (مثل pg-node: ثبت قبلی در ربات معتبر می‌ماند)
  old="$(grep -E '^GUARDIAN_TOKEN=' "$CONF_FILE" | cut -d= -f2- | tr -d '"' | tr -d ' \r\n')"
  if [ -n "$old" ]; then TOKEN="$old"; TOKEN_SRC="kept"; fi
fi
if [ -z "$TOKEN" ]; then
  if command -v openssl >/dev/null 2>&1; then TOKEN="$(openssl rand -hex 16)"
  else TOKEN="$(tr -dc 'a-f0-9' </dev/urandom | head -c 32)"; fi
  TOKEN_SRC="generated"
fi
[ "${#TOKEN}" -ge 16 ] || fail "Invalid token (minimum 16 chars)."

# ---- دانلود فایل ایجنت (با fallback مثل نصاب رله) ----
mkdir -p "$INSTALL_DIR" "$CONF_DIR" "$STATE_DIR"
chmod 700 "$CONF_DIR" "$STATE_DIR"
got=0
for url in "$REPO_RAW" \
           "https://cdn.jsdelivr.net/gh/Aknuun/cloud-guardian@main/guardian-agent.js" \
           "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent.js"; do
  if curl -fsSL --max-time 60 "$url" -o "$INSTALL_DIR/guardian-agent.js.new"; then
    if grep -q "guardian-agent" "$INSTALL_DIR/guardian-agent.js.new" && [ "$(wc -c <"$INSTALL_DIR/guardian-agent.js.new")" -gt 5000 ]; then
      mv "$INSTALL_DIR/guardian-agent.js.new" "$INSTALL_DIR/guardian-agent.js"
      got=1; break
    fi
  fi
done
[ "$got" = "1" ] || fail "Failed to download guardian-agent.js."
node --check "$INSTALL_DIR/guardian-agent.js" || fail "Agent file is corrupt."
# رانر اجرای جاب‌ها (اختیاری ولی توصیه‌شده؛ بدون آن فقط heartbeat/exec کار می‌کند)
_rgot=0
for _rurl in "$RUNNER_RAW" \
           "https://cdn.jsdelivr.net/gh/Aknuun/cloud-guardian@main/guardian-runner.js" \
           "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-runner.js"; do
  if curl -fsSL --max-time 60 "$_rurl" -o "$INSTALL_DIR/guardian-runner.js.new"; then
    if grep -q "guardian-runner" "$INSTALL_DIR/guardian-runner.js.new" && [ "$(wc -c <"$INSTALL_DIR/guardian-runner.js.new")" -gt 5000 ]; then
      mv "$INSTALL_DIR/guardian-runner.js.new" "$INSTALL_DIR/guardian-runner.js"
      _rgot=1; break
    fi
  fi
done
if [ "$_rgot" = "1" ]; then
  node --check "$INSTALL_DIR/guardian-runner.js" || { rm -f "$INSTALL_DIR/guardian-runner.js"; info "Runner file corrupt, continuing without runner."; }
else
  info "Runner download failed, continuing without runner (heartbeat/exec still work)."
fi

# ---- کانفیگ ----
# مدل 3.0: ایجنت فقط بیرون‌زنگ می‌زند (heartbeat به ورکر)؛ ورکر هیچ‌وقت به ایجنت وصل نمی‌شود.
# پس نه آدرس عمومی لازم است، نه باز بودن پورت در فایروال. PUBLIC_URL فقط برای نمایش/دیباگ نگه داشته می‌شود.
if [ -z "$WORKER_URL" ] && [ -f "$CONF_FILE" ]; then
  WORKER_URL="$(grep -E '^WORKER_URL=' "$CONF_FILE" | cut -d= -f2- | tr -d '"' | tr -d ' \r\n')"
fi
if [ -z "$PUBLIC_URL" ] && [ -f "$CONF_FILE" ]; then
  PUBLIC_URL="$(grep -E '^PUBLIC_URL=' "$CONF_FILE" | cut -d= -f2- | tr -d '"' | tr -d ' \r\n')"
fi
{
  echo "# ساخته‌شده توسط guardian-agent-install.sh — دستی ویرایش نکن (600)"
  echo "GUARDIAN_TOKEN=\"$TOKEN\""
  echo "WORKER_URL=\"$WORKER_URL\""
  echo "PORT=\"$PORT\""
  echo "PUBLIC_URL=\"$PUBLIC_URL\""
  echo "STATE_DIR=\"$STATE_DIR\""
} > "$CONF_FILE"
chmod 600 "$CONF_FILE"

# ---- systemd ----
cat > "$UNIT" <<EOF
[Unit]
Description=Guardian agent (Cloudflare offload + relay replacement)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$CONF_FILE
ExecStart=$NODE_BIN $INSTALL_DIR/guardian-agent.js
Restart=always
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable -q guardian-agent.service
systemctl restart guardian-agent.service
sleep 2

# ---- خودتست (چند تلاش با فاصله — بالا آمدن node ممکن است کمی طول بکشد) ----
ping=""
for _try in 1 2 3 4 5; do
  ping="$(curl -fsSL --max-time 5 "http://127.0.0.1:${PORT}/ping" || true)"
  if echo "$ping" | grep -q '"ok":true'; then break; fi
  sleep 2
done
echo "$ping" | grep -q '"ok":true' || fail "Service did not start - logs: journalctl -u guardian-agent -n 50"
ver="$(echo "$ping" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)"

# ---- heartbeat آزمایشی (اگر WORKER_URL داده شده) ----
if [ -n "$WORKER_URL" ]; then
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 -X POST "$WORKER_URL/guardian" \
    -H 'Content-Type: application/json' \
    -d "{\"token\":\"$TOKEN\",\"action\":\"heartbeat\",\"version\":\"$ver\",\"ts\":$(date +%s)000,\"public_url\":\"$PUBLIC_URL\"}" || true)"
  if [ "$code" = "200" ]; then info "heartbeat reached the worker"
  else info "Worker did not answer (http=$code) - check WORKER_URL and token; local agent is healthy."; fi
else
  info "WORKER_URL is empty - heartbeat disabled; set it in $CONF_FILE and restart the service."
fi

echo "======================================================================"
echo " Guardian agent installed - version $ver"
echo " Port         : $PORT (local only - no firewall rule needed)"
echo " Token source : $TOKEN_SRC"
if [ "$TOKEN_SRC" = "generated" ]; then
  echo " Next: register this token in the bot: Versions -> Install agent -> Register agent token."
fi
echo " Local test   : curl http://127.0.0.1:${PORT}/ping"
echo " The agent calls the worker itself (outbound HTTPS); within a minute"
echo " the bot shows it as connected - no public IP or open port needed."
echo " Log          : journalctl -u guardian-agent -f"
echo "======================================================================"

echo ""
echo -e "${TCOL}AGENT TOKEN: ${TOKEN}${RST}"
echo "If this token differs from the one stored in the bot, register it: Versions -> Install agent -> Register agent token."
