#!/usr/bin/env node
"use strict";
/* =====================================================================
 * guardian-runner — اجرای همان کد ورکر (worker.js همان تگ) داخل Node
 * ---------------------------------------------------------------------
 * چرا بدون بازنویسی؟ منطق هاست‌فیلتر ۶۰۰+ خط با ده‌ها helper و تست است؛
 * بازنویسی یعنی واگرایی و دردسر. رانر همان فایل را می‌گیرد و فقط دو چیز
 * را عوض می‌کند:
 *   ۱) import ممنوعهٔ cloudflare:sockets → stub صریح (فقط whois انقضا
 *      از آن استفاده می‌کند که اصلاً به ایجنت منتقل نشده).
 *   ۲) env.BOT_KV → FileKV: کلیدهای ران‌تایم فقط روی دیسک محلی،
 *      host_filter_cfg pull/push با ورکر، بقیه read-sync + write-journal.
 * جدول jobهای مجاز صریح است (allowlist) — هیچ تابع دیگری از راه دور
 * قابل اجرا نیست.
 * ===================================================================== */

const fs = require("node:fs");
const path = require("node:path");
const { pathToFileURL } = require("node:url");

const RUNNER_VERSION = "2.4.2";
// کلیدهایی که ایجنت اجازهٔ pull/push آن‌ها را دارد (مکمل allowlist ورکر)
// فاز ۴: کانفیگ پول نود، مانیتور مصرف، مانیتور سرور، دایجست پنل، SSL و انقضای دامنه
const SYNC_KEYS = [
  "accounts", "panels", "admins", "host_filter_cfg",
  "node_monitors", "usage_monitor_cfg", "srv_mon_cfg", "servers",
  "pghook_cfg", "ssl_monitor", "dom_expiry", "dom_expiry_cfg",
];
const RUNTIME_EXACT = new Set([
  "hosts_cache",
  "hf_progress",
  "host_filter_log",
  "host_filter_state",
  "host_filter_ip_notice",
  "host_filter_crash",
  // شمارنده‌های متریک ورکر: روی ایجنت فقط محلی می‌مانند و هرگز push نمی‌شوند
  // (وگرنه aggregate روزانهٔ ورکر را بازنویسی و خراب می‌کنند)
  "qw:",
  "qal:",
]);
function isRuntimeKey(key) {
  const k = String(key || "");
  if (RUNTIME_EXACT.has(k)) return true;
  // کش‌های CF (زون/رکورد) فقط محلی‌اند؛ push آن‌ها write بیهوده است (ورکر کش خودش را دارد)
  return k.startsWith("dnscache:") || k.startsWith("ipinfo:") || k.startsWith("qw:") || k.startsWith("qal:") || k.startsWith("cache:");
}

// ---------- HTTP کوچک به ورکر ----------
function httpJson(base, method, p, body, token, timeoutMs) {
  return new Promise((resolve) => {
    const data = body ? Buffer.from(JSON.stringify(body)) : null;
    const u = new URL(base.replace(/\/+$/, "") + p);
    const mod = u.protocol === "https:" ? require("node:https") : require("node:http");
    const req = mod.request(
      { hostname: u.hostname, port: u.port || (u.protocol === "https:" ? 443 : 80), path: u.pathname + u.search,
        method, headers: { "Content-Type": "application/json", "X-Guardian-Token": token || "",
          ...(data ? { "Content-Length": data.length } : {}) },
        timeout: timeoutMs || 15000 },
      (res) => {
        let raw = "";
        res.on("data", (c) => { raw += c; });
        res.on("end", () => {
          try { resolve({ ok: res.statusCode >= 200 && res.statusCode < 300, status: res.statusCode, data: raw ? JSON.parse(raw) : null }); }
          catch (e) { resolve({ ok: false, error: "bad_json" }); }
        });
      }
    );
    req.on("timeout", () => { req.destroy(); resolve({ ok: false, error: "timeout" }); });
    req.on("error", (e) => resolve({ ok: false, error: String((e && e.message) || e).slice(0, 120) }));
    if (data) req.write(data);
    req.end();
  });
}

