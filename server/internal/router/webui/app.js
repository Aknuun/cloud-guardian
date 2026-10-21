'use strict';

const tg = window.Telegram && window.Telegram.WebApp ? window.Telegram.WebApp : null;
const $ = (s) => document.querySelector(s);
const app = $('#app');

const S = {
  token: sessionStorage.getItem('cg_token') || '',
  accounts: [],
  servers: [],
  server: null,
  ips: [],
  optsCache: {},
  q: '',
  cur: { view: 'accounts', acc: null, id: null },
  stack: [],
};

const STATUS = {
  running: 'روشن', off: 'خاموش', starting: 'در حال روشن شدن', stopping: 'در حال خاموش شدن',
  rebuilding: 'ریبیلد', migrating: 'انتقال', deleting: 'در حال حذف', unknown: 'نامشخص',
};

function esc(v) {
  return String(v == null ? '' : v).replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}
function num(v) { return Number(v || 0).toLocaleString('en-US'); }
function gb(v) { const n = Number(v || 0); return n >= 1000 ? (n / 1000).toFixed(1) + ' TB' : n.toFixed(n < 10 ? 1 : 0) + ' GB'; }
function dotClass(st) {
  if (st === 'running') return 'running';
  if (st === 'off') return 'off';
  if (st === 'starting' || st === 'migrating' || st === 'rebuilding') return 'starting';
  if (st === 'unknown') return 'error';
  return '';
}
function addr(s) {
  if (s.ipv4 && s.ipv6) return s.ipv4 + ' / ' + s.ipv6;
  return s.ipv4 || s.ipv6 || 'بدون آی‌پی';
}

function toast(msg, isErr) {
  const el = $('#toast');
  el.textContent = msg;
  el.className = 'toast' + (isErr ? ' err' : '');
  clearTimeout(toast._t);
  toast._t = setTimeout(() => el.classList.add('hidden'), 3200);
}
function loading(on) { $('#loader').classList.toggle('hidden', !on); }

async function api(path, body) {
  const headers = { Authorization: 'Bearer ' + S.token };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  const res = await fetch(path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
  let j = null;
  try { j = await res.json(); } catch (_) {}
  if (res.status === 401) {
    sessionStorage.removeItem('cg_token');
    S.token = '';
    throw new Error((j && j.error) || 'نشست منقضی شده');
  }
  if (!res.ok || !j || j.ok === false) throw new Error((j && j.error) || 'خطای سرور (' + res.status + ')');
  return j.data;
}

async function boot() {
  if (tg) {
    tg.ready();
    tg.expand();
    tg.disableVerticalSwipes && tg.disableVerticalSwipes();
    tg.BackButton.onClick(back);
    tg.MainButton.hide();
    applyTheme();
  }
  if (!tg || !tg.initData) {
    app.innerHTML = '<div class="empty"><b>داخل تلگرام باز کن</b>این مینی‌اپ فقط از داخل ربات تلگرام کار می‌کند.</div>';
    return;
  }
  $('#refresh').classList.remove('hidden');
  $('#refresh').onclick = () => reload(true);
  if (S.token) {
    try { await api('/api/me'); return render(); } catch (_) { /* re-auth */ }
  }
  loading(true);
  try {
    const res = await fetch('/api/auth', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ initData: tg.initData }),
    });
    const j = await res.json();
    if (!res.ok || !j.ok) throw new Error(j.error || 'ورود ناموفق');
    S.token = j.data.token;
    sessionStorage.setItem('cg_token', S.token);
    render();
  } catch (e) {
    app.innerHTML = '<div class="empty"><b>دسترسی نداریم</b>' + esc(e.message) + '</div>';
  } finally { loading(false); }
}

