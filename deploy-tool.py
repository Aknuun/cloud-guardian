#!/usr/bin/env python3
# ============================================================
# ابزار نصب/آپدیت نگهبان ابری روی کلادفلر ورکر
# config.json کنار همین فایل باید باشد (نوشته‌شده توسط install.sh).
#
# STANDARD CONTRACT (backend + UI):
#   exit codes: 0 = OK, 1 = required permissions/operation failed,
#               2 = only optional permissions are missing.
#   UI: همهٔ پیام‌های کاربر از T() می‌آیند (fa/en)؛ رنگ‌ها فقط از
#       GREEN/RED/YELLOW/BLUE/BOLD/RST؛ نمادها ثابت: ✅ ❌ ⚠️ ℹ️ ⏭️ ❔.
#   جدول check همیشه ۸ ردیف ثابت دارد؛ ستون وسط (│) با رنگ وضعیت
#   رنگی می‌شود؛ وبهوک هرگز روی دیپلوی ناقص ست نمی‌شود.
# ============================================================
import sys, os, json, time, uuid, argparse, urllib.request, urllib.error, urllib.parse

API = "https://api.cloudflare.com/client/v4"
here = os.path.dirname(os.path.abspath(__file__))
CFG = os.path.join(here, "config.json")
SCHEDULES = [
    "* * * * *",
    "0 9 * * *",
    "*/10 * * * *",
    "*/5 * * * *",
]


# ============================================================
# i18n — language comes from env CG_LANG (fa|en), default fa
# ============================================================
LANG = os.environ.get("CG_LANG", "fa")
if LANG not in ("fa", "en"):
    LANG = "fa"

