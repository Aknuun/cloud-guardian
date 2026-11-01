import type { Dispatch, SetStateAction } from 'react'
import { ArrowsClockwise, CheckSquare, Clock, PencilSimple, Plus, Star, Trash, X, } from '@phosphor-icons/react'
import { Button, Empty } from '../components/ui'
import { SearchInput } from './components'
import { SkeletonList } from './components'
import { splitFavorites } from './model'
import type { CFRec } from './model'

export function RecordsTab(p: {
  view: { v: string; acc?: number; zid?: string }
  zone?: { name?: string }
  records: CFRec[]
  busy: number
  err: string
  recQ: string
  setRecQ: (v: string) => void
  recType: string
  setRecType: (v: string) => void
  sel: Set<string>
  setSel: Dispatch<SetStateAction<Set<string>>>
  selMode: boolean
  setSelMode: (v: boolean) => void
  setDlg: (d: any) => void
  showAudit: (acc: number, zid: string) => void
  confirmBulkDelete: () => void
  bulkEdit: () => void
  bulkType: () => void
  bulk: (op: string) => void
  recordRow: (r: CFRec) => any
  faNum: (n: number) => string
  num: (n: number | undefined) => string
}) {

    if (!p.view.zid) return null
    const ql = p.recQ.trim().toLowerCase()
    const list = p.records.filter(
      (r) =>
        (p.recType === 'all' || r.type === p.recType) &&
        (!ql ||
          r.name.toLowerCase().includes(ql) ||
          r.content.toLowerCase().includes(ql) ||
          r.type.toLowerCase() === ql),
    )
    const { favs: favList, rest } = splitFavorites(list)
    const types = Array.from(new Set(p.records.map((r) => r.type)))
    function selectAll() {
      if (p.sel.size === list.length) p.setSel(new Set())
      else p.setSel(new Set(list.map((r) => r.id)))
    }
    return (
      <>
        <div className="cc-grid3 cc-mb">
          <Button
            variant="primary"
            icon={Plus}
            onClick={() => p.setDlg({ k: 'editor', acc: p.view.acc, zid: p.view.zid, zone: p.zone?.name || '' })}
          >
            رکورد
          </Button>
          <Button
            variant={p.selMode ? 'secondary' : 'outline'}
            icon={p.selMode ? X : CheckSquare}
            onClick={() => {
              p.setSelMode(!p.selMode)
              p.setSel(new Set())
            }}
          >
            {p.selMode ? 'پایان' : 'انتخاب گروهی'}
          </Button>
          <Button variant="outline" icon={Clock} onClick={() => void p.showAudit(p.view.acc!, p.view.zid!)}>
            تاریخچه
          </Button>
        </div>
        {p.selMode && (
          <div className="cc-bulkbar" role="toolbar" aria-label="عملیات گروهی">
            <span className="cc-bulkcount">{p.faNum(p.sel.size)} انتخاب</span>
            <button className="cc-bulkbtn" onClick={selectAll} disabled={list.length === 0}>
              {p.sel.size === list.length && list.length > 0 ? <X size={14} /> : <CheckSquare size={14} />}
              {p.sel.size === list.length && list.length > 0 ? 'پاک‌کردن' : 'همه'}
            </button>
            <button className="cc-bulkbtn red" disabled={!p.sel.size} onClick={p.confirmBulkDelete}>
              <Trash size={14} /> حذف
            </button>
            <button className="cc-bulkbtn" disabled={!p.sel.size} onClick={p.bulkEdit}>
              <PencilSimple size={14} /> مقدار
            </button>
            <button className="cc-bulkbtn" disabled={!p.sel.size} onClick={p.bulkType}>
              <ArrowsClockwise size={14} /> نوع
            </button>
            <button className="cc-bulkbtn green" disabled={!p.sel.size} onClick={() => p.bulk('fav')}>
              <Star size={14} /> منتخب
            </button>
          </div>
        )}
        <SearchInput placeholder="جستجوی رکوردها" value={p.recQ} onChange={p.setRecQ} />
        {types.length > 1 && (
          <div className="cc-chips" role="group" aria-label="فیلتر نوع رکورد">
            {['all', ...types].map((t) => (
              <button
                key={t}
                className={'cc-chip' + (p.recType === t ? ' on' : '')}
                aria-pressed={p.recType === t}
                onClick={() => p.setRecType(t)}
              >
                {t === 'all' ? 'همه' : t}
              </button>
            ))}
          </div>
        )}
        <div className="cc-section">
          {p.num(list.length)} رکورد{p.sel.size > 0 ? ' · ' + p.faNum(p.sel.size) + ' انتخاب' : ''}
        </div>
        {p.records.length === 0 && p.busy > 0 && <SkeletonList />}
        {list.length === 0 && !p.busy && !p.err && <Empty title="رکوردی نیست" />}
        {favList.length > 0 && <div className="cc-section cc-section-fav">ستاره‌دار</div>}
        {favList.map(p.recordRow)}
        {favList.length > 0 && rest.length > 0 && <div className="cc-section">بقیه رکوردها</div>}
        {rest.map(p.recordRow)}
      </>
    )
  
}
