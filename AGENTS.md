# Cloud Guardian — قوانین عامل (AGENTS)

## UI / کیبورد تلگرام (اجباری)
1. **پیش‌فرض همهٔ دکمه‌ها خاکستری است** (بدون style). هرگز به دکمه‌های لیست/گزینه/ناوبری رنگ نده.
2. فقط **دکمه‌های عملیاتی** رنگ می‌گیرند:
   - `success` → ساخت/ارتقا/روشن‌کردن/شروع
   - `danger` → حذف/خاموش‌کردن/توقف/انصراف خطرناک
   - `primary` → حداکثر یک دکمهٔ اصلی در صفحه
3. **لیست گزینه‌ها دو ستونه** است (`gridBtns(items, 2)`): پلن‌ها، ایمیج‌ها، اسنپ‌شات‌ها، آی‌پی‌ها و…
   تک‌ستونه فقط برای برچسب‌های بلند مجاز است (ریجن‌ها، سرورها).
4. برچسب در حالت دوستونه کوتاه بماند (≤ ~۲۶ کاراکتر)؛ قیمت فشرده: `18.6M تومان` با `arvPriceShort`.
5. هر صفحه یک ردیف بازگشت/ناوبری داشته باشد.
6. داده‌ها فوری از کش نمایش داده شوند و در پس‌زمینه تازه‌سازی شوند (SWR).

## آروان (ECC)
- پیلود ساخت: `flavor_id`, `image_id`, `network_id` (**مفرد**), `security_groups:[{name:"<SG UUID>"}]`, `disk_size`, `enable_ipv4`.
- `/sizes` و `/securities` و `/networks` با query خطا می‌دهند؛ `per_page=100` ممنوع.
- `price_per_month` به **تومان** است. خطای 402 = کمبود اعتبار (`data.balance` / `data.required`).
- ساخت/شکار با `arvCreateServer` انجام شود؛ `flavorRef` فقط fallback است.

## انتشار
1. `server/VERSION` + `const Version` + `BOT_VERSION` + `RELEASE_NOTES` را یک واحد بالا ببر.
2. `go build ./... && go vet ./... && go test ./...`
3. commit + push + tag، سپس روی سرور: `REF=vX bash /tmp/cg-install.sh update`.