_S = {
    "kv_routing":      ("KV namespaces API روی این اکانت routable نیست: %s…", "KV namespaces API is not routable on this account: %s…"),
    "kv_reuse":        ("استفادهٔ مجدد از KV namespace موجود: %s", "Reusing an existing KV namespace: %s"),
    "kv_stale":        ("KV id ذخیره‌شده در این اکانت نیست (%s) — یکی تازه می‌سازم.", "Stored KV id not found in this account (%s) — creating a fresh one."),
    "kv_create_fail":  (
        'ساخت KV از API ممکن نشد و namespace آماده‌ای هم روی اکانت پیدا نشد.\n'
        'در داشبورد کلادفلر (Workers & Pages → KV → Create a namespace) یکی بساز و id آن را '
        'در ~/.cloud-guardian/config.json در فیلد "kv_namespace_id" بگذار و دوباره اجرا کن.',
        'Could not create KV via the API and no existing namespace was found.\n'
        'Create one in the Cloudflare dashboard (Workers & Pages → KV → Create a namespace), put its id '
        'in ~/.cloud-guardian/config.json under "kv_namespace_id" and run again.'),
    "workers_dev_warn": ("[!] هشدار workers.dev: %s", "[!] workers.dev WARN: %s"),
    "sched_min":       ("[!] فقط کرون‌های ضروری نصب شدند (سقف پلن رایگان): %s", "[!] Only essential crons installed (free plan limit): %s"),
    "sched_warn":      ("[!] هشدار کرون: %s", "[!] SCHED WARN: %s"),
    "cfg_not_found":   ("config.json پیدا نشد: %s", "config.json not found: %s"),
    "token_missing":   ("token در config نیست.", "token is missing from config."),
    "worker_missing":  ("worker.js کنار config نیست.", "worker.js not found next to config."),
    "status_active":   ("فعال", "active"),
    "status_invalid":  ("نامعتبر — %s", "invalid — %s"),
    "s_present":       ("✅ موجود", "✅ present"),
    "s_absent":        ("❌ پیدا نشد", "❌ not found"),
    "subdomain_none":  ("زیردامنه پیدا نشد", "subdomain not found"),
    # status labels — standard keys so status output is bilingual like check
    "st_worker":      ("ورکر", "worker"),
    "st_account":     ("اکانت", "account"),
    "st_token":       ("توکن", "token"),
    "st_script":      ("اسکریپت", "script"),
    "st_schedules":   ("زمان‌بندها", "schedules"),
    "st_workers_dev": ("workers.dev", "workers.dev"),
    "st_kv":          ("بایندینگ KV", "KV binding"),
    "st_version":     ("ورژن", "version"),
    "st_webhook":     ("وبهوک", "webhook"),
    "st_webhook_err": ("خطای وبهوک", "webhook err"),
    "s_disabled":      ("(غیرفعال) %s", "(disabled) %s"),
    "tg_err":          ("خطا در ارتباط با تلگرام", "error talking to Telegram"),
    "ck_token":        ("توکن کلادفلر", "Cloudflare token"),
    "ck_account":      ("دسترسی به اکانت (Accounts Read)", "Account access (Accounts Read)"),
        "ck_accts":        ("اکانت: %s", "accounts: %s"),
    "ck_noacct":       ("بدون اکانت", "no account"),
    "ck_nozone":       ("بدون زون", "no zones"),
    "ck_allzones":     ("هر %d زون تست‌شده شکست خورد", "all %d tested zones failed"),
    "wh_deleted":      ("حذف وبهوک تلگرام", "Telegram webhook deleted"),
    "wh_fail":         ("حذف وبهوک ناموفق (نادیده گرفته شد)", "Failed to delete webhook (ignored)"),
    "worker_deleted":  ("✅ ورکر حذف شد: %s", "✅ Worker deleted: %s"),
    "worker_del_fail": ("❌ حذف ورکر: %s", "❌ Failed to delete worker: %s"),
    "kv_deleted":      ("✅ KV namespace حذف شد: %s", "✅ KV namespace deleted: %s"),
    "kv_del_fail":     ("⚠️ حذف KV ناموفق: %s", "⚠️ Failed to delete KV: %s"),
    "kv_kept":         ("ℹ️ KV namespace نگه داشته شد: %s", "ℹ️ KV namespace kept: %s"),
    "kv_not_found":    ("❌ KV namespace پیدا نشد.", "❌ KV namespace not found."),
    "saved_kv":        ("✅ ذخیره شد در KV: %s", "✅ Saved to KV: %s"),
    "deleted_kv":      ("✅ حذف شد از KV: %s", "✅ Deleted from KV: %s"),
    "relay_no_token":  ("⚠️ توکن رله داده نشده؛ فقط آدرس ذخیره شد.", "⚠️ No relay token given; only the URL was saved."),
    "relay_registered":("✅ رله در ربات ثبت شد: %s", "✅ Relay registered in the bot: %s"),
    "zones_fail":      ("❌ گرفتن زون‌ها ناموفق: %s", "❌ Failed to fetch zones: %s"),
    "no_active_zone":  ("❌ هیچ زون فعالی پیدا نشد.", "❌ No active zone found."),
    "domain_not_found":("❌ دامنه پیدا نشد در اکانت: %s", "❌ Domain not found in the account: %s"),
    "rec_ok":          ("ℹ️ رکورد از قبل درست است: %s", "ℹ️ Record already correct: %s"),
    "rec_updated":     ("✅ رکورد به‌روزرسانی شد: %s", "✅ Record updated: %s"),
    "rec_created":     ("✅ رکورد ساخته شد: %s", "✅ Record created: %s"),
    "rec_update_fail": ("❌ به‌روزرسانی رکورد: %s", "❌ Failed to update record: %s"),
    "rec_create_fail": ("❌ ساخت رکورد: %s", "❌ Failed to create record: %s"),
    "ck_token_inactive": ("توکن غیرفعال/نامعتبر است", "Token is inactive/invalid"),
    "ck_acct_analytics": ("Account · Account Analytics · Read", "Account · Account Analytics · Read"),
    "ck_opt_title": ("دسترسی‌های اختیاری ناقص است", "Optional permissions are incomplete"),
    "ck_opt_body": ("این‌ها برای نصب لازم نیستند ولی قابلیت‌هایشان کار نمی‌کند:", "These are not needed for install, but their features will not work:"),
    "ck_workers":      ("Account · Workers Scripts · Edit", "Account · Workers Scripts · Edit"),
    "ck_kv":           ("Account · Workers KV Storage · Edit", "Account · Workers KV Storage · Edit"),
    "ck_zones_perm":   ("Zone · DNS · Edit (دسترسی به زون‌ها)", "Zone · DNS · Edit (zone access)"),
    "ck_dns":          ("Zone · DNS · Edit (خواندن رکوردها)", "Zone · DNS · Edit (record read)"),
    "ck_settings":     ("Zone · Zone Settings · Edit", "Zone · Zone Settings · Edit"),
    "ck_mail":         ("Zone · Email Routing Rules · Edit (ایمیل‌ها و صندوق ورودی)", "Zone · Email Routing Rules · Edit (emails and inbox)"),
    "ck_mailaddr":     ("Account · Email Routing Addresses · Edit (مقصدهای ایمیل)", "Account · Email Routing Addresses · Edit (email destinations)"),
    "ck_cache_note":   ("Cache Purge بدون اجرای واقعی قابل تست نیست؛ مطمئن شو دسترسی Purge را هم داده‌ای.",
                        "Cache Purge cannot be tested without a real purge; make sure the Purge permission is granted."),
    "ck_fail_title":   ("دسترسی‌های توکن کلادفلر ناقص است", "Cloudflare token permissions are incomplete"),
    "ck_fail_body":    ("این دسترسی‌ها درست نیستند یا کم هستند:", "These permissions are missing or wrong:"),
    "ck_fail_fix":     ("توکن را در این لینک ویرایش/بساز و ۸ دسترسی لازم را بده:", "Edit/create the token here and grant the 8 required permissions:"),
    "ck_tbl_need":     ("نیازمندی (۸ دسترسی)", "Requirement (8 permissions)"),
    "ck_tbl_have":     ("وضعیت توکن تو", "Your token"),
    "ck_zone_analytics": ("Zone · Analytics · Read (اختیاری)", "Zone · Analytics · Read (optional)"),
    "ck_notest":       ("تست خودکار ندارد", "not auto-testable"),
    "ck_skip":         ("تست نشد (زون در دسترس نیست)", "skipped (no zone reachable)"),
    "ck_hint_settings": ("← DNS داری ولی Zone Settings نداری؛ احتمالاً به‌جای «Zone Settings» گزینهٔ اشتباهی (مثل DNS) را زده‌ای",
                         "← DNS works but Zone Settings does not; you probably picked the wrong group (e.g. DNS) instead of «Zone Settings»"),
    "ck_hint_zonenone": ("← حتی لیست زون‌ها هم باز نشد؛ به توکن هیچ دسترسی Zone نداده‌ای",
                         "← even zone listing failed; the token has no Zone permissions at all"),
    "ck_fix_path":     ("مسیر اصلاح: داشبورد ← Profile ← API Tokens ← ویرایش همین توکن ← بخش Permissions",
                        "Fix path: dashboard ← Profile ← API Tokens ← edit this token ← Permissions section"),
}


_USE_COLOR = sys.stdout.isatty() and not os.environ.get("NO_COLOR")
def _col(c):
    return c if _USE_COLOR else ""
GREEN = _col("\033[1;32m"); RED = _col("\033[1;31m"); YELLOW = _col("\033[1;33m")
BLUE = _col("\033[1;34m"); UB = _col("\033[4m"); BOLD = _col("\033[1m"); RST = _col("\033[0m")


