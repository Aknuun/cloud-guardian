import { Badge, Button, Dialog, Empty, Input, LayerCard } from './ui'
import { CaretLeft, GlobeHemisphereWest, Plus, Trash } from '@phosphor-icons/react'
import { api, invalidateCache, num } from '../lib'
import { CopyBtn } from './CopyBtn'
import { FilterBar } from './FilterBar'
import { FormDialog } from './toolsMeta'
import { AppSkeleton } from './AppDialogs'
import type { Account, Server } from './hetzner/types'
import { badgeVariant, locFa, STATUS } from './hetzner/ui'
import { usePageActions } from './PageAction'

export function AccountsView(p: {
  accounts: Account[]
  q: string
  setQ: (v: string) => void
  aflt: string
  setAflt: (v: string) => void
  reload: () => void
  busy: number
  accForm: { name: string; token: string } | null
  setAccForm: (v: { name: string; token: string } | null) => void
  accErr: string
  setAccErr: (v: string) => void
  accDel: Account | null
  setAccDel: (v: Account | null) => void
  push: (v: any) => void
  toast: (t: string, err?: boolean) => void
  setServers: (v: Server[]) => void
  setServersAcc: (v: number | null) => void
}) {
  const { accounts, q, setQ, aflt, setAflt, reload, busy, accForm, setAccForm, accErr, setAccErr, accDel, setAccDel, push, toast, setServers, setServersAcc } = p
  usePageActions([
    { label: 'افزودن اکانت', icon: Plus, onClick: () => { setAccErr(''); setAccForm({ name: '', token: '' }) } },
  ])

      const aq = q.trim().toLowerCase()
      const list0 = accounts.filter(
        (a) => (!aq || a.name.toLowerCase().includes(aq)) && (aflt === 'has' ? (a.count || 0) > 0 : aflt === 'none' ? (a.count || 0) === 0 : true),
      )
      const _known = accounts.some((a) => a.count !== undefined)
      const _totalServers = accounts.reduce((n, a) => n + (a.count || 0), 0)
      return (
        <>
          <FilterBar
            q={q}
            onQ={setQ}
            placeholder="جستجوی اکانت"
            filters={[
              { id: 'all', label: 'همه اکانت‌ها' },
              { id: 'has', label: 'دارای سرور' },
              { id: 'none', label: 'بدون سرور' },
            ]}
            value={aflt}
            onValue={setAflt}
            onRefresh={reload}
          />
          {!list0.length && busy > 0 && <AppSkeleton />}
          {!list0.length && busy === 0 && (
            <Empty title="اکانتی پیدا نشد" description={aq ? 'چیزی با این جستجو پیدا نشد.' : 'با دکمهٔ بالا اولین اکانت هتزنر را اضافه کن.'} />
          )}
          {list0.map((a) => (
            <LayerCard
              key={a.i}
              className="cc-trow cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => {
                setServers([])
                setServersAcc(null)
                push({ v: 'servers', acc: a.i })
              }}
            >
              <span className="cc-avatar">{(a.name || 'C').trim().charAt(0)}</span>
              <div className="cc-grow">
                <div className="cc-name">{a.name || 'اکانت ' + (a.i + 1)}</div>
                <div className="cc-sub">{a.count === undefined ? 'برای دیدن سرورها بزن' : num(a.count) + ' سرور'}</div>
              </div>
              <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
              <button
                type="button"
                className="cc-rowbtn danger"
                aria-label={'حذف ' + (a.name || 'اکانت')}
                onClick={(e) => {
                  e.stopPropagation()
                  setAccDel(a)
                }}
              >
                <Trash size={16} />
              </button>
            </LayerCard>
          ))}
          <div className="cc-hint cc-center cc-mt">هر اکانت را انتخاب کن تا سرورها و آی‌پی‌ها را ببینی.</div>
          {accForm && (
            <FormDialog
              title="افزودن اکانت هتزنر"
              sub="توکن API را از کنسول هتزنر بساز (Read & Write)."
              error={accErr}
              onClose={() => setAccForm(null)}
              onSave={async () => {
                setAccErr('')
                try {
                  await api('/api/hz/accounts/add', { name: accForm.name.trim(), token: accForm.token.trim() })
                  invalidateCache()
                  setAccForm(null)
                  reload()
                } catch (e: any) {
                  setAccErr(e.message || 'ذخیره ناموفق')
                }
              }}
            >
              <label className="cc-field">
                <span>نام اکانت</span>
                <Input
                  value={accForm.name}
                  placeholder="مثلاً main"
                  onChange={(e: any) => setAccForm({ ...accForm, name: e.target.value })}
                />
              </label>
              <label className="cc-field">
                <span>توکن API</span>
                <Input
                  value={accForm.token}
                  placeholder="مثل 3f2a…"
                  autoComplete="off"
                  onChange={(e: any) => setAccForm({ ...accForm, token: e.target.value })}
                />
              </label>
            </FormDialog>
          )}
          {accDel && (
            <Dialog.Root open onOpenChange={(o: boolean) => !o && setAccDel(null)}>
              <Dialog className="cc-dialog" size="base">
                <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
                <Dialog.Title className="cc-dialog-title">حذف اکانت</Dialog.Title>
                <Dialog.Description className="cc-dialog-desc">
                  «{accDel.name}» از لیست حذف می‌شود. سرورهای هتزنر دست‌نخورده می‌مانند.
                </Dialog.Description>
                <div className="cc-dialog-actions">
                  <Button variant="secondary" onClick={() => setAccDel(null)}>
                    انصراف
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={async () => {
                      try {
                        await api('/api/hz/accounts/del', { name: accDel.name })
                        invalidateCache()
                        setAccDel(null)
                        reload()
                      } catch (e: any) {
                        toast(e.message || 'حذف ناموفق', true)
                      }
                    }}
                  >
                    حذف
                  </Button>
                </div>
              </Dialog>
            </Dialog.Root>
          )}
        </>
      )
}

