import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

// Every tool page, in the order the sidebar shows them.
const TOOLS = ['home', 'hs', 'pg', 'hook', 'hf', 'hfd', 'um', 'srv', 'dom', 'rem', 'mail', 'cfcheck', 'cfgtest', 'fo', 'hub', 'set']

// The app only needs the session token; each /api call is answered here so the
// smoke test needs no backend, bot token or network access.
const FIXTURES: Record<string, unknown> = {
  '/api/me': { id: 172358305, isMain: true },
  '/api/sessions': { sessions: [], current: '' },
  '/api/tools/settings': { version: '0.0.1', admins: [172358305], guides: [], isMain: true },
  '/api/tools/srv': [],
  '/api/tools/rem': [],
  '/api/tools/pg': { panels: [], monitors: [] },
  '/api/tools/um': { enabled: false, hourlyGb: 1, dailyGb: 1, cooldownH: 1, minGb: 1, activeMin: 1, report: false },
  '/api/tools/dom': { ssl: [], doms: [], key: false, sslOn: true, warnDays: 7 },
  '/api/tools/hf': { cfg: { enabled: false, intervalMin: 25, cities: 3 }, panels: [], excCount: 0 },
  '/api/tools/hfd': { cfg: { enabled: false, intervalH: 24, at: '03:00' }, panels: [], items: [] },
  '/api/tools/hook': { enabled: false, silentMe: false, days: [], usage: [], panels: [] },
  '/api/tools/mail': { fwd: [], worker: [] },
  '/api/tools/fo': {
    configured: false,
    enabled: false,
    worker: '',
    url: '',
    account: '',
    namespace: '',
    health: '',
    publicUrl: '',
    owner: 'other',
    serverUp: false,
    probeCode: 0,
    webhook: '',
  },
  '/api/tools/hub': {
    hubUrl: '',
    annUrl: '',
    secret: false,
    root: false,
    hubOk: true,
    installs: [],
    stats: {
      bot: 'test',
      version: '0.0.1',
      go: 'go1',
      uptime: '1m',
      mem_mb: 1,
      db_mb: 1,
      admins: 1,
      users: 0,
      panels: 0,
      zones: 0,
      cf_accs: 0,
      servers: 0,
    },
  },
  '/api/tools/hs': {
    panels: [],
    templates: [],
    hosts: [],
    overrides: [],
    backups: [],
    audit: [],
    groups: [],
  },
  '/api/cf/zones': { zones: [], accounts: [], favs: [] },
  '/api/tools/cfcheck': { rows: [], checked: 0 },
  '/api/tools/cfgtest': {
    engine: { ok: true, version: 'v26.10.31', path: '/opt/cloud-guardian/bin/xray' },
    limits: { links: 20, samples: 5, timeout_s: 20 },
    targets: ['https://cp.cloudflare.com/generate_204'],
    history: [],
  },
}

async function boot(page: Page, go: string) {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(String(e)))
  await page.addInitScript(() => sessionStorage.setItem('cg_token', 'test-token'))
  await page.route('**/api/**', (route) => {
    const path = new URL(route.request().url()).pathname
    const data = path in FIXTURES ? FIXTURES[path] : []
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, data }),
    })
  })
  await page.goto(`/app/?go=${go}`)
  await expect(page.locator('.cc-topbar')).toBeVisible()
  return errors
}

for (const id of TOOLS) {
  test(`tool page renders: ${id}`, async ({ page }) => {
    const errors = await boot(page, id)
    await expect(page.locator('.cc-title')).not.toHaveText('')
    expect(errors).toEqual([])
  })
}

test('sidebar opens a tool and keeps no console errors', async ({ page }) => {
  const errors = await boot(page, 'home')
  await page.getByRole('button', { name: 'تنظیمات', exact: true }).first().click()
  await expect(page.locator('.cc-title')).toContainText('تنظیمات')
  expect(errors).toEqual([])
})

// Older hub peers may send a partial stats object; the page must not crash.
test('hub page tolerates missing stats fields', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(String(e)))
  await page.addInitScript(() => sessionStorage.setItem('cg_token', 'test-token'))
  await page.route('**/api/**', (route) => {
    const path = new URL(route.request().url()).pathname
    const data = path === '/api/tools/hub' ? { hubUrl: '', annUrl: '', secret: false, hubOk: true, installs: [], stats: {} } : FIXTURES[path] ?? []
    return route.fulfill({ json: { ok: true, data } })
  })
  await page.goto('/app/?go=hub')
  await expect(page.locator('.cc-topbar')).toBeVisible()
  await expect(page.locator('.cc-main')).toContainText('حافظه / دیتابیس')
  expect(errors).toEqual([])
})

// Item 4 of the UI pass: every control must have a name a screen reader can read.
test('every button has an accessible name', async ({ page }) => {
  const unnamed: string[] = []
  for (const id of TOOLS) {
    await boot(page, id)
    const bad = await page.$$eval('button', (els) =>
      els
        .filter((b) => {
          const label = b.getAttribute('aria-label') || b.getAttribute('title') || ''
          return !label.trim() && !(b.textContent || '').trim()
        })
        .map((b) => b.outerHTML.slice(0, 140)),
    )
    if (bad.length) unnamed.push(id + ': ' + bad.join(' | '))
  }
  expect(unnamed).toEqual([])
})