def T(key, *args):
    v = _S.get(key)
    if not v:
        return key
    s = v[0] if LANG == "fa" else v[1]
    return s % args if args else s


# UI helpers — mirror install.sh b()/ok()/warn()/err() so both scripts
# share the same visual language (color + symbol + destination).
def _ui(prefix, msg, err_stream=False):
    print(prefix + " " + msg, file=(sys.stderr if err_stream else sys.stdout))

def ui_info(msg): _ui(BLUE + "[*]" + RST, msg)
def ui_ok(msg): _ui(GREEN + "✅" + RST, msg)
def ui_warn(msg): _ui(YELLOW + "⚠️" + RST, msg)
def ui_err(msg): _ui(RED + "❌" + RST, msg, err_stream=True)


def _short_err(detail, limit=80):
    """Table cells stay one short line: turn raw API error blobs into
    'code XXXX — message' instead of multi-line JSON dumps."""
    if not detail:
        return ""
    d = detail
    try:
        parsed = json.loads(detail) if isinstance(detail, str) and detail.strip().startswith(("[", "{")) else None
    except Exception:
        parsed = None
    if isinstance(parsed, list) and parsed and isinstance(parsed[0], dict):
        first = parsed[0]
        code = first.get("code", "")
        msg = first.get("message", "")
        d = f"code {code} — {msg}" if code or msg else detail
    d = " ".join(str(d).split())
    return d if len(d) <= limit else d[:limit - 1] + "…"


SEP = "، " if LANG == "fa" else ", "


def req(tok, method, url, body=None, ctype=None):
    r = urllib.request.Request(url, method=method, data=body)
    r.add_header("Authorization", "Bearer " + tok)
    if ctype:
        r.add_header("Content-Type", ctype)
    try:
        with urllib.request.urlopen(r, timeout=120) as resp:
            return resp.status, resp.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace")


def json_ok(st, out):
    try:
        d = json.loads(out)
    except Exception:
        return None, f"bad json HTTP {st}: {out[:200]}"
    if not d.get("success"):
        return None, json.dumps(d.get("errors"), ensure_ascii=False)[:300]
    return d.get("result"), None


def module_body(code, meta):
    B = "----negahban" + uuid.uuid4().hex
    j = json.dumps(meta, ensure_ascii=False).encode("utf-8")
    body = b"\r\n".join([
        b"--" + B.encode(), b'Content-Disposition: form-data; name="metadata"',
        b"Content-Type: application/json", b"", j,
        b"--" + B.encode(), b'Content-Disposition: form-data; name="worker.js"; filename="worker.js"',
        b"Content-Type: application/javascript+module", b"", code,
        b"--" + B.encode() + b"--", b"",
    ])
    return body, B


def _kv_is_routing(err):
    return ("7003" in (err or "")) or ("7000" in (err or ""))


def find_kv_by_title(tok, acc, title):
    try:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/storage/kv/namespaces?per_page=100", None, "application/json")
        res, err = json_ok(st, out)
        if err:
            return None
        for n in res or []:
            if n.get("title") == title:
                return n.get("id")
    except Exception:
        pass
    return None


def find_kv_by_title(tok, acc, title):
    try:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/storage/kv/namespaces?per_page=100", None, "application/json")
        res, err = json_ok(st, out)
        if err:
            return None
        for n in res or []:
            if n.get("title") == title:
                return n.get("id")
    except Exception:
        pass
    return None


def find_existing_kv_id(tok, acc):
    try:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/workers/scripts")
        res, err = json_ok(st, out)
        if err:
            return None
        for w in res or []:
            wid = w.get("id") or w.get("name")
            if not wid:
                continue
            st2, out2 = req(tok, "GET", f"{API}/accounts/{acc}/workers/scripts/{urllib.parse.quote(wid)}/settings")
            res2, err2 = json_ok(st2, out2)
            if err2:
                continue
            for b in (res2 or {}).get("bindings") or []:
                if b.get("type") == "kv_namespace" and b.get("namespace_id"):
                    return b["namespace_id"]
    except Exception:
        pass
    return None


def _kv_exists(tok, acc, kv_id):
    try:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/storage/kv/namespaces/{kv_id}")
        res, err = json_ok(st, out)
        return not err
    except Exception:
        return False


def ensure_kv(cfg, tok):
    if cfg.get("kv_namespace_id"):
        # ممکن است id از اکانت دیگری مانده باشد (config قدیمی) — بررسی کن، وگرنه تازه بساز
        if _kv_exists(tok, cfg.get("account_id"), cfg["kv_namespace_id"]):
            return cfg["kv_namespace_id"], None
        ui_info(T("kv_stale", cfg["kv_namespace_id"]))
        cfg.pop("kv_namespace_id", None)
    body = json.dumps({"title": f"{cfg['worker']}-kv"}).encode("utf-8")
    st, out = req(tok, "POST", f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces", body, "application/json")
    res, err = json_ok(st, out)
    if err and "10014" in err:
        found = find_kv_by_title(tok, cfg["account_id"], f"{cfg['worker']}-kv")
        if found:
            cfg["kv_namespace_id"] = found
            with open(CFG, "w") as f:
                json.dump(cfg, f, ensure_ascii=False, indent=2)
            print("[*] " + T("kv_reuse", found))
            return found, None
    if err and "10014" in err:
        found = find_kv_by_title(tok, cfg["account_id"], f"{cfg['worker']}-kv")
        if found:
            cfg["kv_namespace_id"] = found
            with open(CFG, "w") as f:
                json.dump(cfg, f, ensure_ascii=False, indent=2)
            print("[*] " + T("kv_reuse", found))
            return found, None
    if err and _kv_is_routing(err):
        found = find_existing_kv_id(tok, cfg["account_id"])
        if found:
            cfg["kv_namespace_id"] = found
            with open(CFG, "w") as f:
                json.dump(cfg, f, ensure_ascii=False, indent=2)
            ui_info(T("kv_routing", err[:120]))
            ui_info(T("kv_reuse", found))
            return found, None
        return None, (err + " — " + T("kv_create_fail"))
    if err:
        return None, err
    cfg["kv_namespace_id"] = res["id"]
    with open(CFG, "w") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)
    return res["id"], None


