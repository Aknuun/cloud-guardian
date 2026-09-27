#!/usr/bin/env python3
"""حسابرسی استانداردهای UI ربات (docs/ui-standards.md) — خطا => exit 1 (توقف دپلوی)."""
import re
import sys

SRC = sys.argv[1] if len(sys.argv) > 1 else "worker.js"
c = open(SRC, encoding="utf-8").read()
lines = c.split("\n")
errors, warns = [], []

# 1) دو خانه در یک ردیف
for i, l in enumerate(lines):
    if l.count('callback_data: "menu"') >= 2:
        errors.append(f"L{i+1}: two home buttons in one row")
# 2) دو بازگشت هم‌مقصد در یک ردیف
for i, l in enumerate(lines):
    cbs = re.findall(r'callback_data: (`[^`]*`|"[^"]*")', l)
    backs = [x for x in cbs if "back" in x.lower() or "بازگشت" in l]
    if len([x for x in re.findall(r'\{ text: "🔙 بازگشت"', l)]) >= 2:
        errors.append(f"L{i+1}: two back buttons in one row")
# 3) [بازگشت→menu] تنها (باید خانه باشد)
for i, l in enumerate(lines):
    if re.search(r'\[\{ text: "🔙 بازگشت", callback_data: "menu" \}\]', l):
        errors.append(f"L{i+1}: lone back-to-menu (must be home)")
# 4) خانه تنها پشت سر ردیف خانه‌دار (تکراری انباشته)
for i, l in enumerate(lines):
    if re.search(r'\[\{ text: "🏠 خانه", callback_data: "menu" \}\]', l):
        prev = lines[i - 1] if i > 0 else ""
        if "🏠 خانه" in prev and 'callback_data: "menu"' in prev and "?" not in lines[max(0, i - 2)]:
            # استثنا: شاخه‌های شرطی (ternary) مجازند
            seg = "\n".join(lines[max(0, i - 3):i + 1])
            if "?" not in seg and ":" not in prev.split("🏠")[0][-20:]:
                warns.append(f"L{i+1}: possible stacked duplicate home (check ternary)")
# 5) دکمه حذف مستقیم به اجراکننده (باید از «بله» بگذرد)
EXECS = ["dy:", "bulkdely:", "sdy:", "srvdelxok:", "srvpwdely:", "pnlxx:", "arvdelok:",
         "zmaildely:", "ztrafdely:", "trazdely:", "daccy:"]
for i, l in enumerate(lines):
    m = re.search(r'\{ text: "([^"]*(?:حذف|🗑)[^"]*)", callback_data: `\?(\w+:)', l)
    if m and m.group(2) in EXECS and "بله" not in m.group(1):
        errors.append(f"L{i+1}: direct delete button to executor {m.group(2)} (needs confirm)")
# 6) back سراسری و مکانیزم تاریخچه سالم است؟
for need in ['data === "back"', "navhist:", "navRecOk", "srvGetBack"]:
    if need not in c:
        warns.append(f"missing mechanism piece: {need}")

# 7) بک‌اند: همه fetchها signal داشته باشند
for m in re.finditer(r'(?<![\w.])fetch\(', c):
    i = m.end(); depth = 1
    while i < len(c) and depth > 0:
        if c[i] == '(': depth += 1
        elif c[i] == ')': depth -= 1
        i += 1
    call = c[m.start():i]
    if 'signal' not in call and 'fetch(request' not in call:
        ln = c[:m.start()].count('\n') + 1
        errors.append(f"L{ln}: fetch without timeout signal")
# 8) بک‌اند: کلیدهای گذرا TTL داشته باشند (فقط putها چک می‌شوند)
for m in re.finditer(r'kv\.put\(', c):
    i = m.end(); depth = 1
    while i < len(c) and depth > 0:
        if c[i] == '(': depth += 1
        elif c[i] == ')': depth -= 1
        i += 1
    call = c[m.start():i]
    if ('"hf_progress"' in call or '"selfup_backoff_until"' in call) and 'expirationTtl' not in call:
        ln = c[:m.start()].count('\n') + 1
        errors.append(f"L{ln}: transient key without TTL")

print(f"audit-ui: {len(errors)} errors, {len(warns)} warnings")
for w in warns:
    print("  WARN:", w)
for e in errors:
    print("  ERR:", e)
sys.exit(1 if errors else 0)