// ---------- FileKV: رفتار حداقلی KV ورکر روی دیسک ----------
class FileKV {
  constructor(dir, journal) {
    this.dir = dir;
    this.journal = journal; // آرایهٔ مشترک ران: [{key, value|null, opts}]
    try { fs.mkdirSync(dir, { recursive: true }); } catch (e) {}
  }
  _f(key) {
    return path.join(this.dir, encodeURIComponent(String(key)).slice(0, 200) + ".json");
  }
  _read(key) {
    try {
      const rec = JSON.parse(fs.readFileSync(this._f(key), "utf8"));
      if (rec && rec.exp && Date.now() > Number(rec.exp)) return null;
      return rec;
    } catch (e) {
      return null;
    }
  }
  async get(key, type) {
    const rec = this._read(key);
    if (!rec) return null;
    const v = rec.v === undefined || rec.v === null ? null : String(rec.v);
    if (v === null) return null;
    if (type === "json") {
      try { return JSON.parse(v); } catch (e) { return null; }
    }
    return v;
  }
  async put(key, value, opts) {
    const k = String(key);
    const v = value === null || value === undefined ? null : String(value);
    let exp = 0;
    try {
      const ttl = Number(opts && opts.expirationTtl);
      if (ttl > 0) exp = Date.now() + Math.min(30 * 86400, ttl) * 1000;
    } catch (e) {}
    try { fs.writeFileSync(this._f(k), JSON.stringify({ v, exp })); } catch (e) {}
    if (!isRuntimeKey(k)) {
      // حذف هم ژورنال می‌شود (مقدار null یعنی delete در kvbatch)
      this.journal.push({ key: k, value: v, opts: exp ? { expirationTtl: Math.round((exp - Date.now()) / 1000) } : undefined });
    }
  }
  async delete(key) {
    const k = String(key);
    try { fs.unlinkSync(this._f(k)); } catch (e) {}
    if (!isRuntimeKey(k)) this.journal.push({ key: k, value: null });
  }
  async list(opts) {
    // حداقلی (برای سازگاری؛ مسیر هاست‌فیلتر استفاده نمی‌کند)
    const prefix = String((opts && opts.prefix) || "");
    let keys = [];
    try {
      for (const f of fs.readdirSync(this.dir)) {
        if (!f.endsWith(".json")) continue;
        const k = decodeURIComponent(f.slice(0, -5));
        if (k.startsWith(prefix)) keys.push({ name: k });
      }
    } catch (e) {}
    return { keys, list_complete: true, cursor: "" };
  }
}