function applyTheme() {
  const p = (tg && tg.themeParams) || {};
  const root = document.documentElement.style;
  const map = {
    bg_color: '--tg-theme-bg-color', secondary_bg_color: '--tg-theme-secondary-bg-color',
    text_color: '--tg-theme-text-color', hint_color: '--tg-theme-hint-color',
    link_color: '--tg-theme-link-color', button_color: '--tg-theme-button-color',
    button_text_color: '--tg-theme-button-text-color',
  };
  for (const k in map) if (p[k]) root.setProperty(map[k], p[k]);
  if (tg && tg.colorScheme === 'light') {
    root.setProperty('--line', 'rgba(0,0,0,.07)');
    root.setProperty('--bg2', p.secondary_bg_color || '#f1f3f5');
  }
}

function nav(view, extra) {
  S.stack.push({ ...S.cur });
  S.cur = { ...S.cur, ...extra, view };
  S.q = '';
  render();
}
function repaint() { S.cur = { ...S.cur }; render(); }
function back() {
  if ($('#modal').classList.contains('hidden') === false) return closeModal();
  if (S.cur.view === 'create') {
    const c = createState();
    if (c.step > 1) {
      c.step--;
      if (c.step === 1) { c.loc = ''; c.type = ''; c.arch = ''; c.image = ''; }
      else if (c.step === 2) { c.type = ''; c.arch = ''; c.image = ''; }
      else if (c.step === 3) { c.image = ''; }
      return render();
    }
  }
  const p = S.stack.pop();
  if (!p) { if (tg) tg.close(); return; }
  S.cur = p;
  S.q = '';
  render();
}
function title(t) { $('#title').textContent = t; }
function chrome(showBack) {
  $('#back').classList.toggle('hidden', !showBack);
  if (tg) showBack ? tg.BackButton.show() : tg.BackButton.hide();
}

async function render() {
  const v = S.cur.view;
  chrome(v !== 'accounts');
  try {
    if (v === 'accounts') await viewAccounts();
    else if (v === 'servers') await viewServers();
    else if (v === 'server') await viewServer();
    else if (v === 'ips') await viewIPs();
    else if (v === 'create') await viewCreate();
  } catch (e) {
    toast(e.message, true);
    app.innerHTML = '<div class="empty"><b>خطا</b>' + esc(e.message) + '</div>';
  }
}

async function reload(silent) {
  if (S.cur.view === 'create') return render();
  if (!silent) loading(true);
  try { await render(); } catch (e) { toast(e.message, true); } finally { loading(false); }
}

/* ---------------- accounts ---------------- */

async function viewAccounts() {
  title('هتزنر');
  const d = await api('/api/hz/accounts');
  S.accounts = d.accounts || [];
  if (!S.accounts.length) {
    app.innerHTML = '<div class="empty"><b>اکانتی ثبت نشده</b>برای افزودن اکانت هتزنر از ربات استفاده کن.</div>';
    return;
  }
  const rows = S.accounts.map((a) => `
    <div class="card tap" data-a="acc" data-i="${a.i}">
      <div class="dot running"></div>
      <div class="grow"><div class="name">${esc(a.name || 'اکانت ' + (a.i + 1))}</div>
        <div class="sub">${a.count === undefined ? 'برای دیدن سرورها بزن' : num(a.count) + ' سرور'}</div></div>
      <div class="chev">›</div>
    </div>`).join('');
  app.innerHTML = rows + '<div class="empty" style="padding:18px">هر اکانت را انتخاب کن تا سرورها و آی‌پی‌ها را ببینی.</div>';
}

/* ---------------- servers ---------------- */

function accountName(i) {
  const a = S.accounts.find((x) => x.i === i);
  return a ? a.name : '';
}

