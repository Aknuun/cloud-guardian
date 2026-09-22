# آموزش متنی: ساخت توکن API کلادفلر

> 🆕 **آپدیت شهریور 1404 — فقط ۱ مجوز کافیست!** اسکریپت نصب با یک توکن Bootstrap (`Account → API Tokens → Edit`) بقیه 10 مجوز را خودش می‌سازد. دیگر لازم نیست یکی یکی فعال کنید.

این آموزش، گامبهگام ساخت توکن Bootstrap است که رباتِ «نگهبان ابری» برای ساخت خودکار توکن اصلی استفاده می‌کند.

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

## مرحلهٔ ۳ — تنظیم دسترسی Bootstrap (فقط ۱ مجوز)

1. یک نام بگذارید، مثلاً `cloud-guardian-bootstrap`.
2. **فقط این یک دسترسی را اضافه کنید:**
| بخش | اسم | سطح | توضیح |
|---|---|---|---|
| Account | API Tokens | Edit | اجازه ساخت توکن اصلی با 10 مجوز به اسکریپت |

3. بخش **Account Resources** را روی **Include → All accounts** بگذارید.
4. **Zone Resources را لازم نیست پر کنید** (خالی بماند).
5. `Continue → Create Token` و توکن `cfut_...` را کپی کنید و به اسکریپت نصب بدهید.

> ✅ اسکریپت با این توکن Bootstrap به صورت خودکار یک توکن اصلی می‌سازد که شامل این 10 مجوز است و آن را ذخیره می‌کند:
> `Workers Scripts/Edit`, `Workers KV/Edit`, `DNS/Edit`, `Zone Settings/Edit`, `Cache Purge/Purge`, `Email Routing Rules/Edit`, `Email Routing Addresses/Read`, `Email Routing Addresses/Edit`, `Analytics/Read`, `Account Analytics/Read`
> توکن Bootstrap بعد از نصب قابل حذف است (TTL پیشنهادی 10 دقیقه).

---

## آزمون سریع

اگر میخواهید بدانید توکن سالم است، در ترمینال سرور:

```bash
curl -s -H "Authorization: Bearer توکن_شما" "https://api.cloudflare.com/client/v4/user/tokens/verify"
```

خروجی باید شامل `"success": true` باشد.