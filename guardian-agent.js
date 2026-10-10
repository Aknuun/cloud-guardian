#!/usr/bin/env node
"use strict";
/* =====================================================================
 * guardian-agent — ایجنت سرور مشتری «نگهبان ابری»
 * ---------------------------------------------------------------------
 * هر سه نقش قدیمی/جدید در یک دیمون تکی، بدون هیچ dependency خارجی
 * (فقط ماژول‌های داخلی node: http/fs/child_process/crypto):
 *
 *  ۱) اجرای دائم همهٔ جاب‌های دوره‌ای روی همین سرور (به‌جای ورکر)؛
 *     فقط «تغییر» به ورکر push می‌شود تا write/read کلادفلر نزدیک صفر شود.
 *  ۲) جایگزین srv-relay: اجرای دستور از راه دور برای منوی 🖥 سرورهای ربات.
 *  ۳) heartbeat هر ۶۰ ثانیه به ورکر؛ اگر قطع شود ورکر خودش کار می‌کند
 *     (حالت اضطراری) تا سرور جایگزین بیاید.
 *
 *  کانفیگ (اولویت: env و بعد فایل):
 *    GUARDIAN_TOKEN  توکن مشترک با ورکر (اجباری)
 *    WORKER_URL      آدرس پایهٔ ورکر، مثل https://x.y.workers.dev (اجباری برای heartbeat)
 *    PORT            پورت شنود (پیش‌فرض 8789)
 *    PUBLIC_URL      آدرس عمومی همین ایجنت (برای ثبت خودکار در ورکر؛ اگر خالی باشد ثبت دستی لازم است)
 *    STATE_DIR       مسیر state محلی (پیش‌فرض /var/lib/guardian-agent)
 *    AGENT_SRC_URL   آدرس دانلود نسخهٔ جدید برای خودآپدیت
 *
 *  endpointها (همهٔ POSTها با هدر x-guardian-token):
 GET  /ping    سلامت + نسخه (بدون احراز)
 POST /exec    اجرای دستور: محلی، یا با SSH روی سرور مقصد (قرارداد رله)
 POST /stats   آمار منابع سرور مقصد با SSH (قرارداد رله)
 POST /http    پروکسی HTTP (قرارداد رله)
 POST /update  خودآپدیت از گیت‌هاب + ری‌استارت
 * ===================================================================== */

const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");
const { execFile, spawn } = require("node:child_process");
const os = require("node:os");

const AGENT_VERSION = "2.3.0";
const DEFAULT_PORT = 8789;
const HEARTBEAT_MS = 60 * 1000;
const EXEC_TIMEOUT_MS = 120 * 1000;
const EXEC_MAXBUF = 2 * 1024 * 1024;
const SSH_STATS_TIMEOUT_MS = 25000;
const SSH_MAX_SESSIONS_PER_MIN = 20;
const _sshHits = [];
function sshRateOk() {
  const now = Date.now();
  while (_sshHits.length && now - _sshHits[0] > 60000) _sshHits.shift();
  if (_sshHits.length >= SSH_MAX_SESSIONS_PER_MIN) return false;
  _sshHits.push(now);
  return true;
}
const HTTP_TIMEOUT_MS = 15 * 1000;
const AGENT_SRC_DEFAULT =
  "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/guardian-agent.js";

// ---------- کانفیگ ----------
function readConfFile() {
  // فایل /etc/guardian-agent/agent.conf با فرمت KEY=value (بدون export)
  const out = {};
  for (const p of ["/etc/guardian-agent/agent.conf", "/opt/guardian-agent/agent.conf"]) {
    try {
      const txt = fs.readFileSync(p, "utf8");
      for (const line of txt.split("\n")) {
        const m = line.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$/);
        if (!m || line.trim().startsWith("#")) continue;
        let v = m[2].trim();
        if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) v = v.slice(1, -1);
        if (!(m[1] in out)) out[m[1]] = v;
      }
    } catch (e) { /* نبود فایل یعنی کانفیگ فقط از env می‌آید */ }
  }
  return out;
}
const FILE_CONF = readConfFile();
const cfg = (k, dflt) => {
  const v = process.env[k] !== undefined && process.env[k] !== "" ? process.env[k] : FILE_CONF[k];
  return v !== undefined && v !== "" ? v : dflt;
};