def bindings(cfg, kv_id):
    accs = cfg.get("cf_accounts") or [{"name": "main", "token": cfg["token"]}]
    return [
        {"type": "kv_namespace", "name": "BOT_KV", "namespace_id": kv_id},
        {"type": "plain_text", "name": "BOT_TOKEN", "text": cfg["bot_token"]},
        {"type": "plain_text", "name": "ADMIN_ID", "text": str(cfg["admin_id"])},
        {"type": "plain_text", "name": "CF_ACCOUNTS", "text": json.dumps(accs, ensure_ascii=False)},
        {"type": "plain_text", "name": "WORKER_ACCOUNT_ID", "text": cfg["account_id"]},
        {"type": "plain_text", "name": "WORKER_NAME", "text": cfg["worker"]},
        # شناسهٔ KV برای اینکه آپدیت خودکارِ داخل ورکر (maybeSelfUpdate) بتواند
        # بایندینگ‌ها را بدون حدس زدن بازسازی کند.
        {"type": "plain_text", "name": "KV_ID", "text": kv_id},
    ]


def set_schedules(cfg, tok, crons):
    st, out = req(tok, "PUT", f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}/schedules",
                  json.dumps([{"cron": c} for c in crons]).encode("utf-8"), "application/json")
    return json_ok(st, out)


def enable_workers_dev(cfg, tok):
    st, out = req(tok, "POST", f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}/subdomain",
                  json.dumps({"enabled": True, "previews_enabled": True}).encode("utf-8"), "application/json")
    res, err = json_ok(st, out)
    if err:
        print(T("workers_dev_warn", err), file=sys.stderr)


def install(cfg, tok, code):
    kv_id, err = ensure_kv(cfg, tok)
    if err:
        return f"KV ERR: {err}"
    meta = {
        "main_module": "worker.js",
        "modules": [{"name": "worker.js"}],
        "compatibility_date": cfg.get("compatibility_date", "2024-11-01"),
        "usage_model": cfg.get("usage_model", "standard"),
        "bindings": bindings(cfg, kv_id),
    }
    body, B = module_body(code, meta)
    st, out = req(tok, "PUT", f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}",
                  body, f"multipart/form-data; boundary={B}")
    res, err = json_ok(st, out)
    if err:
        return f"SCRIPT ERR: {err}"
    enable_workers_dev(cfg, tok)
    res, err = set_schedules(cfg, tok, SCHEDULES)
    # بلافاصله بعد از آپلود اسکریپت، این اندپوینت ممکن است موقتاً 10026 بدهد
    for _ in range(3):
        if not err or "10026" not in err:
            break
        time.sleep(2)
        res, err = set_schedules(cfg, tok, SCHEDULES)
    if err and "10072" in err:
        # سقف کرون‌های پلن رایگان در کل اکانت پر شده؛ حداقل کرون‌های ضروری را نصب کن
        for subset in (["*/10 * * * *", "*/5 * * * *"], ["*/10 * * * *"], ["*/5 * * * *"]):
            res, err = set_schedules(cfg, tok, subset)
            if not err:
                print(T("sched_min", SEP.join(subset)), file=sys.stderr)
                break
    if err:
        print(T("sched_warn", err), file=sys.stderr)
    return None


def update(cfg, tok, code):
    # ⚠️ بایندینگ‌ها را از config بازسازی کن و با همان API نصب (PUT) دیپلوی کن.
    # API جدید (‎/settings و ‎/versions) بایندینگ خالی برمی‌گرداند؛ اگر همان را
    # بفرستیم ورکر بدون BOT_KV/BOT_TOKEN بالا می‌آید و ربات برای همیشه می‌میرد.
    kv_id = cfg.get("kv_namespace_id") or find_kv_id(cfg, tok)
    if not kv_id:
        return "KV ERR: kv_namespace_id در config نیست و پیدا هم نشد."
    cfg["kv_namespace_id"] = kv_id
    with open(CFG, "w") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)
    meta = {
        "main_module": "worker.js",
        "modules": [{"name": "worker.js"}],
        "compatibility_date": cfg.get("compatibility_date", "2024-11-01"),
        "usage_model": cfg.get("usage_model", "standard"),
        "bindings": bindings(cfg, kv_id),
    }
    body, B = module_body(code, meta)
    st, out = req(tok, "PUT", f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}",
                  body, f"multipart/form-data; boundary={B}")
    res, err = json_ok(st, out)
    if err:
        return f"SCRIPT ERR: {err}"
    enable_workers_dev(cfg, tok)
    # PUT ممکن است زمان‌بندها را پاک کند — برگردان (مثل install)
    res, err = set_schedules(cfg, tok, SCHEDULES)
    # بلافاصله بعد از آپلود اسکریپت، این اندپوینت ممکن است موقتاً 10026 بدهد
    for _ in range(3):
        if not err or "10026" not in err:
            break
        time.sleep(2)
        res, err = set_schedules(cfg, tok, SCHEDULES)
    if err and "10072" in err:
        # سقف کرون‌های پلن رایگان در کل اکانت پر شده؛ حداقل کرون‌های ضروری را نصب کن
        for subset in (["*/10 * * * *", "*/5 * * * *"], ["*/10 * * * *"], ["*/5 * * * *"]):
            res, err = set_schedules(cfg, tok, subset)
            if not err:
                print(T("sched_min", SEP.join(subset)), file=sys.stderr)
                break
    if err:
        print(T("sched_warn", err), file=sys.stderr)
    return None


