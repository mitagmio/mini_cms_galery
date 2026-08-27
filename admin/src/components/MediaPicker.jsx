import { useEffect, useMemo, useState } from 'react'
import { apiUrl, admin } from '../api'
import { mediaThumbUrl } from '../blockTypes'
import { useToast } from '../toast'

/** Session cache: reuse last list until kind/q change or invalidateMediaListCache(). */
let listCache = { key: null, items: null }

function cacheKey(kind, q) {
  return `${kind || ''}\0${q || ''}`
}

export function invalidateMediaListCache() {
  listCache = { key: null, items: null }
}

export default function MediaPicker({
  open,
  onClose,
  onSelect,
  multi = false,
  title = 'Pick media',
  kindFilter = '',
}) {
  const toast = useToast()
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [kind, setKind] = useState(kindFilter || '')
  const [q, setQ] = useState('')
  const [debouncedQ, setDebouncedQ] = useState('')
  const [selected, setSelected] = useState([])

  useEffect(() => {
    if (!open) return
    setSelected([])
    setKind(kindFilter || '')
  }, [open, kindFilter])

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQ(q), 300)
    return () => clearTimeout(t)
  }, [q])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    const key = cacheKey(kind, debouncedQ)
    if (listCache.key === key && Array.isArray(listCache.items)) {
      setItems(listCache.items)
      setLoading(false)
      return
    }
    ;(async () => {
      setLoading(true)
      try {
        const data = await admin.media.list({
          kind: kind || undefined,
          q: debouncedQ || undefined,
        })
        const next = data.media || data.items || []
        if (!cancelled) {
          listCache = { key, items: next }
          setItems(next)
        }
      } catch (e) {
        if (!cancelled) {
          setItems([])
          toast.error(e.message)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [open, kind, debouncedQ, toast])

  const filtered = useMemo(() => items, [items])

  if (!open) return null

  function toggle(id) {
    if (!multi) {
      setSelected([id])
      return
    }
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  function confirm() {
    const picks = filtered.filter((m) => selected.includes(m.id))
    if (!picks.length) return
    onSelect(multi ? picks : picks[0])
    onClose()
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="modal">
        <div className="modal-head">
          <h2>{title}</h2>
          <button type="button" className="secondary" onClick={onClose}>
            Close
          </button>
        </div>
        <div className="bar">
          <label>
            Kind
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="">all</option>
              <option value="before">before</option>
              <option value="after">after</option>
              <option value="slide">slide</option>
              <option value="portfolio">portfolio</option>
              <option value="favicon">favicon</option>
            </select>
          </label>
          <label>
            Search
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="title or id" />
          </label>
        </div>
        {loading && <p className="muted">Loading…</p>}
        <div className="grid picker-grid">
          {filtered.map((m) => {
            const src = apiUrl(mediaThumbUrl(m))
            const on = selected.includes(m.id)
            return (
              <button
                type="button"
                key={m.id}
                className={`card pick-card ${on ? 'selected' : ''}`}
                onClick={() => toggle(m.id)}
              >
                {src ? (
                  <img src={src} alt={m.title || m.id} loading="lazy" />
                ) : (
                  <div className="thumb-empty">No preview</div>
                )}
                <div className="meta">
                  {m.kind || '—'} · {m.title || m.id}
                </div>
              </button>
            )
          })}
        </div>
        {!loading && !filtered.length && <p className="muted">No media. Upload in Media library.</p>}
        <div className="modal-foot">
          <span className="muted">{selected.length} selected</span>
          <button type="button" disabled={!selected.length} onClick={confirm}>
            Use selection
          </button>
        </div>
      </div>
    </div>
  )
}
