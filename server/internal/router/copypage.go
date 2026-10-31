package router

import (
	"net/http"
	"net/url"
	"strings"
)

// copyPage is a tiny handoff page: it copies the service username so the user
// can paste it straight into their renewal bot, then opens that bot.
// Telegram gives no way to prefill another bot's chat, so this is the closest.
func (r *Router) copyPage(w http.ResponseWriter, req *http.Request) {
	user := strings.TrimSpace(req.URL.Query().Get("u"))
	bot := strings.TrimLeft(strings.TrimSpace(req.URL.Query().Get("b")), "@")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if user == "" {
		http.Error(w, "username missing", http.StatusBadRequest)
		return
	}
	esc := func(s string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(s)
	}
	botURL := "https://t.me/" + url.PathEscape(bot)
	page := `<!doctype html><html dir="rtl" lang="fa"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>تمدید سرویس</title>
<style>
:root{color-scheme:light dark}
body{margin:0;min-height:100dvh;display:grid;place-items:center;background:#fff;color:#111114;
 font-family:Vazirmatn,system-ui,-apple-system,"Segoe UI",sans-serif;padding:20px;box-sizing:border-box}
@media (prefers-color-scheme:dark){body{background:#141920;color:#e7e9ec}}
.card{width:100%;max-width:420px;text-align:center}
h1{font-size:16px;font-weight:600;margin:0 0 6px}
p{margin:0 0 18px;font-size:13px;opacity:.7;line-height:1.9}
.code{display:block;width:100%;box-sizing:border-box;padding:16px;border:1px solid rgba(0,0,0,.12);
 border-radius:14px;background:rgba(0,0,0,.04);font:600 17px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;
 direction:ltr;word-break:break-all;margin-bottom:14px}
@media (prefers-color-scheme:dark){.code{border-color:rgba(255,255,255,.14);background:rgba(255,255,255,.06)}}
button,a.btn{display:block;width:100%;box-sizing:border-box;padding:15px;border:0;border-radius:14px;
 font:inherit;font-size:14px;font-weight:600;text-decoration:none;cursor:pointer}
button{background:#1f5eff;color:#fff;margin-bottom:10px}
a.btn{background:rgba(0,0,0,.05);color:inherit}
@media (prefers-color-scheme:dark){a.btn{background:rgba(255,255,255,.08)}}
.ok{margin-top:12px;font-size:12px;color:#137a3f;min-height:16px}
</style></head><body><div class="card">
<h1>تمدید سرویس</h1>
<p>یوزرنیم زیر را کپی کن، سپس در ربات تمدید بفرست.</p>
<span class="code" id="u">` + esc(user) + `</span>
<button id="c" type="button">کپی یوزرنیم</button>
<a class="btn" id="go" href="` + esc(botURL) + `">باز کردن ربات تمدید</a>
<div class="ok" id="m"></div>
</div><script>
var u=` + "`" + strings.ReplaceAll(user, "`", "") + "`" + `;var m=document.getElementById('m');
function done(){m.textContent='کپی شد — حالا در ربات بفرست';}
function copy(){var ok=false;
 try{var t=document.createElement('textarea');t.value=u;t.style.position='fixed';t.style.opacity='0';
 document.body.appendChild(t);t.select();ok=document.execCommand('copy');document.body.removeChild(t);}catch(e){}
 if(navigator.clipboard&&navigator.clipboard.writeText){navigator.clipboard.writeText(u).then(done).catch(function(){if(ok)done();});}
 else if(ok){done();}
}
document.getElementById('c').addEventListener('click',copy);
copy();
</script></body></html>`
	w.Write([]byte(page))
}