const TOKEN = cfg("GUARDIAN_TOKEN", "");
const WORKER_URL = String(cfg("WORKER_URL", "")).replace(/\/+$/, "");
const PORT = Number(cfg("PORT", DEFAULT_PORT)) || DEFAULT_PORT;
const PUBLIC_URL = String(cfg("PUBLIC_URL", "")).replace(/\/+$/, "");
const STATE_DIR = cfg("STATE_DIR", "/var/lib/guardian-agent");
const AGENT_SRC_URL = cfg("AGENT_SRC_URL", AGENT_SRC_DEFAULT);

try { fs.mkdirSync(STATE_DIR, { recursive: true }); } catch (e) {}

// ---------- لاگ محلی (چرخشی ساده: بالای ۱ مگ، نصف آخر نگه داشته می‌شود) ----------
const LOG_FILE = path.join(STATE_DIR, "agent.log");
function log(...args) {
  const line = `[${new Date().toISOString()}] ${args.map((a) => (typeof a === "string" ? a : JSON.stringify(a))).join(" ")}`;
  try {
    let st = null;
    try { st = fs.statSync(LOG_FILE); } catch (e) {}
    if (st && st.size > 1024 * 1024) {
      const tail = fs.readFileSync(LOG_FILE, "utf8").slice(-512 * 1024);
      fs.writeFileSync(LOG_FILE, tail);
    }
    fs.appendFileSync(LOG_FILE, line + "\n");
  } catch (e) {}
  console.log(line);
}

// ---------- state محلی (جایگزین KV ورکر؛ هر جاب یک فایل JSON) ----------
function stateGet(name, dflt) {
  try {
    const raw = fs.readFileSync(path.join(STATE_DIR, `job-${name}.json`), "utf8");
    return JSON.parse(raw);
  } catch (e) {
    return dflt;
  }
}
function statePut(name, obj) {
  try {
    const tmp = path.join(STATE_DIR, `.job-${name}.tmp`);
    fs.writeFileSync(tmp, JSON.stringify(obj));
    fs.renameSync(tmp, path.join(STATE_DIR, `job-${name}.json`));
  } catch (e) {
    log("statePut failed", name, String(e));
  }
}

// ---------- رجیستری جاب‌ها (فاز ۳ هر جاب ورکر را اینجا ثبت می‌کند) ----------
const JOBS = {}; // name -> { intervalMs, run(ctx) }
function registerJob(name, intervalMs, fn) {
  if (!name || typeof fn !== "function" || !(intervalMs > 0)) throw new Error("bad job: " + name);
  JOBS[name] = { intervalMs, fn };
}
async function runJobsOnce() {
  // اجرای نوبتی جاب‌های رسیده؛ خطای هر جاب بقیه را متوقف نمی‌کند
  const now = Date.now();
  for (const [name, job] of Object.entries(JOBS)) {
    try {
      const meta = stateGet(`meta-${name}`, {});
      if (meta.next && now < Number(meta.next)) continue;
      const ctx = { stateGet, statePut, pushEvent, log: (...a) => log(`[${name}]`, ...a) };
      await job.fn(ctx);
      statePut(`meta-${name}`, { last: now, next: now + job.intervalMs, err: "" });
    } catch (e) {
      log(`job ${name} failed:`, String(e && e.stack ? e.stack : e));
      try { statePut(`meta-${name}`, { last: now, next: now + job.intervalMs, err: String(e).slice(0, 300) }); } catch (x) {}
    }
  }
}
function startJobLoop() {
  setInterval(() => { runJobsOnce().catch((e) => log("jobs loop:", String(e))); }, 30 * 1000);
}

// ---------- ارتباط با ورکر ----------
function postWorker(body, timeoutMs) {
  return new Promise((resolve) => {
    if (!WORKER_URL) return resolve({ ok: false, error: "no WORKER_URL" });
    const data = Buffer.from(JSON.stringify(body));
    const u = new URL(WORKER_URL + "/guardian");
    const mod = u.protocol === "https:" ? require("node:https") : require("node:http");
    const req = mod.request(
      { hostname: u.hostname, port: u.port || (u.protocol === "https:" ? 443 : 80), path: u.pathname, method: "POST",
        headers: { "Content-Type": "application/json", "Content-Length": data.length },
        timeout: timeoutMs || HTTP_TIMEOUT_MS },
      (res) => {
        let raw = "";
        res.on("data", (c) => { raw += c; });
        res.on("end", () => {
          try { resolve({ ok: true, status: res.statusCode, data: JSON.parse(raw) }); }
          catch (e) { resolve({ ok: false, error: "bad_json" }); }
        });
      }
    );
    req.on("timeout", () => { req.destroy(); resolve({ ok: false, error: "timeout" }); });
    req.on("error", (e) => resolve({ ok: false, error: String(e && e.message ? e.message : e).slice(0, 120) }));
    req.write(data);
    req.end();
  });
}

