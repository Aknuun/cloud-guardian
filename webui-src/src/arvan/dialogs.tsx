import { Badge, Button, Dialog, Empty, Input, LayerCard } from '../components/ui'

type P = {
  accForm: { name: string; token: string } | null
  setAccForm: (fn: any) => void
  accErr: string
  accBusy: boolean
  submitAcc: () => void
  accDel: { name: string } | null
  setAccDel: (v: any) => void
  removeAcc: () => void
  arvIP: { ip: string; clean: boolean; verdict: string } | null
  setArvIP: (v: any) => void
  arvPlans: any[] | null
  setArvPlans: (v: any) => void
  arvHuntOn: boolean
  setArvHuntOn: (v: boolean) => void
  arvHunt: any
  num: (n: number) => string
  toast: (m: string, err?: boolean) => void
  reload: () => void
  api: any
  acc: number | null
  sidx: number | null
  currentServer: { region?: string } | null
  pollHunt: () => Promise<void>
}

export function ArvanDialogs(p: P) {
  const {
    accForm, setAccForm, accErr, accBusy, submitAcc,
    accDel, setAccDel, removeAcc,
    arvIP, setArvIP, arvPlans, setArvPlans,
    arvHuntOn, setArvHuntOn, arvHunt, num, toast, reload, api, acc, sidx, currentServer, pollHunt,
  } = p
  return (
    <>
      <Dialog.Root open={!!accForm} onOpenChange={(o) => !o && setAccForm(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">افزودن اکانت آروان کلاد</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">
            کلید API را از پنل آروان کلاد (IAM → کلیدها) بساز و اینجا بگذار.
          </Dialog.Description>
          <div className="cc-form">
            <label className="cc-field">
              <span>نام اکانت</span>
              <Input
                value={accForm?.name || ''}
                onChange={(e) => setAccForm((f: any) => ({ name: e.target.value, token: f?.token || '' }))}
                placeholder="main"
                autoComplete="off"
              />
            </label>
            <label className="cc-field">
              <span>کلید API</span>
              <textarea
                className="cc-input cc-textarea"
                value={accForm?.token || ''}
                onChange={(e) => setAccForm((f: any) => ({ name: f?.name || '', token: e.target.value }))}
                placeholder="apikey xxxxxxxx-…"
                autoComplete="off"
              />
            </label>
            {accErr && <div className="cc-form-err">{accErr}</div>}
          </div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setAccForm(null)}>انصراف</Button>
            <Button variant="primary" disabled={accBusy} onClick={submitAcc}>ذخیره</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!accDel} onOpenChange={(o) => !o && setAccDel(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">حذف اکانت آروان کلاد</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">
            «{accDel?.name}» فقط از این پنل حذف می‌شود؛ دامنه‌ها و سرورهای آروان کلاد دست‌نخورده می‌مانند.
          </Dialog.Description>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setAccDel(null)}>انصراف</Button>
            <Button variant="primary" onClick={removeAcc}>حذف</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!arvIP} onOpenChange={(o) => !o && setArvIP(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">بررسی IP</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">{arvIP?.ip || 'در حال استعلام'}</Dialog.Description>
          <div className={"cc-hint " + (arvIP?.clean ? '' : 'cc-err')} style={{ lineHeight: 1.9 }}>{arvIP?.verdict}</div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setArvIP(null)}>بستن</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!arvPlans} onOpenChange={(o) => !o && setArvPlans(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">ارتقای پلن</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">سرور باید خاموش باشد. تغییر پلن روی دیسک اثر ندارد.</Dialog.Description>
          {(arvPlans || []).map((p) => (
            <LayerCard key={p.id} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{p.name}</div>
                <div className="cc-sub cc-ltr">{num(p.cpu)} vCPU · {num(p.ram)}GB · {num(p.disk)}GB · {p.price}</div>
              </div>
              {p.current ? <Badge>فعلی</Badge> : (
                <Button
                  variant="outline"
                  onClick={async () => {
                    try {
                      await api('/api/arv/upgrade', { i: acc, idx: sidx, size: p.id })
                      toast('درخواست ارتقا ثبت شد')
                      setArvPlans(null)
                      reload()
                    } catch (e: any) {
                      toast(String(e?.message || e), true)
                    }
                  }}
                >انتخاب</Button>
              )}
            </LayerCard>
          ))}
          {arvPlans && arvPlans.length === 0 && <Empty title="پلنی پیدا نشد" />}
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={arvHuntOn} onOpenChange={(o) => !o && setArvHuntOn(false)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">شکار پلن</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">هر دور چند سرور می‌سازد و اگر پلن موجود نبود دوباره تلاش می‌کند؛ نتیجه در تلگرام هم می‌آید.</Dialog.Description>
          <div className="cc-hint">{arvHunt?.running ? `در حال اجرا — دور ${num(arvHunt.round)}/${num(arvHunt.rounds)} · ساخته‌شده ${num(arvHunt.created)}` : 'غیرفعال'}</div>
          <div className="cc-field-row cc-mt">
            <label className="cc-field">
              <span>تعداد دور</span>
              <Input className="cc-input" type="number" defaultValue={6} id="arv-hunt-rounds" />
            </label>
            <label className="cc-field">
              <span>هر دور</span>
              <Input className="cc-input" type="number" defaultValue={1} id="arv-hunt-batch" />
            </label>
          </div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setArvHuntOn(false)}>بستن</Button>
            <Button
              variant="primary"
              onClick={async () => {
                const r = Number((document.getElementById('arv-hunt-rounds') as HTMLInputElement)?.value || 6)
                const b = Number((document.getElementById('arv-hunt-batch') as HTMLInputElement)?.value || 1)
                try {
                  await api('/api/arv/hunt/start', { i: acc, region: currentServer?.region, idx: sidx, rounds: r, batch: b })
                  await pollHunt()
                } catch (e: any) {
                  toast(String(e?.message || e), true)
                }
              }}
            >شروع</Button>
            <Button
              variant="destructive"
              onClick={async () => {
                try {
                  await api('/api/arv/hunt/stop', {})
                  await pollHunt()
                } catch (e: any) {
                  toast(String(e?.message || e), true)
                }
              }}
            >توقف</Button>
          </div>
        </Dialog>
      </Dialog.Root>
    </>
  )
}
