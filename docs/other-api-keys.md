# راهنمای کوتاه: گرفتن توکنهای اختیاری 🔑

اینها **اختیاری** هستند و فقط وقتی نیاز میشوند که آن قابلیت را در ربات استفاده کنید. همه را داخل خودِ ربات ثبت میکنید (نه در نصب)، به جز `WHOIS_API_KEY` و `GLOBALPING_TOKEN` که بهصورت متغیر روی ورکر گذاشته میشوند.

---

## هتزنر (برای ساخت و مدیریت سرور)

۱. وارد `console.hetzner.cloud` شوید.
۲. روی پروژه → **Security** → **API Tokens** → **Generate API Token**.
۳. سطح دسترسی **Read & Write** و نام دلخواه بگذارید.
۴. توکن را کپی کنید و در ربات: «🇩🇪 هتزنر» ← افزودن اکانت.

> توکن فقط یکبار نمایش داده میشود؛ آن را امن نگه دارید.

---

## لینود (برای ساخت و مدیریت سرور)

۱. وارد `cloud.linode.com` شوید.
۲. منوی **My Profile** → **API Tokens** → **Create a Personal Access Token**.
۳. دسترسی `Read/Write` برای همه را تیک بزنید.
۴. توکن را کپی کنید و در ربات: «🟢 لینود» ← افزودن اکانت.

---

## آروان (دامنه/DNS و سرور ابری)

۱. از لینک ماشین‌یوزرها وارد شو → **New User** (نام لاتین کوچک، ۵ تا ۱۰۰ کاراکتر):
   https://panel.arvancloud.ir/profile/iam/machine-users
۲. کلید نمایش داده شده (`Apikey XXXX-…`) را همان لحظه کپی کنید (فقط یکبار نمایش داده می‌شود).
۳. از لینک مدیریت منابع وارد شو، روی **+** اول بزن تا وارد میز کار (Workspace) بشوی، بعد **قانون دسترسی** (Policy) تعریف کن — همهٔ قوانین، یا حداقل **CDN** و **Cloud Server**:
   https://panel.arvancloud.ir/profile/iam/resource-management
۴. کلید را در ربات: «🇮🇷 آروان» ← اکانت‌ها ← افزودن اکانت ثبت کنید.

> اگر عملی با خطای دسترسی (403) رد شد، یعنی Policy لازم به ماشین‌یوزر داده نشده؛ از پنل آروان دسترسی را اضافه کنید.

---

## WHOSERVICE (انقضای دامنه)

برای «🗓 مانیتور انقضای دامنه» یک کلید از یکی از سرویسهای WHOIS میخواهید؛ مثلاً:

- **Whois XML API**: `whois.whoisxmlapi.com` → API Keys → Token
- **WhoisFreaks**: `whoisfreaks.com` → API Key
- **RDAP دولتی**: نیازی به کلید ندارد (`WHOIS_API_PROVIDER=rdap`)

در دشبورد کلادفلر روی ورکر، متغیرهای `WHOIS_API_KEY` و (در صورت نیاز) `WHOIS_API_PROVIDER` را اضافه کنید.

---

## Global Ping (چک IPv4)

برای امکاناتی که به «آیپی سراسری» نیاز دارند (چک اینکه آیپی از نقاط دنیا در دسترس است):

۱. `globalping.io` → ثبتنام.
۲. `Manage API Access` → یک Token بسازید.
۳. در ورکر، متغیر `GLOBALPING_TOKEN` را اضافه کنید.

---

## ایجنت سرور (SSH و مانیتور)

اتصال SSH و مانیتور سرورها از طریق ایجنت روی سرور خودتان انجام می‌شود (رله جداگانه حذف شده است).

نصب با یک دستور (توکن را از ربات بگیرید):

```bash
sudo GUARDIAN_TOKEN="..." WORKER_URL="https://....workers.dev" bash -c "$(curl -fsSL https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent-install.sh)"
```

بعد توکن را در ربات ثبت کنید.