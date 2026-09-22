#!/usr/bin/env bash
# ============================================================
# srv-relay installer — رلهٔ واحد SSH برای ربات نگهبان ابری
# نصب/آپدیت:  sudo bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"
# یا با توکن دلخواه:  sudo SRV_RELAY_TOKEN=<token> bash srv-relay-install.sh
# • اگر srv-relay.js کنار اسکریپت باشد از همان استفاده می‌شود؛
#   وگرنه از مخزن گیت‌هاب دانلود می‌شود.
# • در آپدیت، توکنِ سرویسِ قبلی حفظ می‌شود (مگر توکن جدید بدهید).
# ============================================================
set -euo pipefail

APP_NAME="srv-relay"
APP_DIR="/opt/srv-relay"
SERVICE="${APP_NAME}.service"
SERVICE_FILE="/etc/systemd/system/${SERVICE}"
RAW_URLS=(
  "https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay.js"
  "https://api.github.com/repos/Aknuun/cloud-guardian-relay/contents/srv-relay.js?ref=main"
  "https://cdn.jsdelivr.net/gh/Aknuun/cloud-guardian-relay@main/srv-relay.js"
)
TOKEN="${SRV_RELAY_TOKEN:-}"

# --- منبع فایل رله: کنار اسکریپت یا دانلود از مخزن ---
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo ".")"
if [ -f "${SRC_DIR}/srv-relay.js" ]; then
  SRC_FILE="${SRC_DIR}/srv-relay.js"
  echo "[*] استفاده از فایل محلی: ${SRC_FILE}"
else
  SRC_FILE="/tmp/srv-relay.js.$(date +%s)"
  echo "[*] دانلود srv-relay.js از مخزن…"
  ok=0
  for u in "${RAW_URLS[@]}"; do
    # cache-buster: کش CDN را دور می‌زنیم تا همیشه آخرین نسخه نصب شود
    case "$u" in
      *api.github.com*) bu="$u&_=$(date +%s)" ;;
      *cdn.jsdelivr.net*) bu="$u" ;; # jsDelivr خودش کش می‌کند؛ با پارامتر نمی‌شکند
      *) bu="$u?_=$(date +%s)" ;;
    esac
    echo "    -> $u"
    curl -fsSL --max-time 30 "$bu" -o "$SRC_FILE" 2>/dev/null || true
    # اگر پاسخ JSON بود (مسیر API)، محتوای base64 را با node دیکد کن (روی سرور رله node هست)
    if head -c 1 "$SRC_FILE" 2>/dev/null | grep -q '{' && command -v node >/dev/null 2>&1; then
      node -e "const fs=require('fs');try{const j=JSON.parse(fs.readFileSync('$SRC_FILE','utf8'));fs.writeFileSync('$SRC_FILE.d',Buffer.from(j.content,'base64'));}catch(e){process.exit(1)}" && mv "$SRC_FILE.d" "$SRC_FILE" || true
    fi
    if grep -q "RELAY_REV" "$SRC_FILE" 2>/dev/null; then ok=1; break; fi
  done
  if [ "$ok" != "1" ]; then
    echo "[✗] دانلود نسخهٔ جدید ناموفق بود (کش CDN ممکن است قدیمی باشد)."
    echo "    چند دقیقه بعد دوباره اجرا کنید، یا فایل srv-relay.js را کنار اسکریپت بگذارید."
    exit 1
  fi
  echo "[✓] فایل جدید تأیید شد (شامل نشانگر rev)."
fi

mkdir -p "$APP_DIR"
cp "$SRC_FILE" "${APP_DIR}/srv-relay.js"
chmod 755 "${APP_DIR}/srv-relay.js"
[ "${SRC_FILE}" != "${APP_DIR}/srv-relay.js" ] && rm -f "$SRC_FILE" || true

# --- توکن پیش‌فرض: AknuunFixedToken2024 (env > سرویس قبلی > پیش‌فرض ثابت) ---
DEFAULT_TOKEN="AknuunFixedToken2024"
if [ -z "$TOKEN" ] && [ -f "$SERVICE_FILE" ]; then
  TOKEN=$(grep -oP 'Environment=SRV_RELAY_TOKEN=\K.*' "$SERVICE_FILE" 2>/dev/null | head -1 || true)
  if [ -n "$TOKEN" ]; then
    echo "[*] توکن سرویس قبلی حفظ شد."
  fi
fi
if [ -z "$TOKEN" ]; then
  TOKEN="$DEFAULT_TOKEN"
  echo "[*] توکن پیش‌فرض استفاده شد: $TOKEN"
fi