async function sendHeartbeat() {
  if (!TOKEN) { log("heartbeat skipped: GUARDIAN_TOKEN not set"); return; }
  const r = await postWorker({ token: TOKEN, action: "heartbeat", version: AGENT_VERSION, ts: Date.now(), public_url: PUBLIC_URL });
  if (!r.ok || !(r.data && r.data.ok)) log("heartbeat failed:", r.error || JSON.stringify(r.data).slice(0, 200));
}
async function pushEvent(ev) {
  // رویداد تغییر از جاب‌ها: { kind, title, text } — ورکر فقط پیام تلگرام می‌فرستد
  if (!TOKEN) return { ok: false, error: "no token" };
  const title = String((ev && ev.title) || "").slice(0, 200);
  const text = String((ev && ev.text) || "").slice(0, 3000);
  if (!title || !text) return { ok: false, error: "bad event" };
  return postWorker({ token: TOKEN, action: "event", kind: String((ev && ev.kind) || "info").slice(0, 40), title, text, ts: Date.now() });
}
function startHeartbeat() {
  setInterval(() => { sendHeartbeat().catch((e) => log("heartbeat:", String(e))); }, HEARTBEAT_MS);
}

// ---------- رانر (اجرای jobهای ورکر با همان کد؛ اگر فایلش نباشد فقط heartbeat/exec کار می‌کند) ----------
let RUNNER = null;
// قفل اجرای تکی هاست‌فیلتر: تیک زمان‌بند و درخواست /job هرگز همزمان اجرا نمی‌شوند.
// (بدون این، تیک‌های ۶۰ثانیه‌ای وسط یک ران ۱۰دقیقه‌ای ران دوم می‌زدند.)
let HF_BUSY = false;
async function runHfExclusive(fn, background) {
  if (HF_BUSY) return { ok: false, error: "busy" };
  HF_BUSY = true;
  try {
    return await fn();
  } finally {
    HF_BUSY = false;
  }
}
function loadRunner() {
  try {
    const mod = require("./guardian-runner.js");
    if (!mod || typeof mod.createRunner !== "function") return null;
    return mod.createRunner({ workerUrl: WORKER_URL, token: TOKEN, stateDir: STATE_DIR, agentVersion: AGENT_VERSION });
  } catch (e) {
    log("runner not loaded:", String(e && e.message ? e.message : e).slice(0, 150));
    return null;
  }
}

function readRuntimeKey(key) {
  // همان انکدینگ FileKV رانر (تک‌منبع نباشد، ولی قرارداد ثابت است)
  const k = String(key || "").slice(0, 256);
  if (!k) return { ok: false, error: "bad_key" };
  try {
    const f = path.join(STATE_DIR, "kv", encodeURIComponent(k).slice(0, 200) + ".json");
    const rec = JSON.parse(fs.readFileSync(f, "utf8"));
    if (!rec || rec.v === null || rec.v === undefined) return { ok: true, found: false };
    if (rec.exp && Date.now() > Number(rec.exp)) return { ok: true, found: false };
    let value = rec.v;
    try { value = JSON.parse(String(rec.v)); } catch (e) { /* متن خام */ }
    return { ok: true, found: true, value };
  } catch (e) {
    return { ok: true, found: false };
  }
}

async function handleJob(body, res) {
  // اجرای اکشن ورکر روی همین سرور (allowlist داخل رانر است)
  if (!RUNNER) return sendJson(res, 503, { ok: false, error: "no_runner" });
  const name = String(body.job || "").slice(0, 40);
  const t0 = Date.now();
  try {
    const out = await runHfExclusive(() => RUNNER.callAction(name, body.args || {}));
    if (!out || out.ok !== true) {
      // busy یعنی یک ران دیگر در حال اجراست — ورکر نباید محلی تکرار کند (هم‌پوشانی)
      if (out && out.error === "busy") return sendJson(res, 200, { ok: false, error: "busy" });
      return sendJson(res, 200, { ok: false, error: String((out && out.error) || "failed") });
    }
    log(`job ${name} done in ${Math.round((Date.now() - t0) / 1000)}s`);
    return sendJson(res, 200, { ok: true, result: out.result === undefined ? null : out.result });
  } catch (e) {
    log(`job ${name} failed:`, String(e && e.stack ? e.stack : e).slice(0, 500));
    return sendJson(res, 200, { ok: false, error: String((e && e.message) || e).slice(0, 200) });
  }
}

