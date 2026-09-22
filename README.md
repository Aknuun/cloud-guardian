# srv-relay — رلهٔ SSH نگهبان ابری

رلهٔ سبک SSH برای ربات [نگهبان ابری](https://github.com/Aknuun/cloud-guardian) — مانیتور سرور و اجرای دستور از راه دور.

## نصب پیش‌فرض (یک خط)

```bash
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"
```

*   پورت پیش‌فرض: `8788`
*   توکن رندوم ساخته میشه و تو ترمینال نمایش داده میشه
*   سرویس `srv-relay` به صورت `systemd` نصب و فعال میشه
*   نیاز به `root` داره

> همین دستور بدون هیچ متغیر اضافی با مقادیر پیش‌فرض نصب میکنه — همون چیزی که میخواستید.

## نصب با توکن/پورت دلخواه

```bash
# توکن دلخواه
sudo SRV_RELAY_TOKEN=mySecret123 bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"

# پورت دلخواه
sudo SRV_RELAY_PORT=9090 bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"

# هر دو
sudo SRV_RELAY_TOKEN=mySecret123 SRV_RELAY_PORT=9090 bash srv-relay-install.sh
```

## بررسی وضعیت

```bash
systemctl status srv-relay --no-pager
curl http://127.0.0.1:8788/ping
cat /opt/srv-relay/config.json | grep token
sudo ufw allow 8788/tcp  # اگر فایروال داری
```

## ثبت در ربات

بعد از نصب، توکن نمایش داده شده رو کپی کن:
ربات تلگرام → `⚙️ تنظیمات → 🖥 سرورها → 🔧 تنظیم رله` → آدرس `http://IP:8788` + توکن رو وارد کن

## آپدیت

```bash
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"
```
توکن قبلی حفظ میشه (مگر `SRV_RELAY_TOKEN` جدید بدی).

## حذف

```bash
sudo systemctl disable --now srv-relay
sudo rm -rf /opt/srv-relay /etc/systemd/system/srv-relay.service
sudo systemctl daemon-reload
```

---

## Default Install (English)

```bash
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian-relay/main/srv-relay-install.sh)"
```
*   Default port `8788`, random token, `systemd` service, requires `root`.