def load_cfg(require_token=True):
    if not os.path.exists(CFG):
        print(T("cfg_not_found", CFG), file=sys.stderr)
        sys.exit(1)
    with open(CFG) as f:
        cfg = json.load(f)
    if require_token and not cfg.get("token"):
        print(T("token_missing"), file=sys.stderr)
        sys.exit(1)
    return cfg


def load_worker_code():
    wpath = os.path.join(here, "worker.js")
    if not os.path.exists(wpath):
        print(T("worker_missing"), file=sys.stderr)
        sys.exit(1)
    with open(wpath, "rb") as f:
        return f.read()


def find_kv_id(cfg, tok):
    if cfg.get("kv_namespace_id"):
        return cfg["kv_namespace_id"]
    base = f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}"
    st, out = req(tok, "GET", base + "/settings")
    s, err = json_ok(st, out)
    if not err:
        for b in (s or {}).get("bindings") or []:
            if b.get("type") == "kv_namespace" and b.get("namespace_id"):
                return b["namespace_id"]
    return None


def get_subdomain(cfg, tok):
    st, out = req(tok, "GET", f"{API}/accounts/{cfg['account_id']}/workers/subdomain")
    res, err = json_ok(st, out)
    return (res or {}).get("subdomain") if not err else None


def script_url(cfg, tok):
    sub = get_subdomain(cfg, tok)
    return f"https://{cfg['worker']}.{sub}.workers.dev" if sub else None


def get_schedules(cfg, tok):
    st, out = req(tok, "GET", f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}/schedules")
    res, err = json_ok(st, out)
    if err:
        return None, err
    return [s.get("cron") for s in (res or {}).get("schedules", [])], None


def cmd_status(cfg, tok):
    aid, name = cfg["account_id"], cfg["worker"]
    print(T("st_worker") + "     : " + name)
    print(T("st_account") + "    : " + str(aid))
    st, out = req(tok, "GET", f"{API}/user/tokens/verify")
    res, err = json_ok(st, out)
    print(T("st_token") + "      : " + (T("status_active") if (res and res.get("status") == "active") else T("status_invalid", str(err)[:120])))
    st, out = req(tok, "GET", f"{API}/accounts/{aid}/workers/scripts")
    res, err = json_ok(st, out)
    exists = None if err else any((w.get("id") == name) for w in res or [])
    if exists:
        print(T("st_script") + "     : " + T("s_present"))
    elif exists is False:
        print(T("st_script") + "     : " + T("s_absent"))
    else:
        print(T("st_script") + "     : ? " + str(err)[:150])
    if exists:
        crons, e = get_schedules(cfg, tok)
        print(T("st_schedules") + "  : " + ((SEP.join(crons) if crons else "—") if not e else "ERR " + str(e)[:120]))
    url = script_url(cfg, tok)
    if url:
        st, out = req(tok, "GET", f"{API}/accounts/{aid}/workers/scripts/{name}/subdomain")
        res, err = json_ok(st, out)
        enabled = bool((res or {}).get("enabled"))
        print(T("st_workers_dev") + ": " + (url if enabled else T("s_disabled", url)))
    else:
        print(T("st_workers_dev") + ": " + T("subdomain_none"))
    print(T("st_kv") + " : " + (find_kv_id(cfg, tok) or "—"))
    print(T("st_version") + "    : " + (str(cfg.get("version")) if cfg.get("version") else "?"))
    bt = cfg.get("bot_token")
    if bt:
        try:
            with urllib.request.urlopen(f"https://api.telegram.org/bot{bt}/getWebhookInfo", timeout=20) as r:
                d = json.loads(r.read().decode("utf-8", "replace"))
            info = d.get("result") or {}
            print(T("st_webhook") + "    : " + (info.get("url") or "—"))
            if info.get("last_error_message"):
                print(T("st_webhook_err") + ": " + str(info.get("last_error_message")))
        except Exception:
            print(T("st_webhook") + "    : " + T("tg_err"))


def _perm_summary(failures):
    if not failures:
        return
    print()
    print(RED + BOLD + "  ╭──────────────────────────────────────────────────────────╮" + RST)
    print(RED + BOLD + "  │" + RST + "  ⛔ " + BOLD + T("ck_fail_title") + RST)
    print(RED + BOLD + "  ╰──────────────────────────────────────────────────────────╯" + RST)
    print()
    print("  " + T("ck_fail_body"))
    for f in failures:
        print("    " + RED + "•" + RST + " " + f)
    print()
    print("  " + T("ck_fail_fix"))
    print("    " + BLUE + UB + "https://dash.cloudflare.com/profile/api-tokens" + RST)
    print("  " + YELLOW + "← " + T("ck_fix_path") + RST)
    print()


def check_acct_analytics(tok, acc):
    if not acc:
        return None, "no account"
    import datetime
    day = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    q = {"query": "{viewer{accounts(filter:{accountTag:\"%s\"}){workersInvocationsAdaptive(limit:1,filter:{datetime_geq:\"%sT00:00:30Z\"}){sum{requests}}}}}" % (acc, day)}
    try:
        st, out = req(tok, "POST", "https://api.cloudflare.com/client/v4/graphql", json.dumps(q).encode("utf-8"), "application/json")
        d = json.loads(out)
    except Exception as ex:
        return None, str(ex)[:120]
    if isinstance(d, dict) and d.get("data") and not d.get("errors"):
        return True, None
    err = ""
    try:
        errs = (d.get("errors") if isinstance(d, dict) else None) or []
        err = "; ".join([str(e.get("message", "")) for e in errs if isinstance(e, dict)][:2])
    except Exception:
        pass
    return None, (err or ("HTTP %s" % st))[:160]