echo "[*] نصب پیش‌نیازها (openssh-client، sshpass برای احراز رمزی)…"
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq || true
  apt-get install -y -qq openssh-client sshpass curl >/dev/null 2>&1 || true
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y openssh-clients sshpass curl >/dev/null 2>&1 || true
elif command -v yum >/dev/null 2>&1; then
  yum install -y openssh-clients sshpass curl >/dev/null 2>&1 || true
fi

# --- اطمینان از وجود node با نسخهٔ کافی (رله به نود >= ۱۸ نیاز دارد) ---
NODE_MAJ=""
if command -v node >/dev/null 2>&1; then
  NODE_MAJ=$(node -v 2>/dev/null | tr -d 'v' | cut -d. -f1)
fi
if [ -z "${NODE_MAJ:-}" ] || ! [ "${NODE_MAJ:-0}" -ge 18 ] 2>/dev/null; then
  echo "[*] node پیدا نشد یا نسخهٔ آن قدیمی است؛ نصب Node.js 22 (NodeSource/Distro)…"
  if command -v apt-get >/dev/null 2>&1; then
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash - || { echo "[✗] نصب NodeSource ناموفق بود؛ node را دستی نصب کنید."; exit 1; }
    apt-get install -y nodejs || { echo "[✗] نصب nodejs ناموفق بود."; exit 1; }
  elif command -v dnf >/dev/null 2>&1; then
    dnf module reset nodejs -y >/dev/null 2>&1 || true
    dnf module enable nodejs:22 -y >/dev/null 2>&1 || true
    dnf install -y nodejs || { echo "[✗] نصب nodejs ناموفق بود."; exit 1; }
  elif command -v yum >/dev/null 2>&1; then
    curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - || { echo "[✗] نصب NodeSource ناموفق بود؛ node را دستی نصب کنید."; exit 1; }
    yum install -y nodejs || { echo "[✗] نصب nodejs ناموفق بود."; exit 1; }
  else
    echo "[✗] پکیج‌منیجر پشتیبانی‌شده پیدا نشد؛ لطفاً node (نسخهٔ ۱۸+) را دستی نصب کنید و دوباره اسکریپت را اجرا کنید."
    exit 1
  fi
fi
command -v node >/dev/null 2>&1 || { echo "[✗] node هنوز در PATH نیست؛ دستی نصب کنید."; exit 1; }
NODE_BIN="$(command -v node)"
echo "[✓] node: $("$NODE_BIN" --version) ($NODE_BIN)"

# --- آزادسازی پورت 8788 و حذف سرویس/پروسهٔ قبلی تا نصب جدید بدون خطا انجام شود ---
echo "[*] بررسی وضعیت قبلی روی پورت 8788…"
if systemctl list-unit-files 2>/dev/null | grep -q '^srv-relay.service'; then
  systemctl stop srv-relay >/dev/null 2>&1 || true
  systemctl disable srv-relay >/dev/null 2>&1 || true
  echo "    سرویس قبلی srv-relay متوقف شد."
fi
OLDPIDS=""
if command -v ss >/dev/null 2>&1; then
  OLDPIDS=$(ss -Hlpt "sport = :8788" 2>/dev/null | sed -n 's/.*pid=\([0-9]*\).*/\1/p' | sort -u || true)
fi
if [ -z "$OLDPIDS" ] && command -v lsof >/dev/null 2>&1; then
  OLDPIDS=$(lsof -ti :8788 2>/dev/null | sort -u || true)
fi
if [ -z "$OLDPIDS" ] && command -v fuser >/dev/null 2>&1; then
  OLDPIDS=$(fuser 8788/tcp 2>/dev/null | tr ' ' '\n' | sort -u || true)
fi
if [ -n "$OLDPIDS" ]; then
  for p in $OLDPIDS; do
    CMDLINE=""
    if [ -r "/proc/$p/cmdline" ]; then
      CMDLINE=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null || true)
    fi
    case "$CMDLINE" in
      *srv-relay.js*)
        kill -TERM "$p" 2>/dev/null || true
        echo "    پروسهٔ قدیمی (pid=$p) که 8788 را گرفته بود متوقف شد."
        ;;
      *)
        echo "    ⚠️ pid=$p روی پورت 8788 متعلق به یک خدمت دیگر است:"
        echo "       ${CMDLINE:-نامشخص}"
        echo "       اگر رله است، آن را متوقف کنید؛ وگرنه پورت/سرویس را دستی آزاد کنید."
        ;;
    esac
  done
  sleep 1
fi

echo "[*] ساخت سرویس systemd…"
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=srv-relay — unified SSH relay for Cloud Guardian Bot
After=network.target

