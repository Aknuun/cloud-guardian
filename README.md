# نگهبان ابری ☁️

مدیریت کامل **دامنه، DNS، پنل پاسارگارد و سرورها (هتزنر، لینود، آروان)** در تلگرام — روی **Cloudflare Workers** اجرا میشود؛ بدون هیچ سروری برای اجرا (فقط یک سرور برای نصب لازم است).

---
https://github.com/Aknuun/cloud-guardian/blob/main/docs/IMG_20260920_000345_938.jpg
https://github.com/Aknuun/cloud-guardian/blob/main/docs/IMG_20260920_000345_594.jpg
https://github.com/Aknuun/cloud-guardian/blob/main/docs/IMG_20260920_000345_118.jpg
## خلاصه

- ربات تلگرامی که رکوردهای DNS را روی کلادفلر و آروان مدیریت میکند
- تشخیص دامنهٔ فیلترشده و تعویض خودکار آن در پاسارگارد
- مانیتور SSL، انقضای دامنه و پایش منابع سرور (از طریق «رلهٔ SSH»)
- اعلان و یادآوری خودکار قطعی نود داخل تلگرام
- مدیریت هتزنر، لینود و آروان

---
## پیشنیازها

| # | پیشنیاز | راهنما |
|---|---|---|
| ۱ | یک حساب کلادفلر برای مدیریت DNS | داشبورد › My Profile › API Tokens |
| ۲ | توکن API کلادفلر | [آموزش تصویری ساخت توکن](docs/cloudflare-api-token.md) |
| ۳ | توکن ربات تلگرام | از [@BotFather](https://t.me/BotFather) با دستور `/newbot` |
| ۴ | شناسهٔ عددی تلگرام خودتان (برای مدیر) | از [@userinfobot](https://t.me/userinfobot) |

**دسترسیهای موردنیاز توکن کلادفلر:**

**Account · Workers Scripts · Edit**
**Account · Workers KV Storage · Edit**
**Zone · DNS · Edit**
**Zone · Zone Settings · Edit**
**Zone · Cache Purge · Purge**
**Account · Workers Subdomain · Edit**

---
## نصب (از طریق سرور روی کلادفلر)
روی سرور موقت لینوکسی اجرا کنید:

```bash
bash -c "$(curl -sL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/install.sh)"
```

اسکریپت به ترتیب میپرسد: توکن API کلادفلر → نام ورکر (پیشفرض `cloud-guardian`) → توکن ربات → شناسهٔ مدیر. تنظیمات در `~/.cloud-guardian/config.json` (فقط شما) ذخیره میشود.
> سرور فقط «ابزار نصب» است. بعد از نصب میتوانید آن را حذف کنید؛ ربات مستقل کار میکند.

---
## آپدیت خودکار 🔄
نسخهٔ جدید از گیتهاب که منتشر شود، ربات **خودش را** آپدیت میکند.

ساخته شده با ❤️ برای کسانی که کار روزمرهٔ مدیریت دامنه و سرور را به تلگرام سپردهاند.
