import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { admin } from '../api'
import { useToast } from '../toast'

function newId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `nav-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}

function pageHref(page) {
  if (!page) return ''
  const slug = String(page.slug || '')
    .replace(/^\/+/, '')
    .replace(/\/+$/, '')
  if (page.is_homepage && !slug) return '/'
  return slug ? `/${slug}` : '/'
}

function pageLabel(page) {
  if (!page) return ''
  const name = page.nav_label || page.title || page.slug || page.id
  const path = pageHref(page)
  return `${name} (${path})`
}

function blankLink() {
  return {
    id: newId(),
    label: '',
    href: '',
    page_id: '',
    kind: 'link',
    visible: true,
    children: [],
  }
}

function blankDropdown() {
  return {
    id: newId(),
    label: '',
    href: '',
    page_id: '',
    kind: 'category',
    visible: true,
    children: [],
  }
}

function normalizeItem(raw) {
  const children = Array.isArray(raw.children) ? raw.children.map(normalizeItem) : []
  const kind = raw.kind === 'category' || children.length > 0 ? 'category' : 'link'
  return {
    id: raw.id || newId(),
    label: raw.label || raw.title || '',
    href: raw.href || raw.external_url || '',
    page_id: raw.page_id || '',
    kind,
    visible: raw.visible !== false,
    children: kind === 'category' ? children.map((c) => ({ ...c, kind: 'link', children: [] })) : [],
  }
}

function moveItem(list, from, to) {
  if (to < 0 || to >= list.length) return list
  const next = list.slice()
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

function serializeItem(item, sortOrder) {
  const kind = item.kind === 'category' ? 'category' : 'link'
  const out = {
    id: item.id,
    label: String(item.label || '').trim(),
    href: String(item.href || '').trim(),
    page_id: item.page_id || '',
    kind,
    visible: item.visible !== false,
    sort_order: sortOrder,
  }
  if (kind === 'category') {
    out.children = (item.children || []).map((child, i) =>
      serializeItem({ ...child, kind: 'link' }, i)
    )
  }
  return out
}

function PageSelect({ pages, value, onChange }) {
  return (
    <select value={value || ''} onChange={(e) => onChange(e.target.value)}>
      <option value="">Custom URL</option>
      {pages.map((p) => (
        <option key={p.id} value={p.id}>
          {pageLabel(p)}
        </option>
      ))}
    </select>
  )
}

function LinkFields({ item, pages, onChange }) {
  function pickPage(pageId) {
    if (!pageId) {
      onChange({ page_id: '', href: item.href })
      return
    }
    const page = pages.find((p) => p.id === pageId)
    const next = { page_id: pageId }
    if (page) {
      next.href = pageHref(page)
      if (!String(item.label || '').trim()) {
        next.label = page.nav_label || page.title || ''
      }
    }
    onChange(next)
  }

  return (
    <div className="menu-fields">
      <label>
        Page
        <PageSelect pages={pages} value={item.page_id} onChange={pickPage} />
      </label>
      <label>
        URL
        <input
          value={item.href || ''}
          placeholder="/about or https://…"
          onChange={(e) => onChange({ href: e.target.value })}
        />
      </label>
    </div>
  )
}

function ChildRow({ child, index, total, pages, onChange, onMove, onRemove }) {
  return (
    <div className="menu-child">
      <div className="menu-child-grid">
        <div className="menu-move">
          <button
            type="button"
            className="icon-btn"
            disabled={index === 0}
            onClick={() => onMove(index, index - 1)}
            aria-label="Move child up"
          >
            ↑
          </button>
          <button
            type="button"
            className="icon-btn"
            disabled={index === total - 1}
            onClick={() => onMove(index, index + 1)}
            aria-label="Move child down"
          >
            ↓
          </button>
        </div>
        <label>
          Label
          <input
            value={child.label || ''}
            placeholder="EDITORIAL I"
            onChange={(e) => onChange({ label: e.target.value })}
          />
        </label>
        <label className="check menu-visible">
          <input
            type="checkbox"
            checked={child.visible !== false}
            onChange={(e) => onChange({ visible: e.target.checked })}
          />
          Visible
        </label>
        <button type="button" className="secondary danger small" onClick={onRemove}>
          Remove
        </button>
      </div>
      <LinkFields item={child} pages={pages} onChange={onChange} />
    </div>
  )
}

function TopItem({ item, index, total, pages, onChange, onMove, onRemove }) {
  function patch(partial) {
    onChange({ ...item, ...partial })
  }

  function setKind(nextKind) {
    if (nextKind === item.kind) return
    if (nextKind === 'link' && (item.children || []).length) {
      if (!confirm('Switching to a link removes dropdown children. Continue?')) return
      patch({ kind: 'link', children: [], href: item.href, page_id: item.page_id })
      return
    }
    patch({
      kind: 'category',
      href: '',
      page_id: '',
      children: item.children || [],
    })
  }

  function patchChild(idx, partial) {
    const children = (item.children || []).map((c, i) => (i === idx ? { ...c, ...partial } : c))
    patch({ children })
  }

  const isDropdown = item.kind === 'category'

  return (
    <article className="menu-item">
      <div className="menu-item-head">
        <div className="menu-move">
          <button
            type="button"
            className="icon-btn"
            disabled={index === 0}
            onClick={() => onMove(index, index - 1)}
            aria-label="Move up"
          >
            ↑
          </button>
          <button
            type="button"
            className="icon-btn"
            disabled={index === total - 1}
            onClick={() => onMove(index, index + 1)}
            aria-label="Move down"
          >
            ↓
          </button>
        </div>
        <label className="menu-label-field">
          Label
          <input
            value={item.label || ''}
            placeholder={isDropdown ? 'BEAUTY' : 'ABOUT'}
            onChange={(e) => patch({ label: e.target.value })}
          />
        </label>
        <label>
          Type
          <select value={isDropdown ? 'category' : 'link'} onChange={(e) => setKind(e.target.value)}>
            <option value="link">Link</option>
            <option value="category">Dropdown</option>
          </select>
        </label>
        <label className="check menu-visible">
          <input
            type="checkbox"
            checked={item.visible !== false}
            onChange={(e) => patch({ visible: e.target.checked })}
          />
          Visible
        </label>
        <button type="button" className="secondary danger small" onClick={onRemove}>
          Remove
        </button>
      </div>

      {!isDropdown && <LinkFields item={item} pages={pages} onChange={patch} />}

      {isDropdown && (
        <div className="menu-children">
          <div className="menu-children-head">
            <h3>Dropdown links</h3>
            <button type="button" className="secondary small" onClick={() => patch({ children: [...(item.children || []), blankLink()] })}>
              Add link
            </button>
          </div>
          {!(item.children || []).length && (
            <p className="muted">
              Add at least one child link. Empty dropdowns are omitted after save.
            </p>
          )}
          {(item.children || []).map((child, i) => (
            <ChildRow
              key={child.id || i}
              child={child}
              index={i}
              total={(item.children || []).length}
              pages={pages}
              onChange={(partial) => patchChild(i, partial)}
              onMove={(from, to) => patch({ children: moveItem(item.children || [], from, to) })}
              onRemove={() =>
                patch({ children: (item.children || []).filter((_, idx) => idx !== i) })
              }
            />
          ))}
        </div>
      )}
    </article>
  )
}

export default function NavEditor() {
  const toast = useToast()
  const [items, setItems] = useState([])
  const [pages, setPages] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [navRes, pagesRes] = await Promise.all([
        admin.nav.get(),
        admin.pages.list().catch(() => ({ pages: [] })),
      ])
      setItems((navRes.nav || navRes.items || navRes.tree || []).map(normalizeItem))
      setPages(pagesRes.pages || pagesRes.items || [])
    } catch (e) {
      setItems([])
      toast.error(e.message)
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    load()
  }, [load])

  function updateItem(idx, next) {
    setItems((prev) => prev.map((item, i) => (i === idx ? next : item)))
  }

  async function save() {
    setSaving(true)
    try {
      const payload = items.map((item, i) => serializeItem(item, i))
      const data = await admin.nav.put({ nav: payload })
      setItems((data.nav || payload).map(normalizeItem))
      toast.ok('Menu saved. Preview will refresh automatically.')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Menu</h1>
          <p className="muted">
            Header navigation: top-level links and dropdowns (like BEAUTY). Page content is edited
            separately under <Link to="/pages">Pages</Link>.
          </p>
        </div>
        <div className="bar tight">
          <button type="button" className="secondary" onClick={load} disabled={loading || saving}>
            Reload
          </button>
          <button type="button" onClick={save} disabled={loading || saving}>
            {saving ? 'Saving…' : 'Save menu'}
          </button>
        </div>
      </header>

      <section className="section">
        <h2>Add item</h2>
        <div className="bar tight">
          <button type="button" className="secondary" onClick={() => setItems((prev) => [...prev, blankLink()])}>
            Add link
          </button>
          <button
            type="button"
            className="secondary"
            onClick={() => setItems((prev) => [...prev, blankDropdown()])}
          >
            Add dropdown
          </button>
        </div>
      </section>

      <section className="section">
        <h2>Menu tree {loading ? '…' : `(${items.length})`}</h2>
        {!loading && !items.length && (
          <p className="muted">No menu items yet. Add a link or a dropdown above.</p>
        )}
        <div className="menu-tree">
          {items.map((item, idx) => (
            <TopItem
              key={item.id || idx}
              item={item}
              index={idx}
              total={items.length}
              pages={pages}
              onChange={(next) => updateItem(idx, next)}
              onMove={(from, to) => setItems((prev) => moveItem(prev, from, to))}
              onRemove={() => {
                if (!confirm('Remove this menu item?')) return
                setItems((prev) => prev.filter((_, i) => i !== idx))
              }}
            />
          ))}
        </div>
      </section>
    </div>
  )
}
