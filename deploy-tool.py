#!/usr/bin/env python3
# ============================================================
# ابزار نصب/آپدیت نگهبان ابری روی کلادفلر ورکر
# config.json کنار همین فایل باید باشد (نوشته‌شده توسط install.sh).
# ============================================================
import sys, os, json, time, uuid, urllib.request, urllib.error, urllib.parse

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


def main():
    if len(sys.argv) < 2:
        print("usage: deploy-tool.py install|update")
        sys.exit(1)
    cmd = sys.argv[1]
    if not os.path.exists(CFG):
        print("config.json پیدا نشد.")
        sys.exit(1)
    with open(CFG) as f:
        cfg = json.load(f)
    tok = cfg.get("token", "")
    if not tok:
        print("token در config نیست.")
        sys.exit(1)
    wpath = os.path.join(here, "worker.js")
    if not os.path.exists(wpath):
        print("worker.js کنار config نیست.")
        sys.exit(1)
    with open(wpath, "rb") as f:
        code = f.read()
    if cmd == "install":
        err = install(cfg, tok, code)
    elif cmd == "update":
        err = update(cfg, tok, code)
    else:
        print(f"دستور نامشخص: {cmd}")
        sys.exit(1)
    if err:
        print(err)
        sys.exit(1)
    print("OK")


if __name__ == "__main__":
    main()