def cmd_check(cfg, tok, show_box=True):
    failures = []
    opt_failures = []
    # exit codes: 0 = همه اوکی، ۲ = فقط اختیاری‌ها ناقص‌اند، ۱ = اجباری‌ها ناقص‌اند
    def line(good, critical, label, detail="", note="", optional=False):
        mark = (GREEN + "✅" + RST) if good else (RED + "❌" + RST)
        extra = (" — " + detail) if detail else ""
        if note:
            extra += " " + YELLOW + note + RST
        print(mark + " " + label + extra)
        if not good:
            if optional:
                opt_failures.append(label)
            elif critical:
                failures.append(label)

    st, out = req(tok, "GET", f"{API}/user/tokens/verify")
    res, err = json_ok(st, out)
    active = bool(res and res.get("status") == "active")
    line(active, True, T("ck_token"), "" if active else T("ck_token_inactive") + ((" " + str(err)[:120]) if err else ""))
    if not active:
        if show_box:
            _perm_summary(failures)
        return 1

    st, out = req(tok, "GET", f"{API}/accounts?per_page=50")
    res, err = json_ok(st, out)
    accts = [a.get("id") for a in (res or [])] if not err else []
    line(not err and bool(accts), True, T("ck_account"),
         (T("ck_accts", ", ".join([a for a in accts[:3] if a]))) if accts else str(err or T("ck_noacct"))[:160])

    # --- جمع‌آوری نتایج ۸ دسترسی، بعد چاپ یک جدول سه‌ستونه ---
    # هر رکورد: [label, ok|None(skip/untestable), optional, detail, hint]
    probes = {}
    acc = cfg.get("account_id") or (accts[0] if accts else "")
    if acc:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/workers/scripts")
        _, err = json_ok(st, out)
        probes["workers"] = (not err, "" if not err else str(err)[:160])
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/storage/kv/namespaces")
        _, err = json_ok(st, out)
        probes["kv"] = (not err, "" if not err else str(err)[:160])
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/email/routing/addresses?per_page=1")
        _, err = json_ok(st, out)
        probes["mailaddr"] = (not err, "" if not err else str(err)[:160])
        ok7, err7 = check_acct_analytics(tok, acc)
        probes["acct"] = (bool(ok7), "" if ok7 else str(err7 or "")[:160])

    zones_ok = dns_ok = False
    st, out = req(tok, "GET", f"{API}/zones?per_page=50")
    res, err = json_ok(st, out)
    if err:
        line(False, True, T("ck_zones_perm"), str(err)[:160])
        probes["zones_fail"] = True
    else:
        zones = [z for z in (res or []) if z.get("name")]
        if not zones:
            line(True, False, T("ck_zones_perm"), "", T("ck_nozone"))
        else:
            zones_ok = True
            # چند زون اول را امتحان کن: اولی ممکن است pending/حذف‌شده یا خارج از اسکوپ توکن باشد.
            # فقط وقتی همه شکست خوردند، دسترسی ناقص حساب کن.
            tried = [z for z in zones[:5] if z.get("id")]
            dns_ok, dns_err, dns_zone = False, "", ""
            set_ok, set_err, set_zone = False, "", ""
            mail_ok, mail_err = False, ""
            for z in tried:
                if not dns_ok:
                    st, out = req(tok, "GET", f"{API}/zones/{z['id']}/dns_records?per_page=1")
                    _, e = json_ok(st, out)
                    if not e:
                        dns_ok, dns_zone = True, z["name"]
                    else:
                        dns_err = str(e)[:160]
                if not set_ok:
                    st, out = req(tok, "GET", f"{API}/zones/{z['id']}/settings")
                    _, e = json_ok(st, out)
                    if not e:
                        set_ok, set_zone = True, z["name"]
                    else:
                        set_err = str(e)[:160]
                if not mail_ok:
                    st, out = req(tok, "GET", f"{API}/zones/{z['id']}/email/routing")
                    _, e = json_ok(st, out)
                    if not e:
                        mail_ok = True
                    else:
                        mail_err = str(e)[:160]
                if dns_ok and set_ok and mail_ok:
                    break
            n = len(tried)
            probes["dns"] = ((True, "", "")
                             if dns_ok else
                             (False, dns_err or T("ck_allzones", n), ""))
            hint = T("ck_hint_settings") if (dns_ok and not set_ok) else ""
            probes["settings"] = (((True, "", "")
                                   if set_ok else
                                   (False, set_err or T("ck_allzones", n), hint)))
            probes["mail"] = ((mail_ok, "", "")
                              if mail_ok else
                              (False, mail_err or T("ck_allzones", n), ""))

    # ترتیب ثابت جدول: ۴ اجباری بعد ۴ اختیاری
    order = [
        ("ck_workers", "workers", False),
        ("ck_kv", "kv", False),
        ("ck_dns", "dns", False),
        ("ck_settings", "settings", False),
        ("ck_mail", "mail", True),
        ("ck_mailaddr", "mailaddr", True),
        ("ck_zone_analytics", None, True),
        ("ck_acct_analytics", "acct", True),
    ]
    print()
    MARKS = {"ok": GREEN + "✅" + RST, "bad": RED + "❌" + RST,
             "opt": YELLOW + "⚠️" + RST, "skip": BLUE + "⏭️" + RST,
             "unknown": YELLOW + "❔" + RST}
    DIVS = {"ok": GREEN, "bad": RED, "opt": YELLOW, "skip": BLUE, "unknown": YELLOW}

    def _split3(label):
        # "Scope · Resource · Perm (note)" -> aligned columns like the
        # token guide (1) Account · Workers Scripts · Edit ...
        parts = [p.strip() for p in label.split("·")]
        if len(parts) >= 3:
            return parts[0], " · ".join(parts[1:-1]), parts[-1]
        if len(parts) == 2:
            return parts[0], parts[1], ""
        return label, "", ""

    # Pass 1: statuses (same contract as before).
    rows = []
    for num, (key, pkey, optional) in enumerate(order, start=1):
        label = T(key)
        if pkey is None:
            stt, detail, hint = "unknown", T("ck_notest"), ""
        elif pkey not in probes:
            stt, detail, hint = "skip", T("ck_skip"), (T("ck_hint_zonenone") if probes.get("zones_fail") else "")
        else:
            p = probes[pkey]
            ok, detail = p[0], p[1]
            hint = p[2] if len(p) > 2 else ""
            stt = "ok" if ok else ("opt" if optional else "bad")
        if stt == "ok":
            pass
        elif stt in ("opt", "skip", "unknown"):
            if stt == "opt":
                opt_failures.append(label)
        else:
            failures.append(label)
        scope, resource, perm = _split3(label)
        rows.append((num, scope, resource, perm, stt, detail, hint))
    W1 = max(len(r[1]) for r in rows)
    W2 = max(len(r[2]) for r in rows)
    W3 = max(len(r[3]) for r in rows)
    W = len("8) ") + W1 + len(" · ") + W2 + len(" · ") + W3
    print("  " + BOLD + T("ck_tbl_need").ljust(W) + RST + " " + BOLD + "│" + RST + " " + BOLD + T("ck_tbl_have") + RST)
    print("  " + "─" * W + "─┼─" + "─" * 14)
    # Pass 2: aligned columns (padding on raw text, color around it).
    for num, scope, resource, perm, stt, detail, hint in rows:
        div = DIVS[stt] + "│" + RST
        # Standard: table cells stay short — no multi-line API blobs.
        # Optional/skipped/untestable rows show the mark only; required
        # failures show a short 'code XXXX — message' hint.
        if stt in ("opt", "skip", "unknown"):
            extra = ""
        elif stt == "bad":
            extra = (" — " + _short_err(detail)) if detail else ""
        else:
            extra = (" — " + detail) if detail else ""
        cols = (BLUE + f"{num})" + RST + " "
                + BOLD + scope + RST + " " * (W1 - len(scope))
                + " · " + resource + " " * (W2 - len(resource))
                + " · " + perm + " " * (W3 - len(perm)))
        print("  " + cols + " " + div + " " + MARKS[stt] + extra)
        if hint:
            print("  " + " " * W + " " + div + " " + YELLOW + hint + RST)

    if show_box:
        _perm_summary(failures)
        if opt_failures:
            print()
            print(YELLOW + BOLD + "  ── " + T("ck_opt_title") + RST)
            print("  " + T("ck_opt_body"))
            for f in opt_failures:
                print("    " + YELLOW + "•" + RST + " " + f)
            print()
    return 1 if failures else (2 if opt_failures else 0)