async function viewServers() {
  title('سرورها' + (accountName(S.cur.acc) ? ' · ' + accountName(S.cur.acc) : ''));
  if (!S.servers.length || S.cur.force) {
    if (!S.servers.length) loading(true);
    try {
      const d = await api('/api/hz/servers?acc=' + S.cur.acc);
      S.servers = d.servers || [];
    } finally { loading(false); }
  }
  const q = S.q.trim().toLowerCase();
  const list = S.servers.filter((s) => !q ||
    (s.name + ' ' + s.ipv4 + ' ' + s.ipv6 + ' ' + (s.type && s.type.name)).toLowerCase().includes(q));
  const rows = list.map((s) => `
    <div class="card tap" data-a="srv" data-id="${esc(s.id)}">
      <div class="dot ${dotClass(s.status)}"></div>
      <div class="grow">
        <div class="name">${esc(s.name)}</div>
        <div class="sub">${esc((s.type && s.type.name) || '')} · ${s.type ? s.type.cores + 'vCPU / ' + gb(s.type.memory) + ' / ' + s.type.disk + 'GB' : ''}</div>
        <div class="sub mono" style="margin-top:2px">${esc(addr(s))}</div>
      </div>
      <div style="text-align:left">
        <div class="badge ${s.status === 'running' ? 'ok' : ''}">${esc(STATUS[s.status] || s.status)}</div>
        <div class="sub" style="margin-top:4px">${esc((s.location && (s.location.city || s.location.name)) || '')}</div>
      </div>
    </div>`).join('');
  app.innerHTML = `
    <input id="search" class="search" placeholder="جست‌وجوی نام یا آی‌پی…" value="${esc(S.q)}">
    <div class="grid2" style="margin-bottom:10px">
      <button class="btn blue" data-a="ips">🌐 آی‌پی‌های تمیز</button>
      <button class="btn primary" data-a="create">➕ ساخت سرور</button>
    </div>
    <div class="section-title">${list.length} سرور</div>
    ${rows || '<div class="empty">سروری پیدا نشد.</div>'}`;
  const inp = $('#search');
  inp.oninput = () => {
    S.q = inp.value;
    const st = inp.selectionStart;
    render().then(() => { const el = $('#search'); if (el) { el.focus(); el.setSelectionRange(st, st); } });
  };
}

/* ---------------- server detail ---------------- */