// ---------- سرور HTTP ----------
function readJsonBody(req) {
  return new Promise((resolve) => {
    let raw = "";
    let tooBig = false;
    req.on("data", (c) => {
      raw += c;
      if (raw.length > 512 * 1024) { tooBig = true; req.destroy(); }
    });
    req.on("end", () => {
      if (tooBig) return resolve(null);
      try { resolve(raw ? JSON.parse(raw) : {}); } catch (e) { resolve(null); }
    });
    req.on("error", () => resolve(null));
  });
}
function sendJson(res, code, obj) {
  const data = Buffer.from(JSON.stringify(obj));
  res.writeHead(code, { "Content-Type": "application/json", "Content-Length": data.length });
  res.end(data);
}
function authed(req, body) {
  if (!TOKEN) return false;
  const h = String(req.headers["x-guardian-token"] || "");
  const b = String((body && body.token) || "");
  return h === TOKEN || b === TOKEN;
}

const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url || "/", "http://localhost");
    if (req.method === "GET" && url.pathname === "/ping") {
      return sendJson(res, 200, { ok: true, agent: "guardian-agent", version: AGENT_VERSION, exec: true, jobs: Object.keys(JOBS), runner: !!RUNNER });
    }
    if ((req.method === "POST" || req.method === "GET") && url.pathname === "/state") {
      // خوانش کلید ران‌تایم برای UI ورکر (agent-first؛ fallback ورکر همان KV خودش است)
      const key = String(url.searchParams.get("key") || "");
      if (req.method === "POST") {
        const body = await readJsonBody(req);
        if (!body) return sendJson(res, 400, { ok: false, error: "bad_json" });
        if (!authed(req, body)) return sendJson(res, 403, { ok: false, error: "bad_token" });
      } else if (String(req.headers["x-guardian-token"] || "") !== TOKEN) {
        return sendJson(res, 403, { ok: false, error: "bad_token" });
      }
      return sendJson(res, 200, readRuntimeKey(key));
    }
    if (req.method === "POST" && (url.pathname === "/exec" || url.pathname === "/stats" || url.pathname === "/http" || url.pathname === "/update" || url.pathname === "/job")) {
      const body = await readJsonBody(req);
      if (!body) return sendJson(res, 400, { ok: false, error: "bad_json" });
      if (!authed(req, body)) return sendJson(res, 403, { ok: false, error: "bad_token" });
      if (url.pathname === "/exec") return handleExec(body, res);
      if (url.pathname === "/stats") return handleStats(body, res);
      if (url.pathname === "/http") return handleHttpProxy(body, res);
      if (url.pathname === "/update") return handleUpdate(body, res);
      return handleJob(body, res);
    }
    return sendJson(res, 404, { ok: false, error: "not_found" });
  } catch (e) {
    log("http:", String(e));
    try { return sendJson(res, 500, { ok: false, error: "internal" }); } catch (x) {}
  }
});

// ---------- SSH proxy (merged from srv-relay): exec/stats on managed servers ----------
// Secrets live only in RAM for the request; temp credential files are 0600 and wiped after.
async function writeAuthFiles(auth) {
  const dir = await fs.promises.mkdtemp(path.join(os.tmpdir(), "gdna-"));
  await fs.promises.chmod(dir, 0o700);
  const files = { dir, keyPath: null, passPath: null };
  if (auth.key) {
    const kp = path.join(dir, "id_key");
    let keyText = String(auth.key).replace(/\r\n/g, "\n");
    if (!keyText.endsWith("\n")) keyText += "\n";
    await fs.promises.writeFile(kp, keyText, { mode: 0o600 });
    await fs.promises.chmod(kp, 0o600);
    files.keyPath = kp;
    if (auth.keyPass) {
      const pp = path.join(dir, "keypass");
      await fs.promises.writeFile(pp, String(auth.keyPass) + "\n", { mode: 0o600 });
      files.passPath = pp;
    }
  } else if (auth.password) {
    const pp = path.join(dir, "pass");
    await fs.promises.writeFile(pp, String(auth.password) + "\n", { mode: 0o600 });
    files.passPath = pp;
  }
  return files;
}

async function cleanupAuthFiles(files) {
  if (!files || !files.dir) return;
  try {
    await fs.promises.rm(files.dir, { recursive: true, force: true });
  } catch (e) {}
}

