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

const RUNNER_VERSION = "2.0.0";
// کلیدهایی که ایجنت اجازهٔ pull/push آن‌ها را دارد (مکمل allowlist ورکر)
const SYNC_KEYS = ["accounts", "panels", "admins", "host_filter_cfg"];
const RUNTIME_EXACT = new Set([
  "hosts_cache",
  "hf_progress",
  "host_filter_log",
  "host_filter_state",
  "host_filter_ip_notice",
  "host_filter_crash",
]);
function isRuntimeKey(key) {
  const k = String(key || "");
  if (RUNTIME_EXACT.has(k)) return true;
  return k.startsWith("dnscache:") || k.startsWith("ipinfo:");
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

// ---------- رانر ----------
function createRunner(opts) {
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
    // پچ ۱: سوکت کلادفلر فقط برای whois انقضاست (منتقل نشده) → خطای صریح به‌جای رفتار ضمنی
    if (!src.includes('from "cloudflare:sockets"')) throw new Error("bundle_shape_changed");
    const patched = src.replace(
      'import { connect } from "cloudflare:sockets";',
      'const connect = () => { throw new Error("no sockets on agent (whois not offloaded)"); };'
    );
    // پچ ۲: جدول صریح jobهای مجاز (allowlist — تنها همین چهار تابع از راه دور صدا زده می‌شوند)
    const bundleSrc = patched + "\nglobalThis.__HFJOBS = { runHostFilter, hostFilterRevert, hfRestoreBackup, hfSnapshot };\n";
    // فایل اجرای یکتا (کش import گره‌گیر نشود) + پاک‌سازی اجراهای قبلی
    try {
      for (const f of fs.readdirSync(bundleDir)) {
        if (/^run-.*\.mjs$/.test(f)) { try { fs.unlinkSync(path.join(bundleDir, f)); } catch (e) {} }
      }
    } catch (e) {}
    const runFile = path.join(bundleDir, `run-${tagSafe}-${Date.now().toString(36)}.mjs`);
    fs.writeFileSync(runFile, bundleSrc);
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

  async function runHostFilterJob(jobOpts) {
    const mode = await checkServerMode();
    if (mode !== "server") return { skipped: "not_server_mode", mode };
    const { botToken, adminId, texts } = await syncKeys();
    if (!botToken) throw new Error("no_bot_token");
    const journal = [];
    const runKv = new FileKV(kvDir, journal);
    // seed خوانده‌شده با ژورنال ران قاطی نمی‌شود: فایل‌ها مشترک‌اند، ژورنال جداست
    const env = { BOT_KV: runKv, BOT_TOKEN: botToken, ADMIN_ID: adminId, WORKER_URL: workerUrl };
    const jobs = await ensureBundle();
    const opts = jobOpts && jobOpts.force ? { force: true } : {};
    const result = await jobs.runHostFilter(env, opts);
    let pushed = { wrote: 0, conflicts: 0 };
    try { pushed = await pushJournal(journal, texts); } catch (e) { /* نتیجه ران معتبر است؛ push بعدی sync می‌کند */ }
    return { ...result, _pushed: pushed.wrote || 0, _conflicts: pushed.conflicts || 0 };
  }

  const ACTIONS = {
    hostfilter: (jobs, env, a) => jobs.runHostFilter(env, a && a.force ? { force: true } : {}),
    hfrevert: (jobs, env, a) => jobs.hostFilterRevert(env.BOT_KV, env, String(a.panel || ""), String(a.host || "")),
    hfrestore: (jobs, env, a) => jobs.hfRestoreBackup(env.BOT_KV, env, String(a.id || "")),
    hfsnapshot: (jobs, env, a) => jobs.hfSnapshot(env.BOT_KV, env, String((a && a.label) || "دستی")),
  };

  async function callAction(name, args) {
    if (!ACTIONS[name]) return { ok: false, error: "bad_job" };
    const mode = await checkServerMode();
    if (mode !== "server") return { ok: false, error: "not_server_mode" };
    const { botToken, adminId, texts } = await syncKeys();
    if (!botToken) throw new Error("no_bot_token");
    const journal = [];
    const runKv = new FileKV(kvDir, journal);
    const env = { BOT_KV: runKv, BOT_TOKEN: botToken, ADMIN_ID: adminId, WORKER_URL: workerUrl };
    const jobs = await ensureBundle();
    const result = await ACTIONS[name](jobs, env, args || {});
    try { await pushJournal(journal, texts); } catch (e) {}
    return { ok: true, result: result === undefined ? null : result };
  }

  return { runHostFilterJob, callAction, isRuntimeKey, FileKV, RUNNER_VERSION };
}

module.exports = { createRunner, isRuntimeKey, FileKV, RUNNER_VERSION };