async function viewServer() {
  title('جزئیات سرور');
  loading(true);
  let d;
  try { d = await api('/api/hz/server?acc=' + S.cur.acc + '&id=' + encodeURIComponent(S.cur.id)); }
  finally { loading(false); }
  const s = d.server;
  S.server = s;
  const st = s.status;
  const tr = s.traffic || {};
  const pct = Math.min(100, Number(tr.used_pct || 0));
  const barCls = pct >= 90 ? 'hot' : pct >= 70 ? 'warm' : '';
  const snapRows = (s.snapshots || []).map((sn) => `
    <div class="card">
      <div class="grow"><div class="name">${esc(sn.name || sn.id)}</div>
        <div class="sub">${gb(sn.size_gb)} · ${esc((sn.created || '').slice(0, 10))}</div></div>
      <button class="btn red" data-a="snapdel" data-id="${esc(s.id)}" data-arg="${esc(sn.id)}">حذف</button>
    </div>`).join('');

  app.innerHTML = `
    <div class="card">
      <div class="dot ${dotClass(st)}"></div>
      <div class="grow">
        <div class="name">${esc(s.name)}</div>
        <div class="sub">${esc(s.image || '')} · ${esc(s.location ? s.location.city || s.location.name : '')} · ${esc(s.type ? s.type.id : '')}</div>
      </div>
      <span class="badge ${st === 'running' ? 'ok' : ''}">${esc(STATUS[st] || st)}</span>
    </div>

    <div class="card" style="display:block">
      <div class="kv"><span>IPv4</span><span class="mono">${esc(s.ipv4 || '—')}</span></div>
      <div class="kv"><span>IPv6</span><span class="mono" style="font-size:12px">${esc(s.ipv6 || '—')}</span></div>
      <div class="kv"><span>پلن</span><span>${s.type ? esc(s.type.name) + ' · ' + s.type.cores + 'vCPU / ' + gb(s.type.memory) + ' / ' + s.type.disk + 'GB' : '—'}</span></div>
      <div class="kv"><span>لوکیشن</span><span>${esc(s.location ? (s.location.city || '') + '، ' + (s.location.country || '') : '—')}</span></div>
      <div class="kv"><span>ساخت</span><span>${esc((s.created || '').slice(0, 10) || '—')}</span></div>
      <div class="kv"><span>هزینه</span><span>${s.prices && s.prices.monthly ? esc(s.prices.monthly) + ' ماهانه' : '—'}</span></div>
    </div>

    <div class="card" style="display:block">
      <div class="kv"><span>ترافیک مصرفی</span><span>${gb(tr.total_gb)} از ${gb(tr.included_gb)}</span></div>
      <div class="bar"><i class="${barCls}" style="width:${pct}%"></i></div>
      <div class="sub">${num(pct)}٪ مصرف${tr.extra_gb > 0 ? ' · ' + gb(tr.extra_gb) + ' اضافه' : ''}</div>
    </div>

    <div class="actions">
      ${st === 'running'
        ? '<button class="btn" data-a="act" data-op="poweroff" data-id="' + esc(s.id) + '">⏻ خاموش</button>'
        : '<button class="btn green" data-a="act" data-op="poweron" data-id="' + esc(s.id) + '">⏻ روشن</button>'}
      <button class="btn" data-a="act" data-op="reboot" data-id="${esc(s.id)}">↻ ری‌بوت</button>
      <button class="btn" data-a="act" data-op="reset" data-id="${esc(s.id)}" data-confirm="1" data-title="ریست سخت">⟳ ریست سخت</button>
      <button class="btn" data-a="act" data-op="reset_password" data-id="${esc(s.id)}" data-confirm="1" data-title="ریست رمز روت">🔑 رمز روت</button>
      <button class="btn" data-a="rename" data-id="${esc(s.id)}">✎ تغییر نام</button>
      <button class="btn" data-a="snapnew" data-id="${esc(s.id)}">📸 اسنپ‌شات</button>
      <button class="btn blue" data-a="rebuild" data-id="${esc(s.id)}">💿 ریبیلد</button>
      <button class="btn blue" data-a="resize" data-id="${esc(s.id)}">⬆ ارتقا</button>
      <button class="btn blue" data-a="ipassign" data-id="${esc(s.id)}">🌐 اتصال IP</button>
      <button class="btn blue" data-a="act" data-op="ip_replace" data-id="${esc(s.id)}" data-confirm="1" data-title="جایگزینی IPv4">♻ تعویض IPv4</button>
      <button class="btn wide" data-a="act" data-op="ip_unassign" data-id="${esc(s.id)}" data-arg="ipv4" data-confirm="1" data-title="حذف IPv4 اختصاصی">✂ حذف IPv4 اختصاصی</button>
      <button class="btn wide" data-a="act" data-op="ip_unassign" data-id="${esc(s.id)}" data-arg="ipv6" data-confirm="1" data-title="حذف IPv6 اختصاصی">✂ حذف IPv6 اختصاصی</button>
      <button class="btn red wide" data-a="act" data-op="delete" data-id="${esc(s.id)}" data-confirm="1" data-title="حذف سرور">🗑 حذف سرور</button>
    </div>

    <div class="section-title">اسنپ‌شات‌ها (${(s.snapshots || []).length})</div>
    ${snapRows || '<div class="empty" style="padding:16px">اسنپ‌شاتی نیست.</div>'}
  `;
}

/* ---------------- IP pool ---------------- */