// ---------- shim سوکت TCP برای whois (فاز ۴: مانیتور انقضای دامنه) ----------
// باندل ورکر whois را با cloudflare:sockets می‌زند که روی Node نیست.
// این connect سازگار با همان سطحِ استفاده‌شده در whoisQuery است
// (writable.getWriter/write/releaseLock + readable.getReader/read/cancel + close)
// و روی node:net سوار است. RDAP (دامنه‌های غیر ir.) همان fetch است و نیازی به shim ندارد.
function createTcpConnect() {
  const net = require("node:net");
  return ({ hostname, port }) => {
    const sock = net.createConnection({ host: String(hostname), port: Number(port) || 43 });
    const queue = [];
    const waiters = [];
    let ended = false;
    const finish = () => {
      if (ended) return;
      ended = true;
      while (waiters.length) {
        const w = waiters.shift();
        try { w({ done: true, value: undefined }); } catch (e) {}
      }
    };
    sock.on("data", (c) => {
      const u8 = new Uint8Array(c);
      if (waiters.length) {
        const w = waiters.shift();
        try { w({ done: false, value: u8 }); } catch (e) {}
      } else {
        queue.push(u8);
      }
    });
    sock.on("end", finish);
    sock.on("error", finish);
    sock.on("close", finish);
    let writerReleased = false;
    return {
      writable: {
        getWriter: () => ({
          write: async (chunk) => {
            if (writerReleased) throw new Error("writer released");
            await new Promise((resolve, reject) => {
              try {
                sock.write(Buffer.from(chunk), (e) => (e ? reject(e) : resolve()));
              } catch (e) {
                reject(e);
              }
            });
          },
          releaseLock: () => { writerReleased = true; },
        }),
      },
      readable: {
        getReader: () => ({
          read: () => new Promise((resolve) => {
            if (queue.length) return resolve({ done: false, value: queue.shift() });
            if (ended) return resolve({ done: true, value: undefined });
            const to = setTimeout(() => {
              const i = waiters.indexOf(done);
              if (i >= 0) waiters.splice(i, 1);
              resolve({ done: true, value: undefined });
            }, 15000);
            // سقف ۱۵ ثانیه برای هر read (حلقهٔ whoisQuery خودش ددلاین ۱۲ ثانیه‌ای دارد؛
            // این فقط برای وقتی است که سرور نه جواب بدهد نه وصل را ببندد)
            if (to.unref) to.unref();
            function done(r) { clearTimeout(to); resolve(r); }
            waiters.push(done);
          }),
          cancel: async () => { try { sock.destroy(); } catch (e) {} finish(); },
        }),
      },
      close: async () => { try { sock.destroy(); } catch (e) {} finish(); },
    };
  };
}

