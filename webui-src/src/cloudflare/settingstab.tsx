import { Broom, CaretDown, GearSix, Info, Pause, Play, SlidersHorizontal } from '@phosphor-icons/react'
import { Button, Empty, LayerCard, Switch } from '../components/ui'
import { SkeletonList } from './components'
import type { SetGroup } from './model'

export function SettingsTab({ view, settings, setSetting, applySetting, setDlg, post, openDetails }: {
  view: { v: string; acc?: number; zid?: string }
  settings: { groups: SetGroup[]; dev: boolean; paused: boolean } | null
  setSetting: (it: any, value: string) => void
  applySetting: (...args: any[]) => void
  setDlg: (d: any) => void
  post: (path: string, body: any, keep?: boolean, opts?: { refresh?: boolean }) => Promise<any>
  openDetails: () => void
}) {

    if (!view.zid!) return null
    if (!settings) return <SkeletonList rows={2} />
    return (
      <>
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>حالت توسعه</span>
            <Switch
              checked={settings.dev}
              aria-label="حالت توسعه"
              onCheckedChange={() =>
                setDlg({
                  k: 'confirm',
                  title: settings.dev ? 'خاموش کردن حالت توسعه' : 'روشن کردن حالت توسعه',
                  desc: 'حالت توسعه کش را دور می‌زند و برای تست مناسب است.',
                  onOk: async () => {
                    if (view.v === 'zone')
                      await post('/api/cf/settings', { op: 'dev', acc: view.acc!, zid: view.zid! })
                  },
                })
              }
            />
          </div>
          <div className="cc-kv">
            <span>کش دامنه</span>
            <Button
              size="sm"
              variant="secondary-destructive"
              icon={Broom}
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'پاک‌سازی کش',
                  desc: 'کل کش دامنه پاک شود؟',
                  onOk: async () => {
                    if (view.v === 'zone')
                      await post('/api/cf/settings', { op: 'purge', acc: view.acc!, zid: view.zid! })
                  },
                })
              }
            >
              پاک‌سازی
            </Button>
          </div>
        </LayerCard>
        {settings.groups.map((g) => (
          <div key={g.key}>
            <div className="cc-section">{g.title}</div>
            <LayerCard className="cc-card">
              {g.items.map((it) => (
                <div className="cc-kv" key={it.key}>
                  <span>{it.label}</span>
                  {it.kind === 'bool' ? (
                    <Switch
                      checked={it.value === 'on'}
                      aria-label={it.label}
                      onCheckedChange={(on) => applySetting(it, on ? 'on' : 'off')}
                    />
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      icon={CaretDown}
                      onClick={() =>
                        setDlg({
                          k: 'pick',
                          title: it.label,
                          items: (it.values || []).map((v) => ({
                            id: v.v,
                            title: v.l,
                            sub: v.v === it.value ? 'فعلی' : '',
                          })),
                          onPick: (v: any) => {
                            setDlg(null)
                            applySetting(it, v)
                          },
                        })
                      }
                    >
                      {it.values?.find((v) => v.v === it.value)?.l || it.value}
                    </Button>
                  )}
                </div>
              ))}
            </LayerCard>
          </div>
        ))}
        <div className="cc-grid2 cc-mt">
          <Button variant="outline" icon={Info} onClick={openDetails}>
            جزئیات دامنه
          </Button>
          <Button
            variant={settings.paused ? 'primary' : 'secondary-destructive'}
            icon={settings.paused ? Play : Pause}
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: settings.paused ? 'فعال‌سازی دامنه' : 'توقف دامنه',
                desc: settings.paused
                  ? 'دامنه دوباره روی کلادفلر فعال شود؟'
                  : 'دامنه متوقف شود؟ سایت از دسترس خارج می‌شود.',
                danger: !settings.paused,
                onOk: async () => {
                  if (view.v === 'zone')
                    await post('/api/cf/settings', { op: 'pause', acc: view.acc!, zid: view.zid! })
                },
              })
            }
          >
            {settings.paused ? 'فعال‌سازی دامنه' : 'توقف دامنه'}
          </Button>
        </div>
      </>
    )
  
}