export function ServersView(p: {
  cur: { acc?: number }
  servers: Server[]
  q: string
  setQ: (v: string) => void
  sflt: string
  setSflt: (v: string) => void
  reload: () => void
  busy: number
  push: (v: any) => void
  copy: (v: string) => void
  setCreateName: (v: string) => void
}) {
  const { cur, servers, q, setQ, sflt, setSflt, reload, busy, push, copy, setCreateName } = p
  usePageActions([
    {
      label: 'ساخت سرور',
      icon: Plus,
      onClick: () => {
        setCreateName('')
        push({ v: 'create', acc: cur.acc, step: 1, loc: '', type: '', typeName: '', arch: '', image: '', imageName: '' })
      },
    },
    { label: 'آی‌پی‌های تمیز', icon: GlobeHemisphereWest, onClick: () => push({ v: 'ips', acc: cur.acc }) },
  ])

      const ql = q.trim().toLowerCase()
      const list = servers
        .filter((s) => (sflt === 'run' ? s.status === 'running' : sflt === 'off' ? s.status !== 'running' : true))
        .filter((s) => !ql || (s.name + ' ' + s.ipv4 + ' ' + s.ipv6 + ' ' + (s.type?.name)).toLowerCase().includes(ql))
      const _running = list.filter((s) => s.status === 'running').length
      return (
        <>
          <FilterBar
            q={q}
            onQ={setQ}
            placeholder="جستجوی نام یا آی‌پی"
            filters={[
              { id: 'all', label: 'همه سرورها' },
              { id: 'run', label: 'روشن' },
              { id: 'off', label: 'خاموش' },
            ]}
            value={sflt}
            onValue={setSflt}
            onRefresh={reload}
          />
          <div className="cc-section">{num(list.length)} سرور</div>
          {servers.length === 0 && busy > 0 && <AppSkeleton />}
          {list.length === 0 && busy === 0 && <Empty title="سروری پیدا نشد" />}
          {list.map((s) => (
            <LayerCard
              key={s.id}
              className="cc-trow cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => push({ v: 'server', acc: cur.acc, id: s.id })}
            >
              <div className="cc-grow">
                <div className="cc-name">
                  {s.name}
                  {s.ipv4 && (
                    <span className="cc-ipinl cc-mono">
                      <span className="cc-copyval">{s.ipv4}</span>
                      <CopyBtn value={s.ipv4} onCopy={copy} label="کپی IPv4" size={13} />
                    </span>
                  )}
                </div>
                <div className="cc-sub">
                  {(s.type?.name) || ''}
                  {s.type ? ` · ${s.type.cores}vCPU / ${s.type.memory} GB / ${s.type.disk} GB` : ''}
                  {s.location ? ` · ${locFa(s.location.name, s.location.city, s.location.country)}` : ''}
                </div>
                {s.ipv6 && (
                  <div className="cc-sub cc-mono cc-copyline">
                    <span className="cc-copyval">{s.ipv6}</span>
                    <CopyBtn value={s.ipv6} onCopy={copy} label="کپی IPv6" size={13} />
                  </div>
                )}
              </div>
              <div className="cc-end">
                <Badge variant={badgeVariant(s.status)} dot>
                  {STATUS[s.status] || s.status}
                </Badge>
              </div>
            </LayerCard>
          ))}
        </>
      )
}
