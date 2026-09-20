#!/usr/bin/env python3
# ============================================================
# ابزار نصب/آپدیت نگهبان ابری روی کلادفلر ورکر
# config.json کنار همین فایل باید باشد (نوشته‌شده توسط install.sh).
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


def ensure_kv(cfg, tok):
    if cfg.get("kv_namespace_id"):
        return cfg["kv_namespace_id"], None
    body = json.dumps({"title": f"{cfg['worker']}-kv"}).encode("utf-8")
    st, out = req(tok, "POST", f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces", body, "application/json")
    res, err = json_ok(st, out)
    if err and _kv_is_routing(err):
        found = find_existing_kv_id(tok, cfg["account_id"])
        if found:
            cfg["kv_namespace_id"] = found
            with open(CFG, "w") as f:
                json.dump(cfg, f, ensure_ascii=False, indent=2)
            print(f"[*] KV namespaces API on this account is not routable: {err[:120]}…")
            print(f"[*] Reusing existing KV namespace from another worker: {found}")
            return found, None
        return None, (err + " — ساخت KV از API ممکن نشد و namespace آماده‌ای هم روی اکانت پیدا نشد.\n"
                      "در داشبورد کلادفلر (Workers & Pages → KV → Create a namespace) یکی بسازید و id آن را "
                      "در ~/.cloud-guardian/config.json در فیلد \"kv_namespace_id\" بگذارید و دوباره اجرا کنید.")
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
        print(f"[!] WORKERS.DEV WARN: {err}", file=sys.stderr)


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
                print(f"[!] فقط کرون‌های ضروری نصب شدند (سقف پلن رایگان): {'، '.join(subset)}", file=sys.stderr)
                break
    if err:
        print(f"[!] SCHED WARN: {err}", file=sys.stderr)
    return None


def update(cfg, tok, code):
    base = f"{API}/accounts/{cfg['account_id']}/workers/scripts/{cfg['worker']}"
    st, out = req(tok, "GET", base + "/settings")
    s, err = json_ok(st, out)
    if err:
        return f"SETTINGS ERR: {err}"
    meta = {
        "main_module": "worker.js",
        "modules": [{"name": "worker.js"}],
        "compatibility_date": s.get("compatibility_date", "2024-11-01"),
        "usage_model": s.get("usage_model", "standard"),
        "bindings": s.get("bindings", []),
    }
    body, B = module_body(code, meta)
    st, out = req(tok, "POST", base + "/versions", body, f"multipart/form-data; boundary={B}")
    res, err = json_ok(st, out)
    if err:
        return f"UPLOAD ERR: {err}"
    dep = json.dumps({"versions": [{"version_id": res["id"], "percentage": 100}]}).encode("utf-8")
    st, out = req(tok, "POST", base + "/deployments", dep, "application/json")
    res, err = json_ok(st, out)
    if err:
        return f"DEPLOY ERR: {err}"
    return None


def load_cfg(require_token=True):
    if not os.path.exists(CFG):
        print("config.json پیدا نشد: " + CFG, file=sys.stderr)
        sys.exit(1)
    with open(CFG) as f:
        cfg = json.load(f)
    if require_token and not cfg.get("token"):
        print("token در config نیست.", file=sys.stderr)
        sys.exit(1)
    return cfg


def load_worker_code():
    wpath = os.path.join(here, "worker.js")
    if not os.path.exists(wpath):
        print("worker.js کنار config نیست.", file=sys.stderr)
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
    print("worker     : " + name)
    print("account    : " + str(aid))
    st, out = req(tok, "GET", f"{API}/user/tokens/verify")
    res, err = json_ok(st, out)
    print("token      : " + ("فعال" if (res and res.get("status") == "active") else "نامعتبر — " + str(err)[:120]))
    st, out = req(tok, "GET", f"{API}/accounts/{aid}/workers/scripts")
    res, err = json_ok(st, out)
    exists = None if err else any((w.get("id") == name) for w in res or [])
    if exists:
        print("script     : ✅ موجود")
    elif exists is False:
        print("script     : ❌ پیدا نشد")
    else:
        print("script     : ? " + str(err)[:150])
    if exists:
        crons, e = get_schedules(cfg, tok)
        print("schedules  : " + (("، ".join(crons) if crons else "—") if not e else "ERR " + str(e)[:120]))
    url = script_url(cfg, tok)
    if url:
        st, out = req(tok, "GET", f"{API}/accounts/{aid}/workers/scripts/{name}/subdomain")
        res, err = json_ok(st, out)
        enabled = bool((res or {}).get("enabled"))
        print("workers.dev: " + (url if enabled else "(غیرفعال) " + url))
    else:
        print("workers.dev: زیردامنه پیدا نشد")
    print("KV binding : " + (find_kv_id(cfg, tok) or "—"))
    print("version    : " + (str(cfg.get("version")) if cfg.get("version") else "?"))
    bt = cfg.get("bot_token")
    if bt:
        try:
            with urllib.request.urlopen(f"https://api.telegram.org/bot{bt}/getWebhookInfo", timeout=20) as r:
                d = json.loads(r.read().decode("utf-8", "replace"))
            info = d.get("result") or {}
            print("webhook    : " + (info.get("url") or "—"))
            if info.get("last_error_message"):
                print("webhook err: " + str(info.get("last_error_message")))
        except Exception:
            print("webhook    : خطا در ارتباط با تلگرام")


def cmd_check(cfg, tok):
    def line(ok, label, detail=""):
        print(("✅ " if ok else "❌ ") + label + ((" — " + detail) if detail else ""))

    st, out = req(tok, "GET", f"{API}/user/tokens/verify")
    res, err = json_ok(st, out)
    line(bool(res and res.get("status") == "active"), "توکن فعال", "" if not err else str(err)[:120])

    st, out = req(tok, "GET", f"{API}/accounts?per_page=50")
    res, err = json_ok(st, out)
    accts = [a.get("id") for a in (res or [])] if not err else []
    line(not err and bool(accts), "Account · Workers Scripts (خواندن اکانت)",
         ("اکانت: " + ", ".join([a for a in accts[:3] if a])) if accts else str(err or "بدون اکانت")[:150])

    acc = cfg.get("account_id") or (accts[0] if accts else "")
    if acc:
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/workers/scripts")
        _, err = json_ok(st, out)
        line(not err, "Account · Workers Scripts · Edit", "" if not err else str(err)[:150])
        st, out = req(tok, "GET", f"{API}/accounts/{acc}/storage/kv/namespaces")
        _, err = json_ok(st, out)
        line(not err, "Account · Workers KV Storage · Edit", "" if not err else str(err)[:150])

    st, out = req(tok, "GET", f"{API}/zones?per_page=50")
    res, err = json_ok(st, out)
    zones = [z.get("name") for z in (res or [])] if not err else []
    line(not err and bool(zones), "Zone · DNS / Zone Settings / Cache Purge",
         ("زون‌ها: " + ", ".join([z for z in zones[:5] if z])) if zones else str(err or "بدون زون")[:150])


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
            print(("✅" if d.get("ok") else "⚠️") + " حذف وبهوک تلگرام")
        except Exception:
            print("⚠️ حذف وبهوک ناموفق (نادیده گرفته شد)")
    st, out = req(tok, "DELETE", f"{API}/accounts/{aid}/workers/scripts/{name}")
    _, err = json_ok(st, out)
    if err:
        print("❌ حذف ورکر: " + str(err)[:200])
        rc = 1
    else:
        print("✅ ورکر حذف شد: " + name)
    kv = find_kv_id(cfg, tok)
    if kv and not keep_kv:
        st, out = req(tok, "DELETE", f"{API}/accounts/{aid}/storage/kv/namespaces/{kv}")
        _, err = json_ok(st, out)
        print(("⚠️ حذف KV ناموفق: " + str(err)[:200]) if err else ("✅ KV namespace حذف شد: " + kv))
    elif kv:
        print("ℹ️ KV namespace نگه داشته شد: " + kv)
    return rc


def cmd_set_kv(cfg, tok, key, value):
    kv = find_kv_id(cfg, tok)
    if not kv:
        print("❌ KV namespace پیدا نشد.", file=sys.stderr)
        return 1
    url = f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces/{kv}/values/{urllib.parse.quote(key)}"
    st, out = req(tok, "PUT", url, value.encode("utf-8"), "text/plain")
    _, err = json_ok(st, out)
    if err:
        print("❌ " + str(err)[:200], file=sys.stderr)
        return 1
    print("✅ ذخیره شد در KV: " + key)
    return 0


def cmd_del_kv(cfg, tok, key):
    kv = find_kv_id(cfg, tok)
    if not kv:
        print("❌ KV namespace پیدا نشد.", file=sys.stderr)
        return 1
    url = f"{API}/accounts/{cfg['account_id']}/storage/kv/namespaces/{kv}/values/{urllib.parse.quote(key)}"
    st, out = req(tok, "DELETE", url)
    _, err = json_ok(st, out)
    if err:
        print("❌ " + str(err)[:200], file=sys.stderr)
        return 1
    print("✅ حذف شد از KV: " + key)
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
            print("⚠️ توکن رله داده نشده؛ فقط آدرس ذخیره شد.")
        print("✅ رله در ربات ثبت شد: " + url)
        return rc

    ip = address
    st, out = req(tok, "GET", f"{API}/zones?per_page=50")
    res, err = json_ok(st, out)
    if err:
        print("❌ گرفتن زون‌ها ناموفق: " + str(err)[:150], file=sys.stderr)
        return 1
    zones = [z for z in (res or []) if z.get("status") == "active" and z.get("name")]
    zones.sort(key=lambda z: z["name"])
    if not zones:
        print("❌ هیچ زون فعالی پیدا نشد.", file=sys.stderr)
        return 1
    zone = None
    if domain:
        for z in zones:
            if domain == z["name"] or domain.endswith("." + z["name"]):
                zone = z
                break
        if not zone:
            print("❌ دامنه پیدا نشد در اکانت: " + domain, file=sys.stderr)
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
        print("ℹ️ رکورد از قبل درست است: " + rec_name)
    elif rec:
        body = json.dumps({"type": "A", "name": rec_name, "content": ip, "ttl": 120, "proxied": False}).encode("utf-8")
        st, out = req(tok, "PUT", f"{API}/zones/{zone['id']}/dns_records/{rec['id']}", body, "application/json")
        _, err = json_ok(st, out)
        if err:
            print("❌ به‌روزرسانی رکورد: " + str(err)[:150], file=sys.stderr)
            return 1
        print("✅ رکورد به‌روزرسانی شد: " + rec_name)
    else:
        body = json.dumps({"type": "A", "name": rec_name, "content": ip, "ttl": 120, "proxied": False}).encode("utf-8")
        st, out = req(tok, "POST", f"{API}/zones/{zone['id']}/dns_records", body, "application/json")
        _, err = json_ok(st, out)
        if err:
            print("❌ ساخت رکورد: " + str(err)[:150], file=sys.stderr)
            return 1
        print("✅ رکورد ساخته شد: " + rec_name)

    url = f"http://{rec_name}:{port}"
    rc = cmd_set_kv(cfg, tok, "srv_relay_url", url)
    if token:
        rc |= cmd_set_kv(cfg, tok, "srv_relay_token", token)
    else:
        print("⚠️ توکن رله داده نشده؛ فقط آدرس ذخیره شد.")
    print("✅ رله در ربات ثبت شد: " + url)
    return rc


def build_parser():
    p = argparse.ArgumentParser(prog="deploy-tool.py", description="ابزار نصب/مدیریت نگهبان ابری روی کلادفلر")
    sub = p.add_subparsers(dest="cmd")
    sub.add_parser("install", help="نصب/بازنصب کامل ورکر (KV + bindings + cron + workers.dev)")
    sub.add_parser("update", help="آپدیت ورکر از فایل محلی worker.js (بدون تغییر bindings)")
    sub.add_parser("status", help="نمایش وضعیت ورکر، کرون‌ها، KV و وبهوک")
    sub.add_parser("check", help="بررسی دسترسی‌های توکن کلادفلر")
    u = sub.add_parser("uninstall", help="حذف ورکر (و به‌صورت پیش‌فرض KV و وبهوک)")
    u.add_argument("--keep-kv", action="store_true", help="KV namespace حذف نشود")
    u.add_argument("--keep-webhook", action="store_true", help="وبهوک تلگرام حذف نشود")
    s = sub.add_parser("set-kv", help="نوشتن یک مقدار در KV ربات")
    s.add_argument("key")
    s.add_argument("value")
    d = sub.add_parser("del-kv", help="حذف یک کلید از KV ربات")
    d.add_argument("key")
    r = sub.add_parser("relay-register", help="ثبت رله در ربات (آدرس/آی‌پی + توکن)")
    r.add_argument("address", help="URL یا آی‌پی سرور رله")
    r.add_argument("--token", default=os.environ.get("SRV_RELAY_TOKEN", ""), help="توکن رله")
    r.add_argument("--domain", default="", help="دامنهٔ زون برای ساخت rel.<domain> (اختیاری)")
    r.add_argument("--port", type=int, default=8788, help="پورت رله (پیش‌فرض 8788)")
    r.add_argument("--dry-run", action="store_true", help="فقط نمایش، بدون تغییر")
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
        cmd_check(cfg, cfg.get("token", ""))
        return

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