// ---------- رانر ----------
function createRunner(opts) {
  const log = (...a) => { try { console.error("[runner]", ...a); } catch (e) {} };
  const workerUrl = String((opts && opts.workerUrl) || "").replace(/\/+$/, "");
  const token = String((opts && opts.token) || "");
  const stateDir = String((opts && opts.stateDir) || "/var/lib/guardian-agent");
  const kvDir = path.join(stateDir, "kv");
  const bundleDir = path.join(stateDir, "bundle");
  const workerTag = String((opts && opts.workerTag) || ("v" + String((opts && opts.agentVersion) || RUNNER_VERSION)));
  let bundleJobs = null;

  async function workerFetch(p, body, timeoutMs) {
    return httpJson(workerUrl, body ? "POST" : "GET", p, body, token, timeoutMs);
  }

  async function ensureBundle() {
    if (bundleJobs) return bundleJobs;
    if (!workerUrl || !token) throw new Error("no worker/token");
    try { fs.mkdirSync(bundleDir, { recursive: true }); } catch (e) {}
    const tagSafe = workerTag.replace(/[^A-Za-z0-9_.-]/g, "_");
    const file = path.join(bundleDir, `worker-${tagSafe}.mjs`);
    let src = "";
    if (opts && opts.bundleFile) {
      // تست محلی: باندل از فایل (بدون دانلود) — همان پچ و اعتبارسنجی اعمال می‌شود
      src = fs.readFileSync(opts.bundleFile, "utf8");
    } else {
      try {
        const st = fs.statSync(file);
        if (st.size > 500000) src = fs.readFileSync(file, "utf8");
      } catch (e) {}
    }
    try {
      const st = fs.statSync(file);
      if (st.size > 500000) src = fs.readFileSync(file, "utf8");
    } catch (e) {}
    if (!src || !src.includes("BOT_VERSION") || !src.includes("async function runHostFilter")) {
      // دانلود تازه از تگ پین‌شده (main هرگز — فقط تگ)
      const urls = [
        `https://raw.githubusercontent.com/Aknuun/cloud-guardian/${encodeURIComponent(workerTag)}/worker.js`,
        `https://cdn.jsdelivr.net/gh/Aknuun/cloud-guardian@${encodeURIComponent(workerTag)}/worker.js`,
      ];
      let ok = false;
      for (const u of urls) {
        try {
          const code = await fetchText(u);
          if (code && code.length > 500000 && code.includes("BOT_VERSION") && code.includes("async function runHostFilter")) {
            fs.writeFileSync(file, code);
            src = code;
            ok = true;
            break;
          }
        } catch (e) {}
      }
      if (!ok) throw new Error("bundle_download_failed");
    }
    // پچ ۱: سوکت کلادفلر روی Node نیست → shim سازگار با node:net (تزریق از رانر).
    // فقط whois انقضای دامنه (.ir روی پورت ۴۳) از آن استفاده می‌کند؛ بقیه fetch است.
    if (!src.includes('from "cloudflare:sockets"')) throw new Error("bundle_shape_changed");
    globalThis.__SOCKET_CONNECT = globalThis.__SOCKET_CONNECT || createTcpConnect();
    const patched = src.replace(
      'import { connect } from "cloudflare:sockets";',
      'const connect = (...args) => globalThis.__SOCKET_CONNECT(...args);'
    );
    // پچ ۲: جدول صریح jobهای مجاز فاز ۳ + ۴ و هیبرید CF (2.4.0) — تنها همین تابع‌ها از راه دور صدا زده می‌شوند
    const bundleSrc = patched + "\nglobalThis.__HFJOBS = { runHostFilter, hostFilterRevert, hfRestoreBackup, hfSnapshot, runNodePoll, runUsageMonitor, runSrvMonitor, runPgDigest, runSslMonitor, runDomExpiryMonitor, getAccounts, getAllZones, getZoneById, getRecords, cfPurgeCache, cfDnsList, cfDnsUpsert, cfDnsDelete };\n";
    // فایل اجرای یکتا (کش import گره‌گیر نشود) + پاک‌سازی اجراهای قبلی
    try {
      for (const f of fs.readdirSync(bundleDir)) {
        if (/^run-.*\.mjs$/.test(f)) { try { fs.unlinkSync(path.join(bundleDir, f)); } catch (e) {} }
      }
    } catch (e) {}
    const runFile = path.join(bundleDir, `run-${tagSafe}-${Date.now().toString(36)}.mjs`);
    fs.writeFileSync(runFile, bundleSrc);
    // گارد crypto: باندل ورکر از crypto سراسری استفاده می‌کند (makeToken و...).
    // اگر محیط Node آن را نداشته باشد (دیده‌شده در لاگ Oct 8)، از node:crypto می‌گیریم تا ران با ReferenceError نمیرد.
    try {
      if (typeof globalThis.crypto === "undefined") globalThis.crypto = require("node:crypto").webcrypto;
    } catch (e) {}
    await import(pathToFileURL(runFile).href);
    if (!globalThis.__HFJOBS || typeof globalThis.__HFJOBS.runHostFilter !== "function") {
      throw new Error("bundle_jobs_missing");
    }
    bundleJobs = globalThis.__HFJOBS;
    return bundleJobs;
  }

  function fetchText(url) {
    return new Promise((resolve, reject) => {
      const u = new URL(url);
      const mod = u.protocol === "https:" ? require("node:https") : require("node:http");
      const req = mod.get(url, { timeout: 60000 }, (res) => {
        if (res.statusCode !== 200) { res.resume(); return reject(new Error("http_" + res.statusCode)); }
        let raw = "";
        res.on("data", (c) => { raw += c; if (raw.length > 4 * 1024 * 1024) req.destroy(); });
        res.on("end", () => resolve(raw));
      });
      req.on("timeout", () => { req.destroy(); reject(new Error("timeout")); });
      req.on("error", reject);
    });
  }

  async function syncKeys() {
    const r = await workerFetch("/guardian/sync", { token, keys: SYNC_KEYS }, 30000);
    if (!r.ok || !(r.data && r.data.ok)) throw new Error("sync_failed:" + String((r.data && r.data.error) || r.error || r.status));
    // seed مستقیم فایل‌ها (بدون ژورنال — ورودی‌اند نه خروجی)
    try { fs.mkdirSync(kvDir, { recursive: true }); } catch (e) {}
    for (const k of SYNC_KEYS) {
      const v = r.data.keys && r.data.keys[k];
      if (v !== null && v !== undefined) {
        try { fs.writeFileSync(path.join(kvDir, encodeURIComponent(k).slice(0, 200) + ".json"), JSON.stringify({ v: String(v), exp: 0 })); } catch (e) {}
      }
    }
    return { botToken: String(r.data.bot_token || ""), adminId: Number(r.data.admin_id) || 0,
      texts: r.data.keys || {} };
  }

  async function pushJournal(journal, expectMap) {
    if (!journal.length) return { ok: true, wrote: 0, conflicts: 0 };
    const puts = journal.map((j) => (expectMap && expectMap[j.key] !== undefined ? { ...j, expect: expectMap[j.key] } : j));
    const r = await workerFetch("/guardian/kvbatch", { token, puts }, 30000);
    if (!r.ok || !(r.data && r.data.ok)) throw new Error("kvbatch_failed");
    if (r.data.conflicts) log(`kvbatch: ${r.data.conflicts} conflict(s) skipped (worker state is newer)`);
    return r.data;
  }

  async function checkServerMode() {
    const r = await workerFetch(`/guardian/mode?token=${encodeURIComponent(token)}`, null, 10000);
    if (!r.ok || !(r.data && r.data.ok)) throw new Error("mode_check_failed");
    return r.data.mode;
  }

  // ارقام فارسی برای متن پیام‌های دایجست (معادل faNum ورکر — باندل در دسترس رانر نیست)
  function faD(s) {
    return String(s).replace(/[0-9]/g, (d) => "۰۱۲۳۴۵۶۷۸۹"[Number(d)]);
  }

  // اجرای generیک یک جاب روی باندل ورکر: mode-check + sync + FileKV + push journal.
  // همهٔ جاب‌های فاز ۴ و هیبرید CF از همین مسیر می‌گذرند (تایمر ایجنت و /job ورکر).
  // خروجی آرایه سالم می‌ماند (object spread آرایه را خراب می‌کند) تا اعتبارسنج ورکر کار کند.
  async function runRemoteJob(name, args) {
    const fn = ACTIONS[name];
    if (!fn) return { ok: false, error: "bad_job" };
    const mode = await checkServerMode();
    if (mode !== "server") return { skipped: "not_server_mode", mode };
    const { botToken, adminId, texts } = await syncKeys();
    if (!botToken) throw new Error("no_bot_token");
    const journal = [];
    const runKv = new FileKV(kvDir, journal);
    // seed خوانده‌شده با ژورنال ران قاطی نمی‌شود: فایل‌ها مشترک‌اند، ژورنال جداست
    const env = { BOT_KV: runKv, BOT_TOKEN: botToken, ADMIN_ID: adminId, WORKER_URL: workerUrl };
    const jobs = await ensureBundle();
    const result = await fn(jobs, env, args || {});
    let pushed = { wrote: 0, conflicts: 0 };
    try { pushed = await pushJournal(journal, texts); } catch (e) { /* نتیجه ران معتبر است؛ push بعدی sync می‌کند */ }
    const base = Array.isArray(result)
      ? { result }
      : { ...(result && typeof result === "object" ? result : { result }) };
    return { ...base, _pushed: pushed.wrote || 0, _conflicts: pushed.conflicts || 0 };
  }

  async function runHostFilterJob(jobOpts) {
    const r = await runRemoteJob("hostfilter", jobOpts && jobOpts.force ? { force: true } : {});
    if (r && r.skipped) return r;
    if (r && r.checked !== undefined) log(`hostfilter run: checked=${r.checked} changed=${r.changed} pushed=${r._pushed}`);
    return r;
  }

  const ACTIONS = {
    hostfilter: (jobs, env, a) => jobs.runHostFilter(env, a && a.force ? { force: true } : {}),
    hfrevert: (jobs, env, a) => jobs.hostFilterRevert(env.BOT_KV, env, String(a.panel || ""), String(a.host || "")),
    hfrestore: (jobs, env, a) => jobs.hfRestoreBackup(env.BOT_KV, env, String(a.id || "")),
    hfsnapshot: (jobs, env, a) => jobs.hfSnapshot(env.BOT_KV, env, String((a && a.label) || "دستی")),
    // فاز ۴ — هر ۵ گروه منتقل‌شده (امضای واقعی توابع ورکر؛ env باندل همان FileKV است)
    nodepoll: (jobs, env, a) => jobs.runNodePoll(env, a && a.force ? { force: true } : {}),
    umon: (jobs, env) => jobs.runUsageMonitor(env, {}),
    srvmon: (jobs, env) => jobs.runSrvMonitor(env, env.BOT_TOKEN, false, {}),
    pgdigest_usage: (jobs, env) =>
      jobs.runPgDigest(env, env.BOT_TOKEN, env.ADMIN_ID, "usage", "pgdlast:usage", "📦", "دایجست ساعتی حجم", (u) => "≈" + faD(u.bucket) + "٪"),
    pgdigest_days: (jobs, env) =>
      jobs.runPgDigest(env, env.BOT_TOKEN, env.ADMIN_ID, "days", "pgdlast:days", "📅", "دایجست روزانه انقضا", (u) => faD(u.bucket) + " روز"),
    sslmon: (jobs, env) => jobs.runSslMonitor(env),
    domexpmon: (jobs, env) => jobs.runDomExpiryMonitor(env),
    // هیبرید Cloudflare (2.4.0): خوانش زون/رکورد و purge — توکن‌ها از همان sync می‌آیند؛
    // خروجی در ورکر اعتبارسنجی می‌شود. کش FileKV محلی است (push نمی‌شود).
    cf_zones: async (jobs, env) => {
      const accounts = await jobs.getAccounts(env.BOT_KV, env);
      return jobs.getAllZones(accounts, env.BOT_KV);
    },
    cf_records: async (jobs, env, a) => {
      const accounts = await jobs.getAccounts(env.BOT_KV, env);
      const zone = await jobs.getZoneById(String((a && a.zone_id) || ""), Number((a && a.acc) || 0), accounts);
      if (!zone || !zone.id) throw new Error("zone_not_found");
      return jobs.getRecords(zone, accounts, env.BOT_KV);
    },
    cf_purge: (jobs, env, a) => jobs.cfPurgeCache(env, Number((a && a.acc) || 0), String((a && a.zone_id) || "")),
    // DNS بدون KV (2.4.2): آرگومان صریح، صفر write — حتی زیر سقف سهمیه کار می‌کند
    cf_dns_list: (jobs, env, a) => jobs.cfDnsList(env, a || {}),
    cf_dns_upsert: (jobs, env, a) => jobs.cfDnsUpsert(env, a || {}),
    cf_dns_delete: (jobs, env, a) => jobs.cfDnsDelete(env, a || {}),
  };

  async function callAction(name, args) {
    if (!ACTIONS[name]) return { ok: false, error: "bad_job" };
    let r;
    try {
      r = await runRemoteJob(name, args || {});
    } catch (e) {
      return { ok: false, error: String((e && e.message) || e).slice(0, 120) };
    }
    if (r && (r.skipped || r.error)) return { ok: false, error: String((r && (r.skipped || r.error)) || "failed") };
    const { _pushed, _conflicts, ...rest } = r || {};
    // خروجی آرایه‌ای (cf_zones/cf_records) از envelope بیرون می‌آید؛ آبجکت‌ها همان فیلدها
    const keys = Object.keys(rest);
    const final = keys.length === 1 && keys[0] === "result" ? rest.result : rest;
    return { ok: true, result: final === undefined ? null : final };
  }

  return { runHostFilterJob, runRemoteJob, callAction, isRuntimeKey, FileKV, RUNNER_VERSION };
}

module.exports = { createRunner, createTcpConnect, isRuntimeKey, FileKV, RUNNER_VERSION };
