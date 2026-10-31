import { Badge, Button, Empty, LayerCard } from '../components/ui'
import { CaretLeft, Trash } from '@phosphor-icons/react'
import type { Dispatch, SetStateAction } from 'react'
import { CopyBtn } from '../components/CopyBtn'
import type { CFAcc, Dlg, FavItem } from './model'
import { FavStar, SkeletonList } from './components'

type Post = (path: string, body: any) => Promise<any>

export function AccountsPage({ accs, busy, err, num, post, setDlg }: {
  accs: CFAcc[]
  busy: number
  err: string
  num: (n: number | undefined) => string
  post: Post
  setDlg: Dispatch<SetStateAction<Dlg | null>>
}) {

    return (
      <>
        {accs.length === 0 && !busy && !err && (
          <Empty title="اکانتی ثبت نشده" description="توکن کلادفلر را اضافه کن." />
        )}
        {accs.length === 0 && busy > 0 && <SkeletonList />}
        {accs.map((a) => (
          <LayerCard key={a.i} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{a.name}</div>
              <div className="cc-sub">{(a.zones ?? 0) >= 0 ? num(a.zones) + ' دامنه' : 'خطا در خواندن دامنه‌ها'}</div>
            </div>
            <Button
              variant="secondary-destructive"
              size="sm"
              icon={Trash}
              aria-label="حذف اکانت"
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'حذف اکانت',
                  desc: `اکانت «${a.name}» حذف شود؟ دامنه‌ها حذف نمی‌شوند ولی از پنل خارج می‌شوند.`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    await post('/api/cf/accounts', { op: 'del', i: a.i })
                  },
                })
              }
            >
              حذف
            </Button>
          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-mt"
          variant="primary"
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'افزودن اکانت کلادفلر',
              desc: 'توکن باید دسترسی Zone → DNS → Edit و Email Routing داشته باشد.',
              fields: [
                {
                  key: 'name',
                  label: 'نام اکانت',
                  placeholder: 'مثلاً main',
                  validate: (v) => (v.trim() ? undefined : 'نام اکانت را وارد کن.'),
                },
                {
                  key: 'token',
                  label: 'API Token',
                  placeholder: 'توکن کلادفلر',
                  password: true,
                  validate: (v) => (v.trim().length >= 20 ? undefined : 'توکن معتبر نیست.'),
                },
              ],
              okText: 'افزودن',
              onSubmit: async (v) => {
                await post('/api/cf/accounts', { op: 'add', name: v.name, token: v.token })
              },
            })
          }
        >
          افزودن اکانت
        </Button>
      </>
    )
  
}

export function FavsPage({ favs, busy, err, openInZone, copy, post, setFav, applyFav }: {
  favs: FavItem[]
  busy: number
  err: string
  openInZone: (acc: number, zid: string, zone: string, rid: string) => void
  copy: (v: string, label?: string) => void
  post: Post
  setFav: (acc: number, zid: string, rid: string, next: boolean) => Promise<void>
  applyFav: (rid: string, fav: boolean) => void
}) {

    return (
      <>
        <div className="cc-section">رکوردهای ستاره‌دار</div>
        {favs.length === 0 && !busy && !err && (
          <Empty title="منتخب‌ها خالی است" description="از فهرست رکوردها یا داخل رکورد، ستاره را بزن." />
        )}
        {favs.length === 0 && busy > 0 && <SkeletonList />}
        {favs.map((f) => (
          <LayerCard
            key={f.rid}
            className={'cc-card cc-row' + (f.missing ? '' : ' cc-tap')}
            onClick={() => {
              if (!f.missing) openInZone(f.acc, f.zid, f.zone, f.rid)
            }}
          >
            <Badge variant="secondary">{f.type || '—'}</Badge>
            <div className="cc-grow">
              <div className="cc-name cc-copyline">
                <span>{f.label || f.rid}</span>
                {!f.missing && (
                  <CopyBtn value={f.name || f.label} onCopy={copy} label="کپی زیردامنه" />
                )}
                {f.missing && <Badge variant="error">حذف‌شده</Badge>}
              </div>
              <div className="cc-sub cc-copyline">
                <span>{f.zone || ''}</span>
                {f.content && (
                  <>
                    <span>·</span>
                    <span className="cc-mono cc-copyval">{f.content}</span>
                    <CopyBtn value={f.content} onCopy={copy} label="کپی مقدار" size={14} />
                  </>
                )}
              </div>
            </div>
            {f.missing ? (
              <Button
                variant="secondary-destructive"
                size="sm"
                icon={Trash}
                onClick={(e) => {
                  e.stopPropagation()
                  applyFav(f.rid, false)
                  void post('/api/cf/favs', { rid: f.rid })
                }}
              >
                پاک کردن
              </Button>
            ) : (
              <>
                <FavStar fav onClick={() => void setFav(f.acc, f.zid, f.rid, false)} label="حذف از منتخب" />
                <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
              </>
            )}
          </LayerCard>
        ))}
      </>
    )
  
}
