import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { admin, apiUrl } from '../api'
import { BLOCK_PALETTE, mediaUrl, newBlock, templateLabel, TEMPLATES } from '../blockTypes'
import MediaPicker from '../components/MediaPicker'
import { useToast } from '../toast'

export default function PageEditor() {
  const { id } = useParams()
  const toast = useToast()
  const [page, setPage] = useState(null)
  const [blocks, setBlocks] = useState([])
  const [selectedId, setSelectedId] = useState(null)
  const [saving, setSaving] = useState(false)
  const [inspectorTab, setInspectorTab] = useState('block')
  const [mediaIndex, setMediaIndex] = useState({})
  const [picker, setPicker] = useState(null)

  const selected = useMemo(
    () => blocks.find((b) => b.id === selectedId) || null,
    [blocks, selectedId]
  )

  const loadMediaIndex = useCallback(async () => {
    try {
      const data = await admin.media.list()
      const list = data.media || data.items || []
      const map = {}
      for (const m of list) map[m.id] = m
      setMediaIndex(map)
    } catch {
      /* picker / thumbs optional */
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const [pRes, bRes] = await Promise.all([
          admin.pages.get(id),
          admin.pages.getBlocks(id).catch(() => ({ blocks: [] })),
        ])
        if (cancelled) return
        const p = pRes.page || pRes
        setPage({
          title: '',
          slug: '',
          template: 'ba_content',
          is_homepage: false,
          ...p,
          seo: { ...(p.seo || {}) },
        })
        const bl = normalizeBlocks(bRes.blocks || bRes.items || p.blocks || [])
        setBlocks(bl)
        setSelectedId(bl[0]?.id || null)
      } catch (e) {
        toast.error(e.message)
      }
      loadMediaIndex()
    })()
    return () => {
      cancelled = true
    }
  }, [id, toast, loadMediaIndex])

  function updatePage(patch) {
    setPage((prev) => (prev ? { ...prev, ...patch } : prev))
  }

  function updateSeo(patch) {
    setPage((prev) =>
      prev ? { ...prev, seo: { ...(prev.seo || {}), ...patch } } : prev
    )
  }

  function updateBlockData(blockId, patch) {
    setBlocks((prev) =>
      prev.map((b) => (b.id === blockId ? { ...b, data: { ...b.data, ...patch } } : b))
    )
  }

  function addBlock(type) {
    const b = newBlock(type)
    setBlocks((prev) => [...prev, b])
    setSelectedId(b.id)
    setInspectorTab('block')
  }

  function moveBlock(blockId, dir) {
    setBlocks((prev) => {
      const i = prev.findIndex((b) => b.id === blockId)
      if (i < 0) return prev
      const j = i + dir
      if (j < 0 || j >= prev.length) return prev
      const next = [...prev]
      ;[next[i], next[j]] = [next[j], next[i]]
      return next
    })
  }

  function removeBlock(blockId) {
    setBlocks((prev) => prev.filter((b) => b.id !== blockId))
    if (selectedId === blockId) setSelectedId(null)
  }

  async function save() {
    if (!page) return
    setSaving(true)
    try {
      await admin.pages.patch(id, {
        title: page.title,
        slug: page.slug,
        template: page.template,
        is_homepage: Boolean(page.is_homepage),
        seo: page.seo || {},
        nav_label: page.nav_label || page.title,
        status: page.status || 'draft',
      })
      const payload = blocks.map((b, i) => ({
        id: String(b.id).startsWith('local-') ? undefined : b.id,
        type: b.type,
        position: i,
        data: b.data || {},
      }))
      const saved = await admin.pages.putBlocks(id, payload)
      const bl = normalizeBlocks(saved.blocks || saved.items || payload)
      setBlocks(bl)
      toast.ok('Saved')
    } catch (e) {
      toast.error(e.message)
    } finally {
      setSaving(false)
    }
  }

  async function generateDraft() {
    try {
      await save()
      const res = await admin.generate({ page_id: id })
      toast.ok(res.message || 'Draft generated')
    } catch (e) {
      toast.error(e.message)
    }
  }

  async function previewPage() {
    try {
      const res = await admin.preview(id)
      const url = res.preview_url || res.url || `/preview/${page?.slug || ''}`
      window.open(apiUrl(url), '_blank', 'noopener')
    } catch (e) {
      toast.error(e.message)
    }
  }

  async function publishPage() {
    try {
      await save()
      await admin.publish({ page_id: id })
      toast.ok('Publish requested')
    } catch (e) {
      toast.error(e.message)
    }
  }

  if (!page) {
    return (
      <div className="page">
        <p className="muted">Loading editor…</p>
      </div>
    )
  }

  return (
    <div className="editor-page">
      <div className="editor-toolbar">
        <div className="editor-toolbar-left">
          <Link to="/pages" className="muted">
            ← Pages
          </Link>
          <input
            className="title-input"
            value={page.title || ''}
            onChange={(e) => updatePage({ title: e.target.value })}
            placeholder="Page title"
          />
          <span className="badge">{templateLabel(page.template)}</span>
        </div>
        <div className="bar tight">
          <button type="button" className="secondary" onClick={generateDraft}>
            Generate draft
          </button>
          <button type="button" className="secondary" onClick={previewPage}>
            Preview
          </button>
          <button type="button" className="secondary" onClick={publishPage}>
            Publish page
          </button>
          <button type="button" onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>

      <div className="editor-meta bar">
        <label>
          Slug
          <input value={page.slug || ''} onChange={(e) => updatePage({ slug: e.target.value })} />
        </label>
        <label>
          Template
          <select
            value={page.template || 'ba_content'}
            onChange={(e) => updatePage({ template: e.target.value })}
          >
            {TEMPLATES.map((t) => (
              <option key={t.id} value={t.id}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={Boolean(page.is_homepage)}
            onChange={(e) => updatePage({ is_homepage: e.target.checked })}
          />
          Homepage
        </label>
      </div>

      <div className="editor-panes">
        <aside className="palette">
          <h3>Blocks</h3>
          {BLOCK_PALETTE.map((b) => (
            <button key={b.type} type="button" className="palette-item" onClick={() => addBlock(b.type)}>
              <strong>{b.label}</strong>
              <span className="muted">{b.hint}</span>
            </button>
          ))}
        </aside>

        <section className="canvas">
          <h3>Canvas</h3>
          {!blocks.length && (
            <p className="empty-canvas">Add blocks from the palette (BA pair, gallery, text, form).</p>
          )}
          {blocks.map((b, idx) => (
            <div
              key={b.id}
              className={`block-card ${selectedId === b.id ? 'selected' : ''}`}
              onClick={() => {
                setSelectedId(b.id)
                setInspectorTab('block')
              }}
            >
              <div className="block-card-head">
                <strong>{labelForType(b.type)}</strong>
                <span className="muted">#{idx + 1}</span>
                <div className="row-actions" onClick={(e) => e.stopPropagation()}>
                  <button type="button" className="icon-btn" onClick={() => moveBlock(b.id, -1)} title="Move up">
                    ↑
                  </button>
                  <button type="button" className="icon-btn" onClick={() => moveBlock(b.id, 1)} title="Move down">
                    ↓
                  </button>
                  <button type="button" className="icon-btn danger" onClick={() => removeBlock(b.id)}>
                    ×
                  </button>
                </div>
              </div>
              <BlockPreview block={b} mediaIndex={mediaIndex} />
            </div>
          ))}
        </section>

        <aside className="inspector">
          <div className="tabs">
            <button
              type="button"
              className={inspectorTab === 'block' ? 'active' : ''}
              onClick={() => setInspectorTab('block')}
            >
              Block
            </button>
            <button
              type="button"
              className={inspectorTab === 'seo' ? 'active' : ''}
              onClick={() => setInspectorTab('seo')}
            >
              Page SEO
            </button>
          </div>

          {inspectorTab === 'seo' ? (
            <div className="inspector-body">
              <label>
                Meta title
                <input
                  value={page.seo?.meta_title || ''}
                  onChange={(e) => updateSeo({ meta_title: e.target.value })}
                />
              </label>
              <label>
                Meta description
                <textarea
                  rows={3}
                  value={page.seo?.meta_description || ''}
                  onChange={(e) => updateSeo({ meta_description: e.target.value })}
                />
              </label>
              <label>
                Canonical path
                <input
                  value={page.seo?.canonical_path || ''}
                  onChange={(e) => updateSeo({ canonical_path: e.target.value })}
                  placeholder={`/${page.slug || ''}`}
                />
              </label>
              <div className="field-row">
                <span>OG image</span>
                <OgThumb media={mediaIndex[page.seo?.og_image_media_id]} />
                <button
                  type="button"
                  className="secondary"
                  onClick={() =>
                    setPicker({
                      field: 'og',
                      multi: false,
                      title: 'Pick OG image',
                    })
                  }
                >
                  Pick
                </button>
              </div>
            </div>
          ) : selected ? (
            <BlockInspector
              block={selected}
              mediaIndex={mediaIndex}
              onChange={(patch) => updateBlockData(selected.id, patch)}
              onPick={(cfg) => setPicker({ ...cfg, blockId: selected.id })}
              onSwap={() => {
                updateBlockData(selected.id, {
                  before_media_id: selected.data?.after_media_id || null,
                  after_media_id: selected.data?.before_media_id || null,
                })
              }}
              onDelete={() => removeBlock(selected.id)}
            />
          ) : (
            <p className="muted inspector-body">Select a block or open Page SEO.</p>
          )}
        </aside>
      </div>

      <MediaPicker
        open={Boolean(picker)}
        title={picker?.title || 'Pick media'}
        multi={Boolean(picker?.multi)}
        kindFilter={picker?.kind || ''}
        onClose={() => setPicker(null)}
        onSelect={(pick) => {
          if (!picker) return
          if (picker.field === 'og') {
            const m = Array.isArray(pick) ? pick[0] : pick
            updateSeo({ og_image_media_id: m.id })
            setMediaIndex((prev) => ({ ...prev, [m.id]: m }))
            return
          }
          if (picker.blockId) {
            if (picker.field === 'gallery_multi') {
              const list = Array.isArray(pick) ? pick : [pick]
              const extras = list.map((m) => {
                const b = newBlock('gallery_image')
                b.data = { media_id: m.id, alt: m.title || '', caption: '' }
                return b
              })
              setBlocks((prev) => {
                const i = prev.findIndex((b) => b.id === picker.blockId)
                if (i < 0) return [...prev, ...extras]
                const next = [...prev]
                next.splice(i + 1, 0, ...extras)
                return next
              })
              list.forEach((m) => setMediaIndex((prev) => ({ ...prev, [m.id]: m })))
              return
            }
            const m = Array.isArray(pick) ? pick[0] : pick
            updateBlockData(picker.blockId, { [picker.field]: m.id })
            setMediaIndex((prev) => ({ ...prev, [m.id]: m }))
          }
        }}
      />
    </div>
  )
}

function normalizeBlocks(list) {
  return (list || []).map((b, i) => ({
    id: b.id || `local-${i}-${Date.now()}`,
    type: b.type,
    data: b.data || {},
  }))
}

function labelForType(type) {
  return BLOCK_PALETTE.find((b) => b.type === type)?.label || type
}

function OgThumb({ media }) {
  const src = media ? apiUrl(mediaUrl(media)) : ''
  if (!src) return <span className="muted">none</span>
  return <img className="mini-thumb" src={src} alt="" />
}

function BlockPreview({ block, mediaIndex }) {
  const d = block.data || {}
  if (block.type === 'comparison_slider') {
    const before = mediaIndex[d.before_media_id]
    const after = mediaIndex[d.after_media_id]
    return (
      <div className="ba-preview">
        <Thumb media={before} label="Before" />
        <span className="ba-sep">|</span>
        <Thumb media={after} label="After" />
        {d.caption ? <span className="muted">{d.caption}</span> : null}
      </div>
    )
  }
  if (block.type === 'gallery_image') {
    return (
      <div className="ba-preview">
        <Thumb media={mediaIndex[d.media_id]} label="Image" />
        <span className="muted">{d.alt || d.caption || 'gallery image'}</span>
      </div>
    )
  }
  if (block.type === 'rich_text') {
    return <div className="text-preview muted">{stripHtml(d.html).slice(0, 120) || 'Empty text'}</div>
  }
  if (block.type === 'contact_form') {
    return <div className="muted">Contact form · {d.heading || 'Contact'}</div>
  }
  return <div className="muted">{block.type}</div>
}

function Thumb({ media, label }) {
  const src = media ? apiUrl(mediaUrl(media)) : ''
  return (
    <div className="thumb-slot">
      {src ? <img src={src} alt={label} /> : <span>{label}</span>}
    </div>
  )
}

function BlockInspector({ block, mediaIndex, onChange, onPick, onSwap, onDelete }) {
  const d = block.data || {}

  if (block.type === 'comparison_slider') {
    return (
      <div className="inspector-body">
        <div className="field-row">
          <span>Before</span>
          <Thumb media={mediaIndex[d.before_media_id]} label="—" />
          <button
            type="button"
            className="secondary"
            onClick={() =>
              onPick({ field: 'before_media_id', kind: 'before', title: 'Pick before image' })
            }
          >
            Pick
          </button>
        </div>
        <div className="field-row">
          <span>After</span>
          <Thumb media={mediaIndex[d.after_media_id]} label="—" />
          <button
            type="button"
            className="secondary"
            onClick={() =>
              onPick({ field: 'after_media_id', kind: 'after', title: 'Pick after image' })
            }
          >
            Pick
          </button>
        </div>
        <button type="button" className="secondary" onClick={onSwap}>
          Swap before ↔ after
        </button>
        <label>
          Caption
          <input value={d.caption || ''} onChange={(e) => onChange({ caption: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }

  if (block.type === 'gallery_image') {
    return (
      <div className="inspector-body">
        <div className="field-row">
          <span>Image</span>
          <Thumb media={mediaIndex[d.media_id]} label="—" />
          <button
            type="button"
            className="secondary"
            onClick={() => onPick({ field: 'media_id', title: 'Pick gallery image' })}
          >
            Pick
          </button>
        </div>
        <button
          type="button"
          className="secondary"
          onClick={() =>
            onPick({
              field: 'gallery_multi',
              multi: true,
              title: 'Add more gallery images',
            })
          }
        >
          Add strip images…
        </button>
        <label>
          Alt
          <input value={d.alt || ''} onChange={(e) => onChange({ alt: e.target.value })} />
        </label>
        <label>
          Caption
          <input value={d.caption || ''} onChange={(e) => onChange({ caption: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }

  if (block.type === 'rich_text') {
    return (
      <div className="inspector-body">
        <label>
          HTML
          <textarea
            rows={12}
            value={d.html || ''}
            onChange={(e) => onChange({ html: e.target.value })}
          />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }

  if (block.type === 'contact_form') {
    return (
      <div className="inspector-body">
        <label>
          Heading
          <input value={d.heading || ''} onChange={(e) => onChange({ heading: e.target.value })} />
        </label>
        <label>
          Mailto
          <input value={d.mailto || ''} onChange={(e) => onChange({ mailto: e.target.value })} />
        </label>
        <label>
          Success message
          <input
            value={d.success_message || ''}
            onChange={(e) => onChange({ success_message: e.target.value })}
          />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }

  return (
    <div className="inspector-body">
      <p className="muted">Unknown block type: {block.type}</p>
      <button type="button" className="secondary danger" onClick={onDelete}>
        Delete block
      </button>
    </div>
  )
}

function stripHtml(html) {
  return String(html || '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}