def cmd_uninstall(cfg, tok, keep_kv=False, keep_webhook=False):
    aid, name = cfg["account_id"], cfg["worker"]
    rc = 0
    bt = cfg.get("bot_token")
    if bt and not keep_webhook:
        try:
            data = urllib.parse.urlencode({"drop_pending_updates": "true"}).encode("utf-8")
            r = urllib.request.Request(f"https://api.telegram.org/bot{bt}/deleteWebhook", data=data)
            with urllib.request.urlopen(r, timeout=20) as resp:
                d = json.loads(resp.read().decode("utf-8", "replace"))
            print(("✅ " if d.get("ok") else "⚠️ ") + T("wh_deleted"))
        except Exception:
            print("⚠️ " + T("wh_fail"))
    st, out = req(tok, "DELETE", f"{API}/accounts/{aid}/workers/scripts/{name}")
    _, err = json_ok(st, out)
    if err:
        print(T("worker_del_fail", str(err)[:200]))
        rc = 1
    else:
        print(T("worker_deleted", name))
    kv = find_kv_id(cfg, tok)
    if kv and not keep_kv:
        st, out = req(tok, "DELETE", f"{API}/accounts/{aid}/storage/kv/namespaces/{kv}")
        _, err = json_ok(st, out)
        print(T("kv_del_fail", str(err)[:200]) if err else T("kv_deleted", kv))
    elif kv:
        print(T("kv_kept", kv))
    return rc


def cmd_set_kv(cfg, tok, key, value):
    kv = find_kv_id(cfg, tok)
    if not kv:
        print(T("kv_not_found"), file=sys.stderr)
        return 1
    url = f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces/{kv}/values/{urllib.parse.quote(key)}"
    st, out = req(tok, "PUT", url, value.encode("utf-8"), "text/plain")
    _, _e = json_ok(st, out)
    if _e:
        ui_err(str(_e)[:200])
        return 1
    print(T("saved_kv", key))
    return 0


def cmd_del_kv(cfg, tok, key):
    kv = find_kv_id(cfg, tok)
    if not kv:
        print(T("kv_not_found"), file=sys.stderr)
        return 1
    url = f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces/{kv}/values/{urllib.parse.quote(key)}"
    st, out = req(tok, "DELETE", url)
    _, _e = json_ok(st, out)
    if _e:
        ui_err(str(_e)[:200])
        return 1
    print(T("deleted_kv", key))
    return 0


