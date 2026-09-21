# آموزش متنی: ساخت توکن API کلادفلر

این آموزش، گامبهگام ساخت توکنی است که رباتِ «نگهبان ابری» برای مدیریت DNS و دیپلوی روی ورکر به آن نیاز دارد.

> 🔗 **لینک مستقیم ساخت توکن:** https://dash.cloudflare.com/profile/api-tokens

---

## مرحلهٔ ۱ — ورود به صفحهٔ توکنها

1. وارد داشبورد کلادفلر شوید: `https://dash.cloudflare.com`
2. از منوی بالای سمت راست روی **My Profile** کلیک کنید.
3. از منوی کناری **API Tokens** را بزنید — یا مستقیم: https://dash.cloudflare.com/profile/api-tokens

---

## مرحلهٔ ۲ — ساخت توکن سفارشی

1. روی دکمهٔ **Create Token** کلیک کنید.
2. در پایین صفحه گزینهٔ **Create Custom Token** را بزنید.

---

## مرحلهٔ ۳ — تنظیم دسترسیها (مهمترین قسمت)

1. یک نام بگذارید، مثلاً `cloud-guardian`.
2. این ده دسترسی را اضافه کنید (روی **Add more** بعد از هر کدام):

| بخش | اسم | سطح | برای چه کاری در ربات |
|---|---|---|---|
| Account | Workers Scripts | Edit | دیپلوی، آپدیت خودکار ورکر و سابدامنهٔ `workers.dev` |
| Account | Workers KV Storage | Edit | ساخت KV ربات |
| Zone | DNS | Edit | مدیریت رکوردها و ساخت سابدامنهٔ رله |
| Zone | Zone Settings | Edit | مشاهده/تغییر تنظیمات هر زون (TLS/کش/…) |
| Zone | Cache Purge | Purge | دکمهٔ «پاککردن کش» زون |
| Zone | Email Routing Rules | Edit | فعال‌سازی ایمیل، قوانین فوروارد و دریافت در ربات (صندوق ورودی) |
| Account | Email Routing Addresses | Read | دیدن مقصدهای ایمیل (جیمیل‌های تأییدشده) |
| Account | Email Routing Addresses | Edit | افزودن مقصد ایمیل جدید |
| Zone | Analytics | Read | «📊 ترافیک ساب‌ها» (آمار ۷ روزهٔ هر ساب) |
| Account | Account Analytics | Read | «📊 ترافیک ساب‌ها» (پیش‌نیاز کوئری آنالیتیکس) |

> ⚠️ کار با سابدامنهٔ `workers.dev` جداگانه نیست؛ زیرمجموعهٔ دسترسی **Account → Workers Scripts → Edit** است و در فهرست پنل ابری به این اسم دیده نمیشود.
> ⚠️ این ده دسترسی، **دقیقاً** همانهایی هستند که ربات استفاده میکند. توکن قبلی‌تان فقط پنج‌تای اول را داشت؟ چون توکن قابل ویرایش نیست، یک توکن جدید با هر ده دسترسی بسازید و در ربات جایگزین کنید: کلودفلر ← 👤 اکانت‌ها ← حذف قدیمی + افزودن جدید.

3. بخش **Account Resources** را روی **Include → All accounts** بگذارید.
4. بخش **Zone Resources** را روی **Include → All zones** بگذارید.

---

## آزمون سریع

اگر میخواهید بدانید توکن سالم است، در ترمینال سرور:

```bash
curl -s -H "Authorization: Bearer توکن_شما" "https://api.cloudflare.com/client/v4/user/tokens/verify"
```

خروجی باید شامل `"success": true` باشد.