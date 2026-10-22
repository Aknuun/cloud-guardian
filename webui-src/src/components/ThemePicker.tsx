import { useState } from 'react'
import { Palette as PaletteIcon, X } from '@phosphor-icons/react'
import { haptic } from '../lib'
import { Button, Dialog } from './ui'
import { PALETTES, useSectionTheme, type Provider } from './sectionTheme'

const LABELS: Record<Provider, string> = {
  hz: 'هتزنر',
  cf: 'کلادفلر',
  ln: 'لینود',
  arv: 'آروان',
}

export function ThemePicker({ provider }: { provider: Provider }) {
  const [open, setOpen] = useState(false)
  const { palette, style, select, reset } = useSectionTheme(provider)

  return (
    <>
      <Button
        variant="ghost"
        shape="square"
        icon={PaletteIcon}
        aria-label="تم این بخش"
        title="تم این بخش"
        onClick={() => setOpen(true)}
      />
      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog className="cc-dialog cc-theme-dialog" size="lg" style={style}>
          <div className="cc-dialog-head">
            <div className="cc-dialog-headtext">
              <Dialog.Title className="cc-dialog-title">تم {LABELS[provider]}</Dialog.Title>
              <Dialog.Description className="cc-dialog-sub">
                یک پالت رنگی برای این بخش انتخاب کن؛ انتخاب هر بخش جدا ذخیره می‌شود.
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <Button variant="ghost" shape="square" icon={X} aria-label="بستن" />
            </Dialog.Close>
          </div>
          <div className="cc-theme-grid">
            {PALETTES.map((item) => {
              const active = item.id === palette.id
              return (
                <button
                  key={item.id}
                  type="button"
                  className={'cc-theme-opt' + (active ? ' on' : '')}
                  aria-pressed={active}
                  onClick={() => {
                    select(item.id)
                    haptic('light')
                    setOpen(false)
                  }}
                >
                  <span
                    className="cc-theme-dot"
                    style={{ backgroundImage: `linear-gradient(135deg, ${item.light[0]}, ${item.light[1]})` }}
                  />
                  <span>{item.label}</span>
                </button>
              )
            })}
          </div>
          <div className="cc-dialog-actions">
            <Button
              variant="secondary"
              onClick={() => {
                reset()
                haptic('light')
              }}
            >
              پیش‌فرض {LABELS[provider]}
            </Button>
            <Dialog.Close asChild>
              <Button variant="primary">بستن</Button>
            </Dialog.Close>
          </div>
        </Dialog>
      </Dialog.Root>
    </>
  )
}
