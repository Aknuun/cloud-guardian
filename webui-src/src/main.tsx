import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './app.css'
import './deepseek.css'
import './utilities.css'
import './panel.css'
import { initMode, ThemeProvider } from './components/theme'
import App from './App'

initMode()

// Anything that kills the bundle before render used to leave a blank page.
// Show the reason instead so a stuck client can report it.
let fatalShown = false
function fatal(msg: string) {
  const root = document.getElementById('root')
  if (!root || fatalShown) return
  fatalShown = true
  root.innerHTML =
    '<div style="font-family:Vazirmatn,sans-serif;direction:rtl;padding:24px;color:#111">' +
    '<div style="font-weight:700;margin-bottom:8px">خطا در بارگذاری مینی‌اپ</div>' +
    '<div style="font-size:13px;color:#666;line-height:1.9;word-break:break-word">' +
    String(msg).replace(/[<>&]/g, (c) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' })[c] as string) +
    '</div>' +
    '<button onclick="location.reload()" style="margin-top:16px;padding:10px 18px;border:0;border-radius:10px;background:#1f5eff;color:#fff;font:inherit">تلاش دوباره</button>' +
    '</div>'
}
window.addEventListener('error', (e) => fatal((e.message || 'خطای نامشخص') + (e.filename ? ' @ ' + e.filename : '')))
window.addEventListener('unhandledrejection', (e: any) => fatal(e?.reason?.message || String(e?.reason || 'خطای نامشخص')))

// telegram.org can be slow or blocked; never let it hold the first paint hostage.

// Number inputs swallow the wheel while focused; blur them so the page scrolls instead.
window.addEventListener(
  'wheel',
  (e) => {
    const t = e.target as HTMLElement | null
    if (t && t.tagName === 'INPUT' && (t as HTMLInputElement).type === 'number' && document.activeElement === t) {
      t.blur()
    }
  },
  { passive: true },
)

async function waitTelegram(ms: number) {
  const t0 = Date.now()
  while (!(window as any).Telegram && Date.now() - t0 < ms) {
    await new Promise((r) => setTimeout(r, 100))
  }
}

// Telegram hands initData in the URL hash; if we can see it, its script must win.
const needsTg = typeof location !== 'undefined' && /tgWebAppData=/.test(location.hash)
waitTelegram(needsTg ? 8000 : 1500).then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </StrictMode>,
  )
})