def cmd_relay_register(cfg, tok, address, token, domain="", port=8788, dry_run=False):
    if address.startswith("http://") or address.startswith("https://"):
        url = address.rstrip("/")
        if dry_run:
            print(f"[dry-run] KV: srv_relay_url={url} , srv_relay_token=***")
            return 0
        rc = cmd_set_kv(cfg, tok, "srv_relay_url", url)
        if token:
            rc |= cmd_set_kv(cfg, tok, "srv_relay_token", token)
        else:
            print("⚠️ " + T("relay_no_token"))
        print(T("relay_registered", url))
        return rc

    ip = address
    st, out = req(tok, "GET", f"{API}/zones?per_page=50")
    res, err = json_ok(st, out)
    if err:
        print(T("zones_fail", str(err)[:150]), file=sys.stderr)
        return 1
    zones = [z for z in (res or []) if z.get("status") == "active" and z.get("name")]
    zones.sort(key=lambda z: z["name"])
    if not zones:
        print(T("no_active_zone"), file=sys.stderr)
        return 1
    zone = None
    if domain:
        for z in zones:
            if domain == z["name"] or domain.endswith("." + z["name"]):
                zone = z
                break
        if not zone:
            print(T("domain_not_found", domain), file=sys.stderr)
            return 1
    else:
        zone = zones[0]
    rec_name = f"rel.{zone['name']}"

    if dry_run:
        print(f"[dry-run] A {rec_name} → {ip} (proxied=false, ttl=120)")
        print(f"[dry-run] KV: srv_relay_url=http://{rec_name}:{port} , srv_relay_token=***")
        return 0

    st, out = req(tok, "GET", f"{API}/zones/{zone['id']}/dns_records?type=A&name={urllib.parse.quote(rec_name)}")
    res, err = json_ok(st, out)
    rec = (res or [None])[0] if (res and not err) else None
    if rec and rec.get("content") == ip and not rec.get("proxied"):
        print(T("rec_ok", rec_name))
    elif rec:
        body = json.dumps({"type": "A", "name": rec_name, "content": ip, "ttl": 120, "proxied": False}).encode("utf-8")
        st, out = req(tok, "PUT", f"{API}/zones/{zone['id']}/dns_records/{rec['id']}", body, "application/json")
        _, err = json_ok(st, out)
        if err:
            print(T("rec_update_fail", str(err)[:150]), file=sys.stderr)
            return 1
        print(T("rec_updated", rec_name))
    else:
        body = json.dumps({"type": "A", "name": rec_name, "content": ip, "ttl": 120, "proxied": False}).encode("utf-8")
        st, out = req(tok, "POST", f"{API}/zones/{zone['id']}/dns_records", body, "application/json")
        _, err = json_ok(st, out)
        if err:
            print(T("rec_create_fail", str(err)[:150]), file=sys.stderr)
            return 1
        print(T("rec_created", rec_name))

    url = f"http://{rec_name}:{port}"
    rc = cmd_set_kv(cfg, tok, "srv_relay_url", url)
    if token:
        rc |= cmd_set_kv(cfg, tok, "srv_relay_token", token)
    else:
        print("⚠️ " + T("relay_no_token"))
    print(T("relay_registered", url))
    return rc


def build_parser():
    p = argparse.ArgumentParser(prog="deploy-tool.py", description="Cloud Guardian installer/manager on Cloudflare")
    sub = p.add_subparsers(dest="cmd")
    sub.add_parser("install", help="Full install/reinstall of the worker (KV + bindings + cron + workers.dev)")
    sub.add_parser("update", help="Update the worker from the local worker.js (keeps bindings)")
    sub.add_parser("status", help="Show worker, cron, KV and webhook status")
    c = sub.add_parser("check", help="Check Cloudflare token permissions")
    c.add_argument("--no-box", action="store_true", help="do not print the summary box")
    u = sub.add_parser("uninstall", help="Delete the worker (KV and webhook by default)")
    u.add_argument("--keep-kv", action="store_true", help="keep the KV namespace")
    u.add_argument("--keep-webhook", action="store_true", help="keep the Telegram webhook")
    s = sub.add_parser("set-kv", help="write a value into the bot KV")
    s.add_argument("key")
    s.add_argument("value")
    d = sub.add_parser("del-kv", help="delete a key from the bot KV")
    d.add_argument("key")
    r = sub.add_parser("relay-register", help="register the relay in the bot (address + token)")
    r.add_argument("address", help="relay server URL or IP")
    r.add_argument("--token", default=os.environ.get("SRV_RELAY_TOKEN", ""), help="relay token")
    r.add_argument("--domain", default="", help="zone domain to create rel.<domain> (optional)")
    r.add_argument("--port", type=int, default=8788, help="relay port (default 8788)")
    r.add_argument("--dry-run", action="store_true", help="print only, no changes")
    return p


def main():
    parser = build_parser()
    args = parser.parse_args()
    cmd = args.cmd
    if not cmd:
        parser.print_help()
        sys.exit(1)

    if cmd in ("install", "update"):
        cfg = load_cfg()
        code = load_worker_code()
        err = install(cfg, cfg["token"], code) if cmd == "install" else update(cfg, cfg["token"], code)
        if err:
            print(err)
            sys.exit(1)
        print("OK")
        return

    if cmd == "check":
        cfg = load_cfg(require_token=False)
        sys.exit(cmd_check(cfg, cfg.get("token", ""), show_box=not args.no_box))

    cfg = load_cfg()
    tok = cfg["token"]
    if cmd == "status":
        cmd_status(cfg, tok)
    elif cmd == "uninstall":
        sys.exit(cmd_uninstall(cfg, tok, keep_kv=args.keep_kv, keep_webhook=args.keep_webhook))
    elif cmd == "set-kv":
        sys.exit(cmd_set_kv(cfg, tok, args.key, args.value))
    elif cmd == "del-kv":
        sys.exit(cmd_del_kv(cfg, tok, args.key))
    elif cmd == "relay-register":
        sys.exit(cmd_relay_register(cfg, tok, args.address, args.token, args.domain, args.port, args.dry_run))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()