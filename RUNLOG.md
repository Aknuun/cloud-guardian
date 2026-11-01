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

## v4.74.0 — UI group A (items 1, 2, 11)
- topbar primary action: new `components/PageAction.tsx` (context + `usePageAction`), `ProviderShell` renders the single primary next to the title; wired srv/rem/pg/mail/dom/hs; full-width `.cc-pagebar` removed (kept only for the empty server list).
- stat tiles: `note` support; empty ring replaced by "از N" where no real ratio (srv, dom).
- server list: status dot (up/down/unknown) + "آخرین بررسی …" from new per-server snapshot `srv_last:<id>` written by `srvmon.Run`, exposed as `last` in `/api/tools/srv`; `timeAgo()` helper in lib.
- fix found by e2e: `usePageAction` sat after early returns (React #310) — hooks moved to the top of each view.
- checks: tsc clean, biome 0 errors (20 warnings), build ok, playwright 17/17.

## v4.75.0 — UI group B (items 5, 8)
- Empty states: `Empty` gained `action` slot; srv/rem/dom empty lists now offer the next step (add server/reminder/cert/domain) instead of dead text.
- Errors: every tool's hard `if (err) return <Empty title="خطا" .../>` replaced with `ErrBox` + a working «تلاش دوباره» (`reload`); um got an explicit `load()`.
- Loading: full-page spinners in dom/hf/hfd/hook/mail/set/um replaced with `<Skeleton rows={5}/>` so layout height stays stable.
- Forms: `FormDialog` accepts `disabled`; add-server save stays disabled until a host is typed; numeric inputs got `inputMode="numeric"`.
- checks: tsc clean, biome 0 errors (20 warnings), build ok, playwright 17/17.

## v4.76.0 — UI group C (items 3, 4, 7, 9)
- type scale: name 15/600, sub 12, section 13/600, stat 26/600, dialog title 20/600, hints 12.
- contrast: `--ds-subtle` #96979e → #6b6d75 (≥4.5:1 on white).
- bidi: `unicode-bidi: plaintext` for `.cc-ltr`, `.cc-mono`, `.cc-num`, row subs — host/port/IDs no longer jump in RTL.
- topbar order: hamburger moved to the end of the bar (title sits at the start edge) in both ProviderShell and App.
- new e2e: every button on all 15 tool pages has an accessible name (aria-label/title/text).
- checks: tsc clean, biome 0 errors (174 warnings), build ok, playwright 18/18.

## v4.77.0 — UI group D (items 6, 10, 12)
- motion policy replaces the global kill-switch: animations stay off, only colour/background/border/opacity transition at 120ms; hover restored on sidebar items, rows, row buttons and menu items (no transform).
- mobile <420px: the 52–64px rail is gone entirely (drawer only), content uses full width, and the global search becomes an icon that opens a sheet with the same results.
- version chip removed from both topbars (the installed version is already shown in Settings).
- breadcrumb next to the page title for nested Hetzner pages (account / server, "clean IPs", "build server").
- checks: tsc clean, biome 0 errors, build ok, playwright 18/18.

## v4.77.1 — stat tiles fixed properly (item 2)
- `components/Stats.tsx` redesigned: `of` = real denominator → value + "از N" + ring only when the ratio is computable; tiles without a denominator are compact one-line cards; a lone zero renders "—" instead of "0".
- call sites updated: srv (با کلید/با رمز از کل), rem (در انتظار/انجامشده از کل), pg (فعال از مانیتورها), dom (نیاز به بررسی از گواهی‌ها).
- checks: tsc clean, build ok, 18/18 e2e.

## v4.89.0 — UI kit wired into every tool page (package A)
- `components/ui-kit.tsx` is now used everywhere it fits: `ListRow` replaces hand-built `.cc-trow` markup, `SectionHeader` replaces every inline `.cc-section-head`, `SettingsCard`/`SettingsRow` group the read-only status blocks (hub "این نصب", fo worker), `ChipGroup` picks the panel (hf/hfd), toggles host fields, and picks the add-source (srv).
- New CSS: `.cc-setcard*`, `.cc-setrow*`, `.cc-section-action`, `.cc-rowchev`, `button.cc-trow`; section counts now sit next to the title instead of being pushed to the far edge.
- `.cc-btnrow` is gone from markup and CSS; action groups use the existing `.cc-actions` card list or the header action slot.
- Pages touched: dom, hf, hfd, hook, mail, set, fo, hub, hs (HostStudio), srv, pg, cfcheck, HetznerIpsView.
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18.

## v4.90.0 — settings forms on SettingsCard (package B)
- In-page forms (hf, hfd, hook, um, dom SSL warning) moved from loose `.cc-form` blocks into a titled `SettingsCard`: one row per setting, the label on the right, the control on the left, units as `.cc-unit` next to the number («دقیقه»، «ساعت»، «شهر»، «بار»، «از ۴»، «GB», «روز»).
- Save + feedback live in the card footer: the primary button and «ذخیره شد» / the error text right next to it — no more page-top hint floating away from the form.
- Toggles use the same Switch row inside the card; hook saves each switch immediately, hf/hfd/um/dom save on the button.
- Dialog forms (server, mail, backups, monitor, host studio, cfcheck, arvan) stay on `FormDialog` — the dialog is already one card.
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18.

## v4.91.0 — last-run strip, monitor cards, filter counts, row menu (packages C/D)
- New `components/RunStrip.tsx`: one line between the toolbar and the list — a status dot, «آخرین اجرا: …» and an optional summary that turns red on trouble. Wired into hf (`lastSummary`), hfd (new `lastRun` from the server) and cfcheck (time + «N رکورد · M مشکل»).
- Monitor rows on pg are now `.cc-moncard` cards: status dot and name on top, then node counts (سالم / قطع in red when >0), «آخرین بررسی» from the newest node sample, and the poll interval. The server now returns `nodes`, `up`, `down` and `last` per monitor (webtools.go).
- FilterBar items carry an optional `count`: it shows as a small `.cc-fcount` pill in the menu and next to «فیلتر» on the trigger. Counts wired on pg, cfcheck and srv.
- srv gained a «فقط خراب‌ها» filter (servers whose last check failed) next to key/password.
- New `RowMenu` in ui-kit: a ⋯ button using the existing `.cc-menu` styles, so row action clusters stop eating the row width. Used on dom domain rows (استعلام/ثبت تاریخ/حذف) and pg panel cards (ویرایش/حذف).
- `timeAgo()` now returns Persian digits, so every «… دقیقه پیش» matches the rest of the UI; `.cc-chip` centers its text as an inline-flex pill.
- checks: tsc clean, biome 0 errors (27 warnings — the new menu scrim matches the existing FilterBar warning), build ok, playwright 18/18.

## v4.92.0 — global search removed, mobile pass, instant field chips
- The topbar search is gone for good: `GlobalSearch.tsx` deleted, the field and the mobile sheet removed from both shells, and every `.cc-gsearch*`/`.cc-gres*` rule dropped. Tool pages keep their own `FilterBar`; nothing else searches globally.
- hf/hfd field chips now answer instantly: the chip flips locally on tap, the API call confirms in the background, and the host list only reloads if the call fails. Previously every tap waited for two round-trips (`exc` + `hosts`) and a slow one felt like a lost tap.
- Mobile (390px) pass over all 15 pages: section counts sit next to their titles again (the stray `margin-inline-end:auto` on `.cc-section-head .cc-sub` pushed them away and forced action buttons onto a second line); pg panel actions no longer wrap the ⋯ menu above the node button.
- hs no longer crashes on a template without `segments` (preview, editor state, and the save validation all treat it as an empty list).
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18.

## v4.93.0 — single-column home with collapsible sections
- Home is one column again: the 2-col grid is gone, every group is its own card.
- Each section header is now a button with a caret; tapping it folds the list away. All sections start open, and the caret flips when the section is closed. The rows inside are unchanged.
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18.

## v4.94.0 — unslop pass: quiet chrome, phone-first
- Chrome is quieter: the FAB lost its brand-coloured glow (neutral soft shadow in both themes), stat tiles dropped the conic ring and became compact label/value tiles on phones, and «خروج کامل» left the mobile topbar — it now sits at the bottom of the drawer, above a hairline, outside the scrolling nav, so a misplaced thumb can no longer sign out.
- Drawer: nav gets its own scroll area (`overflow` on the list, sticky footer for profile + sign-out); the brand line no longer repeats «Cloud Guardian» when the version is unknown.
- `SectionHeader` was rewritten as title+count in one group with the action in its own span. On phones the header packs from the reading edge, so a short action sits next to the count and a long one wraps underneath instead of floating at the far side; the zone accordion header on cfcheck got the same treatment (title, count, caret at the end).
- Rows: `.cc-rowbtn` and RowMenu are 40px on touch screens (were 34), hs template preview/actions aligned to the reading edge, tabs 40px, `.cc-tplbtns` gap 6 with 36–40px buttons. hf host rows drop the empty address line, mail forward rows read «به …» instead of a bidi-flipped arrow, and the cfcheck monitor interval moved into its button label («مانیتور · هر ۱۵ دقیقه») instead of a clipped caption inside the action grid.
- SettingsCards stay inline on phones (label right, unit+input left) instead of stacking to full-width controls; RunStrip stacks the summary under the timestamp; monitor meta is 12px with real gaps.
- dom dropped the second «افزودن» button (the page action already exists), and hover tints no longer stick after a tap.
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18; every page reviewed at 390×844.

## v4.95.0 — denser, quieter controls (MoreBot pass)
- Controls got professional-sized: `--cc-btn-h` is 40px, the FAB is 50px, and `.cc-actions` is no longer a 2×54px card list — it is a wrap toolbar of 34px (36 on phones) outline buttons. Dialogs inherit the same row.
- Rows stopped reserving empty space: `.cc-trow` lost its forced 78px min-height (padding 10×12, radius 12, 6px gap), and destructive icon buttons are a quiet tint instead of a bordered red square.
- Settings cards gained a header action slot (`SettingsCard action={…}`): the master on/off switch now sits in the card title row on hf, hfd, hook, um and dom instead of taking a whole row.
- Numeric settings are a compact grid (`cc-setgrid`): two fields per row on phones, four on wide screens, label above a small input — hf/hfd/hook/um moved from stacked full-width rows to this.
- hf reads shorter: the intro is one sentence, a host shows one combined address/SNI/host line instead of three, and field chips are 28px.
- checks: tsc clean, biome 0 errors (26 warnings), build ok, playwright 18/18 (reviewed hf/hook/um/dom/srv/set/mail at 390 and 1280).