async function viewIPs() {
  title('آی‌پی‌های تمیز');
  loading(true);
  try {
    const d = await api('/api/hz/ips?acc=' + S.cur.acc);
    S.ips = d.ips || [];
  } finally { loading(false); }
  const rows = S.ips.map((p) => {
    const c = p.check;
    let badge = '<span class="badge">بررسی نشده</span>';
    if (c) badge = c.clean
      ? '<span class="badge ok">تمیز</span>'
      : (c.listed && c.listed.length ? '<span class="badge bad">لیست‌شده</span>' : '<span class="badge warn">نامشخص</span>');
    const busy = !!p.assignee;
    return `<div class="card">
      <div class="grow">
        <div class="name mono">${esc(p.ip)}</div>
        <div class="sub">${esc(p.loc || '')}${busy ? ' · ' + esc(p.assignee) : ' · آزاد'}${c && c.listed && c.listed.length ? ' · ' + esc(c.listed.slice(0, 2).join(', ')) : ''}</div>
      </div>
      ${badge}
      ${busy ? '' : `<button class="btn blue" data-a="ipfree" data-ip="${p.id}">اتصال</button>`}
    </div>`;
  }).join('');
  app.innerHTML = `
    <div class="grid2" style="margin-bottom:10px">
      <button class="btn" data-a="back-servers">→ سرورها</button>
      <button class="btn" data-a="refreships">↻ بروزرسانی</button>
    </div>
    ${rows || '<div class="empty">آی‌پی‌ای در استخر نیست.</div>'}`;
}

/* ---------------- create wizard ---------------- */

function createState() {
  if (!S.cur.create) S.cur.create = { step: 1, loc: '', type: '', image: '', name: '', arch: '' };
  return S.cur.create;
}

async function options(loc, arch) {
  const key = loc + '|' + arch;
  if (!S.optsCache[key]) {
    S.optsCache[key] = await api('/api/hz/options?acc=' + S.cur.acc +
      (loc ? '&loc=' + encodeURIComponent(loc) : '') +
      (arch ? '&arch=' + encodeURIComponent(arch) : ''));
  }
  return S.optsCache[key];
}

async function viewCreate() {
  title('ساخت سرور');
  const c = createState();
  loading(true);
  let body = '';
  try {
    if (c.step === 1) {
      const o = await options('', '');
      body = '<div class="section-title">لوکیشن</div>' + (o.locations.map((l) => `
        <div class="choice" data-a="cloc" data-v="${esc(l.name)}">
          <div class="grow"><div>${esc(l.city || l.name)}</div><div class="sub">${esc(l.country || '')} · ${esc(l.name)}</div></div>
        </div>`).join(''));
    } else if (c.step === 2) {
      const o = await options(c.loc, '');
      body = `<div class="section-title">پلن · ${esc(c.loc)}</div>` + (o.types.length ? o.types.map((t) => `
        <div class="choice" data-a="ctype" data-v="${esc(t.id)}" data-arch="${esc(t.architecture)}">
          <div class="grow"><div>${esc(t.name)}</div><div class="sub">${t.cores} vCPU · ${gb(t.memory)} RAM · ${t.disk}GB · ${esc(t.architecture)}</div></div>
          <div>${esc(t.monthly || '')}</div>
        </div>`).join('') : '<div class="empty">پلنی برای این لوکیشن نیست.</div>');
    } else if (c.step === 3) {
      const o = await options(c.loc, c.arch);
      body = '<div class="section-title">سیستم‌عامل</div>' + (o.images.map((im) => `
        <div class="choice" data-a="cimg" data-v="${esc(im.id)}">
          <div class="grow"><div>${esc(im.name)}</div><div class="sub">${esc(im.architecture)} · ${esc(im.os_flavor || '')}</div></div>
        </div>`).join(''));
    } else {
      c.name = c.name || '';
      body = `
        <div class="section-title">نام و تأیید</div>
        <input id="cname" class="input" placeholder="نام سرور (مثلاً de-1)" value="${esc(c.name)}" maxlength="63">
        <div class="card" style="display:block">
          <div class="kv"><span>لوکیشن</span><span>${esc(c.loc)}</span></div>
          <div class="kv"><span>پلن</span><span>${esc(c.type)}</span></div>
          <div class="kv"><span>تصویر</span><span class="mono">${esc(c.image)}</span></div>
          <div class="kv"><span>سیستم</span><span>Ubuntu + رمز روت تصادفی</span></div>
        </div>
        <button class="btn primary wide" data-a="ccreate">🚀 ساخت سرور</button>`;
    }
  } finally { loading(false); }
  app.innerHTML = `<div class="steps"><i class="${c.step >= 1 ? 'on' : ''}"></i><i class="${c.step >= 2 ? 'on' : ''}"></i><i class="${c.step >= 3 ? 'on' : ''}"></i><i class="${c.step >= 4 ? 'on' : ''}"></i></div>` + body;
}