[Service]
Type=simple
WorkingDirectory=${APP_DIR}
Environment=SRV_RELAY_PORT=8788
Environment=SRV_RELAY_TOKEN=${TOKEN}
ExecStart=${NODE_BIN} ${APP_DIR}/srv-relay.js
Restart=always
RestartSec=5
NoNewPrivileges=no
ProtectSystem=full
ProtectHome=yes

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null 2>&1 || true
systemctl restart "$SERVICE"

sleep 1
if systemctl is-active --quiet "$SERVICE"; then
  echo "[✓] srv-relay فعال شد روی پورت 8788"
  echo "    تست:  curl http://127.0.0.1:8788/ping"
  echo "    (توکن در فایل سرویس ذخیره شده: ${SERVICE_FILE})"
else
  echo "[✗] سرویس فعال نشد؛ لاگ:"
  journalctl -u "$SERVICE" -n 20 --no-pager
  exit 1
fi

# --- خودتست: مطمئن شو کد جدید (rev>=2) واقعاً فعال است ---
REV=$(curl -s -m 5 http://127.0.0.1:8788/ping | grep -o '"rev":[0-9]*' | grep -o '[0-9]*' || echo 0)
if [ "${REV:-0}" -ge 2 ] 2>/dev/null; then
  echo "[✓] بازبینی فعال: rev=$REV (کد جدید)"
else
  echo "[✗] هشدار: /ping بازبینی rev=$REV نشان می‌دهد — کد جدید فعال نشده!"
  echo "    دستی اجرا کنید:  systemctl restart srv-relay  و دوباره اسکریپت را بزنید."
  exit 1
fi
TEST=$(curl -s -m 25 -X POST http://127.0.0.1:8788/exec \
  -H "Content-Type: application/json" -H "X-SRV-Token: $TOKEN" \
  -d '{"host":"127.0.0.1","port":22,"user":"root","password":"x","command":"echo ok"}' || true)
if printf '%s' "$TEST" | grep -q 'resolve hostname /tmp/'; then
  echo "[✗] خودتست شکست: هنوز کد قدیمی در حال اجراست! سرویس را دستی ری‌استارت کنید:"
  echo "    systemctl restart srv-relay"
  exit 1
elif printf '%s' "$TEST" | grep -q 'Connection refused\|Connection timed out\|Permission denied\|password'; then
  echo "[✓] خودتست موفق: احراز رمزی درست کار می‌کند (ssh به هاست واقعی وصل شد)."
else
  echo "[i] خودتست (اطلاعاتی): ${TEST:0:200}"
fi

# ============================================================
# 🎉 راهنمای ثبت رله در ربات — بعد از نصب
# ============================================================
ORANGE='\033[1;38;5;208m'
RESET='\033[0m'
if [ ! -t 1 ] || [ "${TERM:-dumb}" = "dumb" ]; then
  ORANGE=''
  RESET=''
fi

# یافتن آیپی عمومی سرور
PUB_IP=""
for cmd in \
  "curl -s4 --max-time 8 https://ifconfig.me" \
  "curl -s4 --max-time 8 https://api.ipify.org" \
  "curl -s4 --max-time 8 https://ipinfo.io/ip"; do
  PUB_IP=$(eval "$cmd" 2>/dev/null | tr -d '[:space:]')
  [ -n "$PUB_IP" ] && break
done
[ -z "$PUB_IP" ] && PUB_IP="<آیپی-عمومی-سرور>"

LINE="=========================================================="
echo ""
echo "$LINE"
echo "  🎉 نصب رله کامل شد — حالا آن را در ربات ثبت کن:"
echo "$LINE"
echo ""
echo "  1️⃣  در ربات وارد شو: «🖥 سرورها ← ℹ️ راهنمای رله ← 🔧 تنظیم رله»"
echo ""
echo "  2️⃣  این متن را کپی کن و در ربات بفرست (آدرس رله):"
echo "      $(printf "${ORANGE}http://%s:8788${RESET}" "$PUB_IP")"
echo ""
echo "  3️⃣  توکن رله را کپی کن و در ربات بفرست:"
echo "      $(printf "${ORANGE}%s${RESET}" "$TOKEN")"
echo ""
echo "  ربات خودش اتصال و توکن را تست می‌کند و «✅ رله ثبت شد» می‌دهد."
echo "  🔥 قبل از ثبت، پورت 8788 باید از بیرون باز باشد:"
echo "      sudo ufw allow 8788/tcp   (اگر فایروال فعال است)"
echo ""
echo "  تست در همین سرور:  curl -s http://127.0.0.1:8788/ping"
echo "$LINE"
echo ""