// --- ساخت آرگومان‌های ssh برای یک اتصال ---
// نکته: BatchMode عمداً گذاشته نمی‌شود؛ با sshpass و askpass ناسازگار است (پرامپت رمز را کور می‌کند).
function sshArgs(host, port, user, files, extraArgs = []) {
  const args = [
    "-o", "StrictHostKeyChecking=no",
    "-o", "UserKnownHostsFile=/dev/null",
    "-o", "ConnectTimeout=12",
    "-o", "LogLevel=ERROR",
    "-p", String(Number(port) || 22),
  ];
  if (files.keyPath) args.push("-i", files.keyPath);
  args.push(`${user || "root"}@${host}`);
  args.push(...extraArgs);
  return args;
}

// اجرای یک پروسه (ssh یا sshpass) با گرفتن stdout/stderr و timeout
function runProc(bin, args, env, timeoutMs, onStdout) {
  return new Promise((resolve) => {
    const child = spawn(bin, args, {
      env: { ...env, PATH: process.env.PATH },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let out = "";
    let err = "";
    let done = false;
    const timer = setTimeout(() => {
      if (!done) {
        done = true;
        try { child.kill("SIGKILL"); } catch (e) {}
        resolve({ code: 124, out, err: err + "\n⏱ مهلت اجرا به پایان رسید.", timedOut: true });
      }
    }, timeoutMs);
    child.stdout.on("data", (d) => {
      if (out.length < 900000) out += d.toString();
      if (onStdout) { try { onStdout(d.toString()); } catch (e) {} }
    });
    child.stderr.on("data", (d) => {
      if (err.length < 100000) err += d.toString();
    });
    child.on("error", (e) => {
      if (done) return;
      done = true;
      clearTimeout(timer);
      resolve({ code: -1, out, err: String(e) });
    });
    child.on("close", (code) => {
      if (done) return;
      done = true;
      clearTimeout(timer);
      resolve({ code: code === null ? -1 : code, out, err, timedOut: false });
    });
  });
}

// اعتبارنامهٔ رمزی: با sshpass اجرا می‌کنیم (در صورت نصب)
async function sshExec(host, port, user, auth, command, timeoutMs, onStdout) {
  if (!host || !/^[A-Za-z0-9._:-]+$/.test(host)) return { code: -1, err: "invalid_host" };
  const portNum = Number(port) || 22;
  if (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535) return { code: -1, err: "invalid_port" };
  if (!command) return { code: -1, err: "no_command" };

  const files = await writeAuthFiles(auth);
  try {
    let env = { ...process.env };
    let bin = "ssh";
    let args;
    if (files.keyPath) {
      // کلید: اگر passphrase دارد از طریق اسکریپت askpass (در env) پاس می‌شود
      if (files.passPath) {
        const askpass = path.join(files.dir, "askpass.sh");
        await fs.promises.writeFile(askpass, `#!/bin/sh\ncat "${files.passPath}"\n`, { mode: 0o700 });
        env = { ...env, SSH_ASKPASS: askpass, SSH_ASKPASS_REQUIRE: "force", DISPLAY: ":0" };
      }
      args = sshArgs(host, portNum, user, files);
    } else if (files.passPath) {
      // رمز عبور: sshpass (اگر نصب نبود خطای روشن می‌دهیم)
      // ⚠️ آرگومان‌های sshpass فقط برای باینری sshpass است — به ssh داده نمی‌شود.
      const hasSshpass = await new Promise((r) => execFile("which", ["sshpass"], (e) => r(!e)));
      if (!hasSshpass) return { code: -1, err: "sshpass_not_installed" };
      bin = "sshpass";
      args = ["-f", files.passPath, "ssh", ...sshArgs(host, portNum, user, files)];
    } else {
      return { code: -1, err: "no_auth" };
    }
    args.push(command);
    const r = await runProc(bin, args, env, timeoutMs, onStdout);
    return r;
  } finally {
    await cleanupAuthFiles(files);
  }
}

// --- جمع‌کردن آمار منابع از سرور مقصد ---
async function sshStats(host, port, user, auth) {
  const script = [
    "echo '===SYS==='",
    "hostname",
    "echo '===CPU==='",
    "top -bn1 | grep 'Cpu(s)' | awk '{print int($2+$4)}'",
    "echo '===CORES==='",
    "nproc",
    "echo '===MEM==='",
    "free -m | awk '/Mem:/{printf \"%d %d %d\\n\", $2, $3, $7}'",
    "echo '===SWAP==='",
    "free -m | awk '/Swap:/{printf \"%d %d\\n\", $2, $3; exit}'",
    "echo '===DISK==='",
    "df -h / | awk 'NR==2{printf \"%s %s %s %s\\n\", $2, $3, $4, $5}'",
    "echo '===LOAD==='",
    "cat /proc/loadavg | awk '{print $1, $2, $3}'",
    "echo '===UPTIME==='",
    "awk '{printf \"%.0f\\n\", $1}' /proc/uptime",
    "echo '===NET==='",
    "cat /proc/net/dev | awk 'NR>2 && $1!~/lo:/ {gsub(\":\",\"\",$1); print $1, $2, $10}'",
    "echo '===PG==='",
    "docker ps --format '{{.Names}}|{{.Status}}' 2>/dev/null || true",
    "echo '===END==='",
  ].join("; ");
  const r = await sshExec(host, port, user, auth, script, SSH_STATS_TIMEOUT_MS);
  if (r.code !== 0 && !r.out) return { error: r.err || `ssh_exit_${r.code}` };
  // تجزیه
  const pick = (tag) => {
    const m = r.out.match(new RegExp(`===${tag}===\\n([^=]*?)\\n`));
    return m ? m[1].trim() : "";
  };
  const sys = pick("SYS");
  const cpuPct = parseInt(pick("CPU"), 10) || 0;
  const cores = parseInt(pick("CORES"), 10) || 1;
  const memParts = pick("MEM").split(/\s+/).map(Number);
  const swapParts = pick("SWAP").split(/\s+/).map(Number);
  const diskParts = pick("DISK").split(/\s+/);
  const load = pick("LOAD").split(/\s+/).map(Number);
  const uptimeS = parseInt(pick("UPTIME"), 10) || 0;
  const netLines = (r.out.split("===NET===\n")[1] || "").split("===PG===")[0].trim().split("\n").filter(Boolean);
  const net = [];
  for (const l of netLines) {
    const p = l.trim().split(/\s+/);
    if (p.length >= 3) net.push({ iface: p[0], rx: Number(p[1]) || 0, tx: Number(p[2]) || 0 });
  }
  const pg = (r.out.split("===PG===\n")[1] || "").split("===END===")[0].trim().split("\n").filter((x) => x && x.includes("|"));
  return {
    host, sys,
    cpu: { pct: cpuPct, cores, load: load[0] ?? 0 },
    mem: { totalMb: memParts[0] || 0, usedMb: memParts[1] || 0, availMb: memParts[2] || 0 },
    swap: { totalMb: swapParts[0] || 0, usedMb: swapParts[1] || 0 },
    disk: { total: diskParts[0] || "?", used: diskParts[1] || "?", avail: diskParts[2] || "?", pct: diskParts[3] || "?" },
    uptimeS,
    net,
    pg,
    _raw: r.out.length,
  };
}
function handleExec(body, res) {
  // SSH mode (relay contract): with host, run on the target server via system ssh.
  // Response mirrors srv-relay exactly: {code,out,err,timedOut,timeoutMs}.
  const host = String(body.host || "");
  if (host) {
    if (!sshRateOk()) return sendJson(res, 429, { ok: false, error: "rate_limited" });
    const reqTimeout = Number(body.timeoutMs) || EXEC_TIMEOUT_MS;
    const tMs = Math.min(Math.max(reqTimeout, 3000), 570000);
    sshExec(host, body.port, body.user, { password: body.password, key: body.key, keyPass: body.keyPass }, String(body.cmd || body.command || "").slice(0, 20000), tMs).then((r) => {
      sendJson(res, 200, { code: r.code, out: String(r.out || "").slice(-60000), err: String(r.err || "").slice(-4000), timedOut: !!r.timedOut, timeoutMs: tMs });
    }).catch((e) => {
      sendJson(res, 200, { code: -1, err: String((e && e.message) || e).slice(0, 500) });
    });
    return;
  }
  const cmd = String(body.cmd || body.command || "");
  if (!cmd || cmd.length > 4000) return sendJson(res, 400, { ok: false, error: "bad_cmd" });
  // local shell exec (unchanged legacy behavior)
  execFile("/bin/sh", ["-c", cmd], { timeout: EXEC_TIMEOUT_MS, maxBuffer: EXEC_MAXBUF }, (err, stdout, stderr) => {
    sendJson(res, 200, {
      ok: !err || err.killed !== true,
      exit: err && typeof err.code === "number" ? err.code : 0,
      timeout: !!(err && err.killed),
      stdout: String(stdout || "").slice(-20000),
      stderr: String(err && err.message ? err.message + "\n" : "" + stderr || "").slice(-5000),
    });
  });
}

async function handleStats(body, res) {
  // Relay-contract stats via SSH on the target server.
  const host = String(body.host || "");
  if (!host) return sendJson(res, 400, { ok: false, error: "no_host" });
  if (!sshRateOk()) return sendJson(res, 429, { ok: false, error: "rate_limited" });
  try {
    const r = await sshStats(host, body.port, body.user, { password: body.password, key: body.key, keyPass: body.keyPass });
    if (r.error) return sendJson(res, 200, { error: r.error });
    return sendJson(res, 200, r);
  } catch (e) {
    return sendJson(res, 200, { error: String((e && e.message) || e).slice(0, 300) });
  }
}

async function handleHttpProxy(body, res) {
  // Relay-contract HTTP proxy: fetched from this server IP (not Cloudflare).
  const m = String(body.method || "GET").toUpperCase();
  if (!/^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)$/.test(m)) return sendJson(res, 400, { ok: false, error: "bad_method" });
  let tu;
  try { tu = new URL(String(body.url || "")); } catch (e) { return sendJson(res, 400, { ok: false, error: "bad_url" }); }
  if (!/^https?:$/.test(tu.protocol) || !tu.hostname) return sendJson(res, 400, { ok: false, error: "bad_url" });
  const tMs = Math.min(Math.max(Number(body.timeoutMs) || 30000, 3000), 120000);
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), tMs);
  try {
    const init = { method: m, headers: { ...(body.headers || {}) }, signal: controller.signal };
    if (body.body !== undefined && body.body !== null && !/^(GET|HEAD)$/.test(m)) init.body = String(body.body);
    const resp = await fetch(tu.href, init);
    const buf = Buffer.from(await resp.arrayBuffer());
    const MAX_BODY = 3 * 1024 * 1024;
    const bodyText = buf.length > MAX_BODY ? buf.slice(0, MAX_BODY).toString("utf8") : buf.toString("utf8");
    return sendJson(res, 200, {
      status: resp.status,
      statusText: resp.statusText,
      headers: {
        "content-type": resp.headers.get("content-type") || "",
        "location": resp.headers.get("location") || "",
        "retry-after": resp.headers.get("retry-after") || "",
      },
      body: bodyText,
      bytes: buf.length,
    });
  } catch (e) {
    return sendJson(res, 502, { ok: false, error: "fetch_failed", detail: String(e).slice(0, 300) });
  } finally {
    clearTimeout(timer);
  }
}