/* ---------------- modals ---------------- */

function openModal(html) {
  $('#modal-body').innerHTML = html;
  $('#modal').classList.remove('hidden');
}
function closeModal() { $('#modal').classList.add('hidden'); $('#modal-body').innerHTML = ''; }

function confirmBox(title, desc, cb) {
  openModal(`<h3>${esc(title)}</h3><p>${esc(desc || '')}</p>
    <div class="modal-actions">
      <button class="btn" data-a="mclose">انصراف</button>
      <button class="btn red" id="mok">تأیید</button>
    </div>`);
  $('#mok').onclick = async () => { closeModal(); await cb(); };
}

function promptBox(title, placeholder, value, cb) {
  openModal(`<h3>${esc(title)}</h3>
    <input id="minput" class="input" placeholder="${esc(placeholder)}" value="${esc(value || '')}">
    <div class="modal-actions">
      <button class="btn" data-a="mclose">انصراف</button>
      <button class="btn primary" id="mok">تأیید</button>
    </div>`);
  const inp = $('#minput');
  inp.focus();
  $('#mok').onclick = async () => { const v = inp.value.trim(); closeModal(); await cb(v); };
}

function pickerBox(title, items, render, cb) {
  openModal(`<h3>${esc(title)}</h3>
    <div>${items.map(render).join('') || '<div class="empty">گزینه‌ای نیست.</div>'}</div>
    <div class="modal-actions"><button class="btn wide" data-a="mclose">انصراف</button></div>`);
  $('#modal-body').querySelectorAll('[data-pick]').forEach((el) => {
    el.onclick = () => { const v = el.getAttribute('data-pick'); closeModal(); cb(v, el); };
  });
}

/* ---------------- actions ---------------- */

async function doAction(opts) {
  loading(true);
  try {
    const d = await api('/api/hz/action', {
      acc: S.cur.acc, id: opts.id, action: opts.op,
      arg: opts.arg || '', ipId: Number(opts.ipId || 0), confirm: true,
    });
    if (d && d.password) {
      openModal(`<h3>رمز روت جدید</h3><div class="secret mono">${esc(d.password)}</div>
        <div class="modal-actions"><button class="btn" data-a="copy" data-v="${esc(d.password)}">کپی</button>
        <button class="btn primary" data-a="mclose">باشه</button></div>`);
    } else {
      toast((d && d.message) || 'انجام شد');
    }
    S.servers = [];
    await render();
  } catch (e) { toast(e.message, true); }
  finally { loading(false); }
}

function actionFromEl(el) {
  const op = el.getAttribute('data-op');
  const id = el.getAttribute('data-id');
  const arg = el.getAttribute('data-arg') || '';
  const run = () => doAction({ op, id, arg, ipId: el.getAttribute('data-ip') });
  if (el.getAttribute('data-confirm')) {
    const label = el.getAttribute('data-title') || 'تأیید عملیات';
    const name = (S.server && S.server.name) || id;
    confirmBox(label, (op === 'delete' ? 'سرور «' + name + '» برای همیشه حذف می‌شود.' : 'عملیات «' + label + '» روی «' + name + '» انجام شود؟'), run);
  } else run();
}

