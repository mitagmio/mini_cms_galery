import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { admin, apiUrl } from '../api'
import { BLOCK_PALETTE, allowedBlocksForTheme, mediaUrl, newBlock, templateLabel, TEMPLATES } from '../blockTypes'
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
  const [navTree, setNavTree] = useState(null)
  const [navLoaded, setNavLoaded] = useState(false)
  const [navError, setNavError] = useState('')
  const [savingNav, setSavingNav] = useState(false)
  const [navDirty, setNavDirty] = useState(false)
  const [menu, setMenu] = useState(emptyMenuState())

  const selected = useMemo(
    () => blocks.find((b) => b.id === selectedId) || null,
    [blocks, selectedId]
  )

  const palette = useMemo(() => {
    const theme = page?.template || page?.theme
    const allowed = allowedBlocksForTheme(theme)
    if (!allowed) return BLOCK_PALETTE
    return BLOCK_PALETTE.filter((b) => allowed.includes(b.type))
  }, [page?.template, page?.theme])

  const isLookbook = (page?.template || page?.theme) === 'lookbook_gallery'

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
    setNavLoaded(false)
    setNavTree(null)
    setNavDirty(false)
    setNavError('')
    setPage(null)
    setMenu(emptyMenuState())
    ;(async () => {
      try {
        const [pRes, bRes, nRes] = await Promise.all([
          admin.pages.get(id),
          admin.pages.getBlocks(id).catch(() => ({ blocks: [] })),
          admin.nav.get().catch((e) => ({ __error: e })),
        ])
        if (cancelled) return
        const p = pRes.page || pRes
        const nextPage = {
          title: '',
          slug: '',
          template: 'ba_content',
          is_homepage: false,
          ...p,
          seo: {
            meta_title: p.seo?.meta_title || p.title || '',
            meta_description: p.seo?.meta_description || p.meta_description || '',
            canonical_path: p.seo?.canonical_path || '',
            og_image_media_id: p.seo?.og_image_media_id || p.og_image || '',
          },
        }
        setPage(nextPage)
        const bl = normalizeBlocks(bRes.blocks || bRes.items || p.blocks || [])
        setBlocks(bl)
        setSelectedId(bl[0]?.id || null)
        if (nRes?.__error) {
          setNavError(nRes.__error.message || 'Could not load menu')
          setNavLoaded(false)
          setMenu(initMenuState([], nextPage))
        } else {
          const tree = nRes.nav || nRes.items || nRes.tree
          if (!Array.isArray(tree)) {
            setNavError('Could not load menu')
            setNavLoaded(false)
            setNavTree(null)
            setMenu(initMenuState([], nextPage))
          } else {
            setNavTree(tree)
            setNavLoaded(true)
            setNavError('')
            setMenu(initMenuState(tree, nextPage))
          }
        }
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

  function patchMenu(patch) {
    setMenu((prev) => {
      const next = { ...prev, ...patch }
      if (patch.placement || patch.categoryId != null || patch.include === true) {
        next.sortIndex = siblingList(navTree || [], next.placement, next.categoryId, id).length
      }
      return next
    })
    setNavDirty(true)
    if (patch.label != null) {
      setPage((prev) => (prev ? { ...prev, nav_label: patch.label } : prev))
    }
  }

  async function persistNav(currentPage, { quiet } = {}) {
    if (!navLoaded || !currentPage || !Array.isArray(navTree)) return false
    const tree = buildUpdatedNav(navTree, currentPage, menu)
    const savedNav = await admin.nav.put({ nav: tree })
    const nextTree = savedNav.nav || savedNav.items || savedNav.tree || tree
    setNavTree(nextTree)
    setMenu(initMenuState(nextTree, currentPage))
    setNavDirty(false)
    if (!quiet) toast.ok('Menu saved. Preview will refresh automatically.')
    return true
  }

  async function saveMenu() {
    if (!page) return
    setSavingNav(true)
    try {
      await persistNav(page)
    } catch (e) {
      toast.error(e.message)
    } finally {
      setSavingNav(false)
    }
  }

  async function save() {
    if (!page) return
    setSaving(true)
    try {
      const patched = await admin.pages.patch(id, {
        title: page.title,
        slug: page.slug,
        template: page.template,
        is_homepage: Boolean(page.is_homepage),
        seo: page.seo || {},
        nav_label: (menu.include ? menu.label : page.nav_label) || page.title,
        status: page.status || 'draft',
      })
      const p = patched.page || patched
      if (p && typeof p === 'object') {
        setPage((prev) =>
          prev
            ? {
                ...prev,
                ...p,
                settings: p.settings || {},
            seo: {
                  meta_title: p.seo?.meta_title || p.title || prev.seo?.meta_title || '',
                  meta_description:
                    p.seo?.meta_description || p.meta_description || prev.seo?.meta_description || '',
                  canonical_path: p.seo?.canonical_path || prev.seo?.canonical_path || '',
                  og_image_media_id:
                    p.seo?.og_image_media_id || p.og_image || prev.seo?.og_image_media_id || '',
                },
              }
            : prev,
        )
      }
      const payload = blocks.map((b, i) => ({
        id: String(b.id).startsWith('local-') ? undefined : b.id,
        type: b.type,
        position: i,
        data: b.data || {},
      }))
      const saved = await admin.pages.putBlocks(id, payload)
      const bl = normalizeBlocks(saved.blocks || saved.items || payload)
      setBlocks(bl)
      if (navLoaded) {
        const pageForNav = {
          ...page,
          ...(p && typeof p === 'object' ? p : {}),
          id: page.id || id,
        }
        try {
          await persistNav(pageForNav, { quiet: true })
        } catch (navErr) {
          toast.error(`Page saved, menu failed: ${navErr.message}`)
          return
        }
      }
      toast.ok(navLoaded ? 'Saved page and menu' : 'Saved')
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
      try {
        const pRes = await admin.pages.get(id)
        const p = pRes.page || pRes
        if (p?.settings) {
          setPage((prev) => (prev ? { ...prev, settings: p.settings } : prev))
        }
      } catch {
        /* seed refresh is optional */
      }
    } catch (e) {
      toast.error(e.message)
    }
  }

  async function reshuffleLookbook() {
    try {
      const seed = Date.now()
      const patched = await admin.pages.patch(id, {
        settings: { ...(page?.settings || {}), shuffle_seed: seed },
      })
      const p = patched.page || patched
      if (p?.settings) {
        setPage((prev) => (prev ? { ...prev, settings: p.settings } : prev))
      } else {
        setPage((prev) => (prev ? { ...prev, settings: { ...(prev.settings || {}), shuffle_seed: seed } } : prev))
      }
      await admin.generate({ page_id: id })
      toast.ok('Lookbook reshuffled — generate draft applied')
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
          {isLookbook ? (
            <button type="button" className="secondary" onClick={reshuffleLookbook}>
              Reshuffle
            </button>
          ) : null}
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
          {palette.map((b) => (
            <button key={b.type} type="button" className="palette-item" onClick={() => addBlock(b.type)}>
              <strong>{b.label}</strong>
              <span className="muted">{b.hint}</span>
            </button>
          ))}
        </aside>

        <section className="canvas">
          <h3>Canvas</h3>
          {!blocks.length && (
            <p className="empty-canvas">Add blocks from the palette.</p>
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
            <button
              type="button"
              className={inspectorTab === 'menu' ? 'active' : ''}
              onClick={() => setInspectorTab('menu')}
            >
              Menu
            </button>
          </div>

          {inspectorTab === 'menu' ? (
            <PageMenuPanel
              page={page}
              navTree={navTree || []}
              navLoaded={navLoaded}
              navError={navError}
              navDirty={navDirty}
              menu={menu}
              savingNav={savingNav}
              onChange={patchMenu}
              onSave={saveMenu}
            />
          ) : inspectorTab === 'seo' ? (
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
            <p className="muted inspector-body">Select a block, or open Page SEO / Menu.</p>
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
          Mailto (fallback)
          <input
            type="email"
            value={d.mailto || ''}
            onChange={(e) => onChange({ mailto: e.target.value })}
            placeholder="only if Settings → Contact email is empty"
          />
        </label>
        <p className="muted">Messages go to Settings → Contact email. This field is a fallback only.</p>
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

const NEW_DROPDOWN = '__new__'

function emptyMenuState() {
  return {
    include: false,
    label: '',
    placement: 'top',
    categoryId: NEW_DROPDOWN,
    newCategoryName: '',
    sortIndex: 0,
    existingId: '',
  }
}

function newNavId() {
  try {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  } catch {
    /* fall through */
  }
  return `n-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`
}

function pageHref(page) {
  if (!page || page.is_homepage) return '/'
  const slug = String(page.slug || '').replace(/^\/+|\/+$/g, '')
  return slug ? `/${slug}` : '/'
}

function cloneNav(tree) {
  return JSON.parse(JSON.stringify(tree || []))
}

function navCategories(tree) {
  return (tree || []).filter((n) => n.kind === 'category' || (n.children && n.children.length))
}

function findPageInNav(nodes, pageId, parent = null) {
  const list = nodes || []
  const pid = String(pageId || '')
  if (!pid) return null
  for (const item of list) {
    if (item.kind !== 'category' && String(item.page_id || '') === pid) {
      return { item, parent, siblings: list }
    }
    if (item.children?.length) {
      const inner = findPageInNav(item.children, pid, item)
      if (inner) return inner
    }
  }
  return null
}

function siblingList(tree, placement, categoryId, pageId) {
  const roots = tree || []
  const pid = String(pageId || '')
  const notThis = (n) => String(n.page_id || '') !== pid
  if (placement === 'dropdown' && categoryId && categoryId !== NEW_DROPDOWN) {
    const cat = roots.find((n) => n.id === categoryId)
    return (cat?.children || []).filter(notThis)
  }
  return roots.filter(notThis)
}

function initMenuState(tree, page) {
  const found = findPageInNav(tree, page?.id)
  const cats = navCategories(tree)
  if (!found) {
    return {
      include: false,
      label: page?.nav_label || page?.title || '',
      placement: 'top',
      categoryId: cats[0]?.id || NEW_DROPDOWN,
      newCategoryName: '',
      sortIndex: (tree || []).length,
      existingId: '',
    }
  }
  const sortIndex = Math.max(
    0,
    found.siblings.findIndex((s) => s.id === found.item.id),
  )
  return {
    include: true,
    label: found.item.label || page?.nav_label || page?.title || '',
    placement: found.parent ? 'dropdown' : 'top',
    categoryId: found.parent?.id || cats[0]?.id || NEW_DROPDOWN,
    newCategoryName: '',
    sortIndex,
    existingId: found.item.id || '',
  }
}

function removePageFromNav(nodes, pageId) {
  const pid = String(pageId || '')
  const out = []
  for (const item of nodes || []) {
    if (item.kind !== 'category' && String(item.page_id || '') === pid) continue
    const children = item.children?.length ? removePageFromNav(item.children, pid) : []
    if (item.kind === 'category' && children.length === 0) continue
    out.push({ ...item, children })
  }
  return out
}

function insertAt(list, item, index) {
  const raw = Number(index)
  const i = Number.isFinite(raw)
    ? Math.max(0, Math.min(raw, list.length))
    : list.length
  const next = [...list]
  next.splice(i, 0, item)
  return next
}

function reindexNav(nodes, parentId = '') {
  return (nodes || []).map((n, i) => ({
    ...n,
    parent_id: parentId,
    sort_order: i,
    visible: n.visible !== false,
    children: reindexNav(n.children || [], n.id || ''),
  }))
}

function serializeNavItem(item, sortOrder) {
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
      serializeNavItem({ ...child, kind: 'link' }, i)
    )
  }
  return out
}

function buildUpdatedNav(tree, page, menu) {
  if (menu.include) {
    const label = String(menu.label || page.nav_label || page.title || '').trim()
    if (!label) throw new Error('Menu label is required')
    if (menu.placement === 'dropdown') {
      if (menu.categoryId === NEW_DROPDOWN) {
        const name = String(menu.newCategoryName || '').trim()
        if (!name) throw new Error('Name the new dropdown')
      } else if (!menu.categoryId) {
        throw new Error('Choose a dropdown')
      }
    }
  }

  let next = removePageFromNav(cloneNav(tree), page.id)
  if (!menu.include) return reindexNav(next).map((item, i) => serializeNavItem(item, i))

  const link = {
    id: menu.existingId || newNavId(),
    label: String(menu.label || page.nav_label || page.title || 'Untitled').trim(),
    href: pageHref(page),
    page_id: page.id,
    parent_id: '',
    sort_order: 0,
    kind: 'link',
    visible: true,
    children: [],
  }

  if (menu.placement === 'dropdown' && menu.categoryId === NEW_DROPDOWN) {
    const catId = newNavId()
    const cat = {
      id: catId,
      label: String(menu.newCategoryName || '').trim(),
      href: '',
      page_id: '',
      parent_id: '',
      sort_order: 0,
      kind: 'category',
      visible: true,
      children: [{ ...link, parent_id: catId }],
    }
    next = insertAt(next, cat, menu.sortIndex)
  } else if (menu.placement === 'dropdown') {
    const catIdx = next.findIndex((n) => n.id === menu.categoryId)
    if (catIdx < 0) {
      next = insertAt(next, link, menu.sortIndex)
    } else {
      const cat = next[catIdx]
      const child = { ...link, parent_id: cat.id }
      const children = insertAt(cat.children || [], child, menu.sortIndex)
      next = next.map((n, i) => (i === catIdx ? { ...n, kind: 'category', children } : n))
    }
  } else {
    next = insertAt(next, link, menu.sortIndex)
  }
  return reindexNav(next).map((item, i) => serializeNavItem(item, i))
}

function PageMenuPanel({
  page,
  navTree,
  navLoaded,
  navError,
  navDirty,
  menu,
  savingNav,
  onChange,
  onSave,
}) {
  const saved = findPageInNav(navTree, page.id)
  const cats = navCategories(navTree)
  const others = siblingList(navTree, menu.placement, menu.categoryId, page.id)
  const savedWhere = !navLoaded
    ? 'Menu is not loaded'
    : !saved
      ? 'This page is not in the menu'
      : saved.parent
        ? `In the menu · inside “${saved.parent.label}”`
        : 'In the menu · top-level link'
  const posLabel =
    menu.placement === 'dropdown' && menu.categoryId !== NEW_DROPDOWN
      ? 'Position in dropdown'
      : menu.placement === 'dropdown'
        ? 'Dropdown position in top menu'
        : 'Position in top menu'
  const sortValue = String(Math.min(menu.sortIndex ?? others.length, others.length))

  return (
    <div className="inspector-body">
      <p className="menu-status">{savedWhere}</p>
      {navError ? <p className="error">{navError}</p> : null}
      {navDirty && navLoaded ? <p className="muted">Unsaved menu changes</p> : null}

      <label className="check">
        <input
          type="checkbox"
          checked={Boolean(menu.include)}
          disabled={!navLoaded}
          onChange={(e) => onChange({ include: e.target.checked })}
        />
        Include in menu
      </label>

      {menu.include ? (
        <>
          <p className="muted">
            {menu.placement === 'top'
              ? 'Will appear as a top-level link'
              : menu.categoryId === NEW_DROPDOWN
                ? `Will appear in new dropdown “${menu.newCategoryName || '…'}”`
                : `Will appear in “${cats.find((c) => c.id === menu.categoryId)?.label || '…'}”`}
          </p>
          <label>
            Label
            <input
              value={menu.label || ''}
              onChange={(e) => onChange({ label: e.target.value })}
              placeholder={page.title || 'Menu label'}
            />
          </label>
          <label>
            Placement
            <select
              value={menu.placement}
              onChange={(e) => onChange({ placement: e.target.value })}
            >
              <option value="top">Top-level link</option>
              <option value="dropdown">Inside dropdown</option>
            </select>
          </label>
          {menu.placement === 'dropdown' ? (
            <>
              <label>
                Dropdown
                <select
                  value={menu.categoryId}
                  onChange={(e) => onChange({ categoryId: e.target.value })}
                >
                  {cats.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.label}
                    </option>
                  ))}
                  <option value={NEW_DROPDOWN}>Create new dropdown…</option>
                </select>
              </label>
              {menu.categoryId === NEW_DROPDOWN ? (
                <label>
                  New dropdown name
                  <input
                    value={menu.newCategoryName || ''}
                    onChange={(e) => onChange({ newCategoryName: e.target.value })}
                    placeholder="e.g. BEAUTY"
                  />
                </label>
              ) : null}
            </>
          ) : null}
          <label>
            {posLabel}
            <select
              value={sortValue}
              onChange={(e) => onChange({ sortIndex: Number(e.target.value) })}
            >
              <option value="0">First</option>
              {others.map((s, i) => (
                <option key={s.id || i} value={String(i + 1)}>
                  After {s.label || 'item'}
                </option>
              ))}
            </select>
          </label>
        </>
      ) : null}

      <button type="button" onClick={onSave} disabled={!navLoaded || savingNav}>
        {savingNav ? 'Saving menu…' : 'Save menu'}
      </button>
      <Link to="/nav" className="menu-full-link">
        Edit full menu →
      </Link>
    </div>
  )
}