async function handleUpdate(body, res) {
  const src = String(body.src || AGENT_SRC_URL).slice(0, 500);
  if (!/^https?:\/\//.test(src)) return sendJson(res, 400, { ok: false, error: "bad_src" });
  // رانر همیشه هم‌نسخه با ایجنت آپدیت می‌شود (ورکر src را به تگ پین می‌کند)
  const runnerSrc = src.includes("guardian-agent.js") ? src.replace("guardian-agent.js", "guardian-runner.js") : "";
  sendJson(res, 200, { ok: true, updating: true, from: AGENT_VERSION });
  // دانلود + اعتبارسنجی + جایگزینی اتمی + خروج (systemd دوباره بالا می‌آورد)
  setImmediate(async () => {
    try {
      const code = await fetchText(src);
      if (!code || code.length < 5000 || !code.includes("guardian-agent") || !code.includes("AGENT_VERSION")) {
        log("self-update rejected: bad agent payload");
        return;
      }
      await selfReplace(__filename, code, "agent");
      if (runnerSrc) {
        try {
          const rcode = await fetchText(runnerSrc);
          if (rcode && rcode.length > 5000 && rcode.includes("guardian-runner")) {
            await selfReplace(path.join(path.dirname(__filename), "guardian-runner.js"), rcode, "runner");
          } else {
            log("runner update rejected: bad payload");
          }
        } catch (e) {
          log("runner update failed:", String(e && e.message ? e.message : e).slice(0, 200));
        }
      }
      log(`self-update done (${AGENT_VERSION} -> new), restarting`);
      setTimeout(() => process.exit(0), 500);
    } catch (e) {
      log("self-update failed:", String(e));
    }
  });
}

// جایگزینی امن فایل: اول node --check روی فایل موقت (پسوند js. چون --check پسوند ناشناس را رد می‌کند)، بعد rename اتمی
async function selfReplace(file, code, what) {
  const { execFile: ef } = require("node:child_process");
  const tmp = file + ".check.js";
  fs.writeFileSync(tmp, code);
  try {
    await new Promise((resolve, reject) => {
      ef(process.execPath, ["--check", tmp], (err) => (err ? reject(err) : resolve()));
    });
  } catch (e) {
    try { fs.unlinkSync(tmp); } catch (x) {}
    throw new Error(what + " syntax check failed");
  }
  fs.renameSync(tmp, file);
}
function fetchText(url) {
  return new Promise((resolve, reject) => {
    const u = new URL(url);
    const mod = u.protocol === "https:" ? require("node:https") : require("node:http");
    const req = mod.get(url, { timeout: 30000 }, (res) => {
      if (res.statusCode !== 200) { res.resume(); return reject(new Error("http_" + res.statusCode)); }
      let raw = "";
      res.on("data", (c) => { raw += c; if (raw.length > 2 * 1024 * 1024) req.destroy(); });
      res.on("end", () => resolve(raw));
    });
    req.on("timeout", () => { req.destroy(); reject(new Error("timeout")); });
    req.on("error", reject);
  });
}

function start() {
  RUNNER = loadRunner();
  if (RUNNER && TOKEN && WORKER_URL) {
    // جاب هاست‌فیلتر: خودِ ران گیت فاصله/حالت را چک می‌کند؛ تیک هر ۶۰ ثانیه فقط بیدارباش است
    try {
      registerJob("hostfilter", 60 * 1000, async () => {
        const r = await runHfExclusive(() => RUNNER.runHostFilterJob(false), true);
        if (r && r.error === "busy") return; // ران قبلی هنوز تمام نشده — تیک بعدی
        if (r && !r.skipped && r.checked !== undefined) log(`hostfilter run: checked=${r.checked} changed=${r.changed} pushed=${r._pushed}`);
      });
    } catch (e) {
      log("hostfilter job register failed:", String(e));
    }
    // فاز ۴: پول نود، مانیتور مصرف، مانیتور سرور، دایجست پنل، SSL و انقضای دامنه.
    // همه از قفل تکی runHfExclusive می‌گذرند (هیچ دو رانی هم‌زمان نیست) و خودِ رانر
    // mode را چک می‌کند (اگر ورکر به حالت worker برگشته باشد ران skip می‌شود تا هم‌پوشانی نشود).
    // گیت‌های درشت‌دانه (ساعتی/روزانه) با state محلی است تا ورکر cron_state را کثیف نکنیم.
    const phase4 = [
      ["nodepoll", 60 * 1000, 0, "nodepoll"],
      ["umon", 5 * 60 * 1000, 0, "umon"],
      ["srvmon", 5 * 60 * 1000, 0, "srvmon"],
      ["pgdigest_usage", 10 * 60 * 1000, 55 * 60 * 1000, "pgdigest_usage"],
      ["pgdigest_days", 60 * 60 * 1000, 20 * 3600000, "pgdigest_days"],
      ["sslmon", 60 * 60 * 1000, 20 * 3600000, "sslmon"],
      ["domexpmon", 60 * 60 * 1000, 20 * 3600000, "domexpmon"],
    ];
    for (const [job, intervalMs, gateMs, action] of phase4) {
      try {
        registerJob(job, intervalMs, async () => {
          if (gateMs > 0) {
            const lk = `agent-last-${job}`;
            const last = Number(stateGet(lk, 0)) || 0;
            if (Date.now() - last < gateMs) return;
            statePut(lk, Date.now());
          }
          const r = await runHfExclusive(() => RUNNER.runRemoteJob(action, {}), true);
          if (r && (r.error === "busy" || r.skipped)) return;
          if (r && (r._pushed || r._conflicts)) log(`${job} run: pushed=${r._pushed} conflicts=${r._conflicts}`);
        });
      } catch (e) {
        log(`${job} job register failed:`, String(e));
      }
    }
  } else if (!RUNNER) {
    log("running without runner (heartbeat + exec only)");
  } else {
    log("runner loaded but idle: set WORKER_URL + GUARDIAN_TOKEN to enable jobs");
  }
  startJobLoop();
  startHeartbeat();
  server.listen(PORT, () => {
    log(`guardian-agent v${AGENT_VERSION} listening on :${PORT} (worker: ${WORKER_URL || "not set"})`);
    sendHeartbeat().catch((e) => log("heartbeat:", String(e)));
  });
}
process.on("SIGTERM", () => { log("SIGTERM, exiting"); process.exit(0); });
process.on("SIGINT", () => { log("SIGINT, exiting"); process.exit(0); });

// مستقیم اجرا شد (node guardian-agent.js) → روشن شو؛ require شد (تست/رانر) → فقط توابع
if (require.main === module) start();

// برای تست واحد (node guardian-agent.js سرور را بالا می‌آورد؛ require فقط توابع می‌دهد)
module.exports = { AGENT_VERSION, registerJob, stateGet, statePut, start, readRuntimeKey, runHfExclusive, handleUpdate };
