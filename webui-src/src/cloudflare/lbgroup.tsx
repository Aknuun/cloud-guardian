import type { Dispatch, SetStateAction } from 'react'
import { ArrowRight, ArrowsClockwise, Globe, Info, Lifebuoy, Plus, SlidersHorizontal, Trash } from '@phosphor-icons/react'
import { Button, Empty, LayerCard, Switch } from '../components/ui'
import { CopyBtn } from '../components/CopyBtn'
import { faNum } from '../lib'
import { isHost, isIPv4, isIPv6 } from './model'
import type { LbEntry } from './model'
import { SkeletonList } from './components'

type View = { v: string; acc?: number; zid?: string; name?: string }

export function LbGroupPage({ view, entries, setEntries, addLb, dropLb, patchLb, setDlg, post, push, copy, num, busy, err }: {
  view: View
  entries: LbEntry[]
  setEntries: Dispatch<SetStateAction<LbEntry[]>>
  addLb: (e: LbEntry) => void
  dropLb: (content: string) => void
  patchLb: (content: string, patch: Partial<LbEntry>) => void
  setDlg: (d: any) => void
  post: (path: string, body: any, keep?: boolean, opts?: { refresh?: boolean }) => Promise<any>
  push: (v: any) => void
  busy: number
  err: string
  copy: (v: string, label?: string) => void
  num: (n: number | undefined) => string
}) {
  const acc = view.acc
  const zid = view.zid
  const vname = view.name

    function entryAction(op: string, extra: any = {}, apply?: () => void, undo?: () => void) {
      setDlg(null)
      apply?.()
      void post('/api/cf/lb', { op, acc: acc!, zid: zid!, name: vname!, ...extra }, true, {
        refresh: false,
      }).then((d) => {
        if (!d) undo?.()
      })
    }
    function confirmProxy(on: boolean) {
      setDlg({
        k: 'confirm',
        title: on ? 'روشن‌کردن پروکسی همه' : 'خاموش‌کردن پروکسی همه',
        desc: `پروکسی برای هر ${faNum(entries.length)} آی‌پی این گروه ${on ? 'روشن' : 'خاموش'} شود؟`,
        danger: !on,
        okText: on ? 'روشن کن' : 'خاموش کن',
        onOk: () => entryAction('proxy', { on }),
      })
    }
    return (
      <>
        <div className="cc-hintbox">
          <Info size={17} />
          <span>
            در این گروه، زاپاس فقط وقتی آی‌پی اصلی از کار بیفتد استفاده می‌شود؛ «جایگزینی» زاپاس را به‌جای آی‌پی اصلی
            می‌نشاند.
          </span>
        </div>
        <div className="cc-lb-top">
          <Button
            variant="primary"
            icon={Plus}
            onClick={() =>
              setDlg({
                k: 'form',
                title: 'افزودن آی‌پی',
                fields: [
                  {
                    key: 'type',
                    label: 'نوع',
                    select: ['A', 'AAAA', 'CNAME'].map((t) => ({ value: t, label: t })),
                  },
                  {
                    key: 'content',
                    label: 'مقدار',
                    placeholder: '1.2.3.4 یا host.example.com',
                    validate: (v: string) =>
                      isIPv4(v.trim()) || isIPv6(v.trim()) || isHost(v.trim())
                        ? undefined
                        : 'آی‌پی یا میزبان معتبر نیست.',
                  },
                ],
                okText: 'افزودن',
                onSubmit: async (v: any) => {
                              const temp: LbEntry = {
                    on: true,
                    id: '',
                    type: v.type,
                    content: v.content,
                    proxied: false,
                    ttl: 1,
                    weight: 1,
                  }
                  setDlg(null)
                  addLb(temp)
                  const d = await post(
                    '/api/cf/lb',
                    {
                      op: 'add',
                      acc: acc!,
                      zid: zid!,
                      name: vname!,
                      type: v.type,
                      content: v.content,
                    },
                    true,
                    { refresh: false },
                  )
                  if (!d) setEntries((list) => list.filter((e) => e !== temp))
                },
              })
            }
          >
            افزودن آی‌پی
          </Button>
          <Button variant="secondary" icon={Globe} onClick={() => confirmProxy(true)}>
            پروکسی همه
          </Button>
        </div>
        <div className="cc-lb-links">
          <button type="button" className="cc-link" onClick={() => confirmProxy(false)}>
            خاموش‌کردن پروکسی همه
          </button>
          <span aria-hidden="true">·</span>
          <button
            type="button"
            className="cc-link"
            onClick={() => push({ v: 'lb', acc: acc!, zid: zid! })}
          >
            <ArrowRight size={13} />
            بازگشت به گروه‌ها
          </button>
        </div>
        <div className="cc-section">{faNum(entries.length)} آی‌پی</div>
        {entries.length === 0 && busy > 0 && <SkeletonList />}
        {entries.length === 0 && !busy && !err && <Empty title="آی‌پی‌ای نیست" />}
        {entries.map((e) => (
          <LayerCard key={e.content + e.type} className="cc-card cc-lb-card">
            <div className="cc-row">
              <span className={'cc-dot ' + (e.on ? 'run' : 'off')} />
              <div className="cc-grow">
                <div className="cc-name cc-mono cc-copyline">
                  <span>{e.content}</span>
                  <CopyBtn value={e.content} onCopy={copy} label="کپی آی‌پی" />
                </div>
                <div className="cc-sub cc-copyline">
                  <span>{e.type} · وزن {faNum(e.weight)}</span>
                  {e.spare ? (
                    <>
                      <span>· زاپاس:</span>
                      <span className="cc-mono cc-copyval">{e.spare.content}</span>
                      <CopyBtn value={e.spare.content} onCopy={copy} label="کپی آی‌پی زاپاس" size={14} />
                    </>
                  ) : (
                    <span>· بدون زاپاس</span>
                  )}
                </div>
              </div>
              <Switch
                checked={e.on}
                aria-label={'فعال بودن ' + e.content}
                onCheckedChange={() => {
                  const next = !e.on
                  entryAction(
                    'toggle',
                    { content: e.content, on: next },
                    () => patchLb(e.content, { on: next }),
                    () => patchLb(e.content, { on: e.on }),
                  )
                }}
              />
            </div>
            <div className="cc-lb-actions">
              <button
                type="button"
                className="cc-lb-act"
                onClick={() =>
                  setDlg({
                    k: 'form',
                    title: 'وزن آی‌پی',
                    fields: [
                      {
                        key: 'weight',
                        label: 'وزن (۰ یا بیشتر)',
                        value: String(e.weight),
                        validate: (v: string) => {
                          const n = Number(v)
                          return Number.isFinite(n) && n >= 0 ? undefined : 'وزن باید عددی صفر یا بیشتر باشد.'
                        },
                      },
                    ],
                    okText: 'ذخیره',
                    onSubmit: async (v: any) => {
                                      const prev = e.weight
                      const next = Number(v.weight || 0)
                      patchLb(e.content, { weight: next })
                      setDlg(null)
                      const d = await post(
                        '/api/cf/lb',
                        {
                          op: 'weight',
                          acc: acc!,
                          zid: zid!,
                          name: vname!,
                          content: e.content,
                          weight: next,
                        },
                        true,
                        { refresh: false },
                      )
                      if (!d) patchLb(e.content, { weight: prev })
                    },
                  })
                }
              >
                <SlidersHorizontal size={16} />
                <span>وزن</span>
                <b>{faNum(e.weight)}</b>
              </button>
              <button
                type="button"
                className="cc-lb-act"
                onClick={() =>
                  setDlg({ k: 'spare', acc: acc!, zid: zid!, name: vname!, entry: e })
                }
              >
                <Lifebuoy size={16} />
                <span>زاپاس</span>
                <b className={e.spare ? 'cc-lb-ok' : 'cc-lb-muted'}>{e.spare ? 'فعال' : 'ندارد'}</b>
              </button>
              {e.spare && (
                <button
                  type="button"
                  className="cc-lb-act"
                  onClick={() =>
                    setDlg({
                      k: 'confirm',
                      title: 'جایگزینی با زاپاس',
                      desc: `آی‌پی زاپاس ${e.spare?.content} جای ${e.content} بنشیند؟`,
                      onOk: () => {
                        const sp = e.spare
                        if (!sp) return
                        const nextType = sp.type || e.type
                        entryAction(
                          'replace',
                          { content: e.content },
                          () => patchLb(e.content, { content: sp.content, type: nextType, spare: undefined }),
                          () => patchLb(sp.content, { content: e.content, type: e.type, spare: e.spare }),
                        )
                      },
                    })
                  }
                >
                  <ArrowsClockwise size={16} />
                  <span>جایگزینی</span>
                </button>
              )}
              <button
                type="button"
                className="cc-lb-del"
                aria-label={'حذف آی‌پی ' + e.content}
                onClick={() =>
                  setDlg({
                    k: 'confirm',
                    title: 'حذف آی‌پی از گروه',
                    desc: `${e.type} · ${e.content} و تنظیمات وزن/زاپاس آن حذف شود؟ این کار رکورد DNS را هم پاک می‌کند.`,
                    danger: true,
                    okText: 'حذف',
                    onOk: () =>
                      entryAction(
                        'del',
                        { content: e.content },
                        () => dropLb(e.content),
                        () => addLb(e),
                      ),
                  })
                }
              >
                <Trash size={16} />
              </button>
            </div>
          </LayerCard>
        ))}
      </>
    )
  
}
