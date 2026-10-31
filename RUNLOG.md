# RUNLOG — بررسی و رفع خودکار

## 4.73.3 — استودیوی نام هاست: priority تکراری در اعمال گروهی
- **مشکل:** `hsPlan` فقط در مسیر «انتخاب چند هاست» جایگاه‌ها را جایگشت می‌داد؛ در اعمال «روی یک گروه» هاست‌های همان گروه از صفر شماره می‌گرفتند و با priority بقیهٔ گروه‌ها تصادم می‌کرد.
- **رفع:** منطق تخصیص به `hsAssignPriorities(rows, inScope, partial, anchor, slot)` منتقل شد؛ هر اعمال جزئی (گروه یا انتخاب) اسلات‌های موجود همان هاست‌ها را جایگشت می‌دهد و به بیرون از دامنه دست نمی‌زند.
- **تست:** `TestHsAssignPriorities` (چیدمان کامل ۰/۱۰/۲۰ + جایگشت اسلات‌های گروه + بدون تداخل).
- **فایل‌ها:** `server/internal/router/webstudio.go`, `webstudio_test.go`

## 4.73.3 — Idempotency: کلید به مسیر و بدنه بسته شد
- **مشکل:** کلید فقط `uid|key` بود؛ همان کلید برای درخواست دیگری، پاسخ درخواست قبلی را برمی‌گرداند و عملیات جدید اجرا نمی‌شد.
- **رفع:** کلید مؤثر = `uid|key` و هش `sha256(path + body)`؛ در تکرار با بدنهٔ متفاوت → ۴۲۲ «این کلید قبلاً برای درخواست دیگری استفاده شده». بدنه برای هندلر بازگردانده می‌شود (سقف ۱MiB).
- **تست:** `TestWebIdemKeyBoundToBody` (مسیر متفاوت ۴۲۲، بدنه متفاوت ۴۲۲، تکرار یکسان replay و اجرای یک‌بارهٔ هندلر).
- **فایل‌ها:** `server/internal/router/webidem.go`, `webidem_test.go`

## 4.73.4 — worker پشتیبان: پین به تگ + امضای ed25519، و health روی HTTPS
- **مشکل ۱:** `fetchWorkerJS` فایل `worker.js` را از شاخهٔ `main` و بدون بررسی امضا می‌گرفت و ورکرِ دارای توکن ربات/KV را دیپلوی می‌کرد.
- **رفع ۱:** دانلود از همان تگ در حال اجرا (`worker.js` + `worker.js.sig`) و بررسی با `verifyScriptSig`/`updatePubKeyHex`؛ بدون امضا رد می‌شود مگر `CG_ALLOW_UNSIGNED_UPDATE=1`. `scripts/release.sh` حالا `worker.js.sig` را هم می‌سازد.
- **مشکل ۲:** health ورکر روی `http://…:8789/health` بود (قابل جعل/شنود).
- **رفع ۲:** `foPublicBase()` اگر `public_url` اچ‌تی‌تی‌پی‌اس نباشد، origin اچ‌تی‌تی‌پی‌اس پنل (`WebAppURL`) را ترجیح می‌دهد؛ پس دیپلوی بعدی ورکر روی `https://cg.aknuun.online/health` می‌نشیند.
- **تست:** `TestFoPublicBasePrefersHTTPS`.
- **نکته:** ورکرِ فعال فعلی تا دیپلوی مجدد از پنل، هنوز همان آدرس HTTP را probe می‌کند؛ تغییر در پیکربندی ذخیره‌شده کافی نیست چون `CG_HEALTH_URL` در زمان دیپلوی bind می‌شود.
- **فایل‌ها:** `server/internal/router/failover.go`, `failover_test.go`, `scripts/release.sh`

## 4.73.5 — زمان‌بند: هر کار goroutine مستقل + recover
- **مشکل:** `alertLoop` همهٔ کارها (پاکسازی بافرها، ndmon، srvmon، cf health، ssl، domexp، purge، sync، snapshot، backup) را سری و در یک goroutine و **بدون recover** اجرا می‌کرد؛ یک panic = مرگ کل ربات، و یک تماس کند = عقب‌افتادن بکاپ روزانه و بقیه.
- **رفع:** هلپر `every(name, d, fn)` ساخته شد (goroutine مستقل + `defer recover` + لاگ `loop <name> panic`)، و هر کار با دورهٔ خودش جدا شد: دقیقه‌ای (quota/alerts/um/ndmon/srvmon)، cf-health ۱م، hfd ۱م، hub ۵م، digest ۱ساعت/۲۴ساعت، ssl ۶ساعت، domexp ۲۴ساعت، purge ۱ساعت، sync ۱۰م، telemetry ۶ساعت، snapshot ۲۴ساعت، backup-push ۳۰م. (`hostfilterLoop` از قبل جدا بود.)
- **تست:** `TestEverySurvivesPanic` (بعد از panic، حلقه ادامه می‌دهد).
- **فایل‌ها:** `server/cmd/cg-server/main.go`, `main_test.go`

## 4.73.6 — شروع نرم زمان‌بندها (بدون انفجار پیام در بوت)
- **مشکل:** با جدا شدن حلقه‌ها، همه در لحظهٔ بوت یک‌بار اجرا می‌شدند و بک‌لاگ هشدارهای بافر‌شده یک‌جا ارسال شد (لاگ: `tg error: sendMessage Bad Request: chat not found` ×۱۲).
- **رفع:** `every` قبل از اولین اجرا به اندازهٔ دوره (حداکثر ۱ دقیقه) صبر می‌کند.
- **باز (ثبت‌شده):** یکی از شناسه‌های ادمین در لیست، چتی در ربات ندارد و ارسال به آن همیشه `chat not found` می‌دهد؛ نیاز به حذف آن ادمین یا `/start` از سمت او. (بدون تغییر داده انجام نشد.)