const handlers = {
  acc(el) { S.cur.acc = Number(el.getAttribute('data-i')); S.servers = []; nav('servers'); },
  srv(el) { S.cur.id = el.getAttribute('data-id'); S.server = null; nav('server'); },
  ips() { nav('ips', { acc: S.cur.acc }); },
  create() { nav('create', { create: { step: 1, loc: '', type: '', image: '', name: '', arch: '' } }); },
  act(el) { actionFromEl(el); },
  rename(el) {
    const id = el.getAttribute('data-id');
    promptBox('نام جدید سرور', 'مثلاً de-2', S.server && S.server.name, (v) => {
      if (!v) return toast('نام خالی است', true);
      doAction({ op: 'rename', id, arg: v });
    });
  },
  snapnew(el) {
    const id = el.getAttribute('data-id');
    promptBox('توضیح اسنپ‌شات', 'اختیاری', '', (v) => doAction({ op: 'snapshot_create', id, arg: v }));
  },
  snapdel(el) {
    const id = el.getAttribute('data-id');
    const snapID = el.getAttribute('data-arg');
    confirmBox('حذف اسنپ‌شات', 'این اسنپ‌شات برای همیشه حذف می‌شود.', () => doAction({ op: 'snapshot_delete', id, arg: snapID }));
  },
  rebuild(el) {
    const id = el.getAttribute('data-id');
    const s = S.server;
    if (!s) return;
    (async () => {
      loading(true);
      try {
        const o = await options(s.location.name, s.type.architecture);
        pickerBox('انتخاب تصویر برای ریبیلد', o.images,
          (im) => `<div class="choice" data-pick="${esc(im.id)}"><div class="grow"><div>${esc(im.name)}</div><div class="sub">${esc(im.architecture)}</div></div></div>`,
          (img) => confirmBox('ریبیلد سرور', 'دیسک سرور پاک و تصویر تازه نصب می‌شود. ادامه؟', () => doAction({ op: 'rebuild', id, arg: img })));
      } catch (e) { toast(e.message, true); } finally { loading(false); }
    })();
  },
  resize(el) {
    const id = el.getAttribute('data-id');
    const s = S.server;
    if (!s) return;
    (async () => {
      loading(true);
      try {
        const o = await options(s.location.name, '');
        const types = o.types.filter((t) => t.architecture === s.type.architecture);
        pickerBox('ارتقای پلن (سرور باید خاموش باشد)', types,
          (t) => `<div class="choice" data-pick="${esc(t.id)}"><div class="grow"><div>${esc(t.name)}</div><div class="sub">${t.cores} vCPU · ${gb(t.memory)} · ${t.disk}GB</div></div><div>${esc(t.monthly || '')}</div></div>`,
          (tid) => confirmBox('ارتقای پلن', 'نوع سرور تغییر می‌کند و دیسک هم ارتقا می‌یابد. ادامه؟', () => doAction({ op: 'resize', id, arg: tid })));
      } catch (e) { toast(e.message, true); } finally { loading(false); }
    })();
  },
  ipassign(el) {
    const id = el.getAttribute('data-id');
    (async () => {
      loading(true);
      try {
        const d = await api('/api/hz/ips?acc=' + S.cur.acc);
        const free = (d.ips || []).filter((p) => !p.assignee);
        pickerBox('اتصال آی‌پی (سرور خاموش می‌شود)', free,
          (p) => `<div class="choice" data-pick="${p.id}"><div class="grow"><div class="mono">${esc(p.ip)}</div><div class="sub">${esc(p.loc || '')}${p.check ? (p.check.clean ? ' · تمیز' : ' · لیست‌شده') : ''}</div></div></div>`,
          (ipid) => confirmBox('اتصال آی‌پی', 'سرور خاموش، آی‌پی وصل و دوباره روشن می‌شود. ادامه؟', () => doAction({ op: 'ip_assign', id, ipId: ipid })));
      } catch (e) { toast(e.message, true); } finally { loading(false); }
    })();
  },
  ipfree(el) {
    const ipid = el.getAttribute('data-ip');
    (async () => {
      loading(true);
      try {
        if (!S.servers.length) S.servers = (await api('/api/hz/servers?acc=' + S.cur.acc)).servers || [];
        pickerBox('اتصال به کدام سرور؟', S.servers,
          (s) => `<div class="choice" data-pick="${esc(s.id)}"><div class="grow"><div>${esc(s.name)}</div><div class="sub mono">${esc(addr(s))}</div></div></div>`,
          (sid) => confirmBox('اتصال آی‌پی', 'سرور خاموش، آی‌پی وصل و دوباره روشن می‌شود. ادامه؟', () => doAction({ op: 'ip_assign', id: sid, ipId: ipid })));
      } catch (e) { toast(e.message, true); } finally { loading(false); }
    })();
  },
  refreships() { reload(true); },
  'back-servers'() { S.stack = []; S.cur = { ...S.cur, view: 'servers' }; render(); },
  cloc(el) { const c = createState(); c.loc = el.getAttribute('data-v'); c.step = 2; render(); },
  ctype(el) { const c = createState(); c.type = el.getAttribute('data-v'); c.arch = el.getAttribute('data-arch'); c.step = 3; render(); },
  cimg(el) { const c = createState(); c.image = el.getAttribute('data-v'); c.step = 4; render(); },
  ccreate() {
    const c = createState();
    const nameEl = $('#cname');
    const name = nameEl ? nameEl.value.trim() : c.name;
    if (!name) return toast('نام سرور را وارد کن', true);
    c.name = name;
    confirmBox('ساخت سرور', `«${name}» با پلن ${c.type} در ${c.loc} ساخته شود؟`, async () => {
      loading(true);
      try {
        const d = await api('/api/hz/create', { acc: S.cur.acc, name, type: c.type, image: c.image, location: c.loc });
        S.servers = [];
        S.cur.create = null;
        openModal(`<h3>سرور ساخته شد</h3>
          <p>این اطلاعات را ذخیره کن؛ رمز روت فقط یک‌بار نشان داده می‌شود.</p>
          <div class="kv"><span>نام</span><span>${esc(name)}</span></div>
          <div class="kv"><span>IPv4</span><span class="mono">${esc(d.ip || '—')}</span></div>
          <div class="secret mono">${esc(d.password || '')}</div>
          <div class="modal-actions">
            <button class="btn" data-a="copy" data-v="${esc((d.ip || '') + ' root ' + (d.password || ''))}">کپی همه</button>
            <button class="btn primary" data-a="donecreate">باشه</button>
          </div>`);
      } catch (e) { toast(e.message, true); } finally { loading(false); }
    });
  },
  donecreate() { closeModal(); S.stack = []; S.cur = { ...S.cur, view: 'servers', acc: S.cur.acc }; render(); },
  copy(el) {
    const v = el.getAttribute('data-v') || '';
    const done = () => toast('کپی شد');
    if (navigator.clipboard) navigator.clipboard.writeText(v).then(done).catch(() => fallbackCopy(v, done));
    else fallbackCopy(v, done);
  },
  mclose() { closeModal(); },
};

function fallbackCopy(v, done) {
  const ta = document.createElement('textarea');
  ta.value = v; ta.style.position = 'fixed'; ta.style.opacity = '0';
  document.body.appendChild(ta); ta.select();
  try { document.execCommand('copy'); done(); } catch (_) { toast('کپی نشد', true); }
  ta.remove();
}

document.addEventListener('click', (e) => {
  const el = e.target.closest('[data-a]');
  if (!el) {
    if (e.target.id === 'modal') closeModal();
    return;
  }
  const fn = handlers[el.getAttribute('data-a')];
  if (fn) { e.preventDefault(); fn(el); }
});

boot();
