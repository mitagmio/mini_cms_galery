import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { admin, ApiError } from '../api'
import { BLOCK_PALETTE, TEMPLATES, newBlock } from '../blockTypes'
import { useToast } from '../toast'

const THEME_OPTIONS = TEMPLATES.map((t) => ({
  id: t.id,
  label: t.label,
}))

const EMPTY_FORM = {
  id: '',
  name: '',
  theme: 'ba_content',
  description: '',
  allowed_blocks: ['comparison_slider'],
  default_blocks_json: '[]',
}

/** Local starters used when API templates are unavailable. */
const FALLBACK_STARTERS = [
  {
    id: 'ba_content',
    theme: 'ba_content',
    name: 'BA page',
    description: TEMPLATES[0].description,
    allowed_blocks: ['comparison_slider'],
    default_blocks: [
      { type: 'comparison_slider', data: newBlock('comparison_slider').data },
      { type: 'comparison_slider', data: newBlock('comparison_slider').data },
    ],
    is_system: true,
  },
  {
    id: 'panorama_gallery',
    theme: 'panorama_gallery',
    name: 'Gallery',
    description: TEMPLATES[1].description,
    allowed_blocks: ['gallery_image'],
    default_blocks: [
      { type: 'gallery_image', data: newBlock('gallery_image').data },
      { type: 'gallery_image', data: newBlock('gallery_image').data },
      { type: 'gallery_image', data: newBlock('gallery_image').data },
    ],
    is_system: true,
  },
  {
    id: 'text_content',
    theme: 'text_content',
    name: 'Blank / text',
    description: TEMPLATES[2].description,
    allowed_blocks: ['rich_text', 'contact_form'],
    default_blocks: [{ type: 'rich_text', data: newBlock('rich_text').data }],
    is_system: true,
  },
]

const DEFAULT_BLOCKS_BY_THEME = {
  ba_content: () => [
    { type: 'comparison_slider', data: newBlock('comparison_slider').data },
    { type: 'comparison_slider', data: newBlock('comparison_slider').data },
  ],
  panorama_gallery: () => [
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
  ],
  text_content: () => [{ type: 'rich_text', data: newBlock('rich_text').data }],
}

const ALLOWED_BY_THEME = {
  ba_content: ['comparison_slider'],
  panorama_gallery: ['gallery_image'],
  text_content: ['rich_text', 'contact_form'],
}

export default function Templates() {
  const toast = useToast()
  const navigate = useNavigate()
  const [templates, setTemplates] = useState([])
  const [loading, setLoading] = useState(true)
  const [apiReady, setApiReady] = useState(true)
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const [form, setForm] = useState(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const data = await admin.templates.list()
      const list = data.templates || data.items || []
      setTemplates(list.map(normalizeTemplate))
      setApiReady(true)
    } catch (e) {
      setTemplates(FALLBACK_STARTERS.map(normalizeTemplate))
      setApiReady(!(e instanceof ApiError && e.status === 404))
      if (!(e instanceof ApiError && e.status === 404)) {
        toast.error(e.message)
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function openCreate() {
    const theme = 'ba_content'
    setEditingId(null)
    setForm({
      ...EMPTY_FORM,
      theme,
      allowed_blocks: [...(ALLOWED_BY_THEME[theme] || [])],
      default_blocks_json: JSON.stringify(DEFAULT_BLOCKS_BY_THEME[theme](), null, 2),
    })
    setFormOpen(true)
  }

  function openEdit(tmpl) {
    setEditingId(tmpl.id)
    setForm({
      id: tmpl.id,
      name: tmpl.name || '',
      theme: tmpl.theme || tmpl.key || 'ba_content',
      description: tmpl.description || '',
      allowed_blocks: [...(tmpl.allowed_blocks || [])],
      default_blocks_json: JSON.stringify(tmpl.default_blocks || [], null, 2),
    })
    setFormOpen(true)
  }

  function closeForm() {
    setFormOpen(false)
    setEditingId(null)
    setForm(EMPTY_FORM)
  }

  function patchForm(field, value) {
    setForm((prev) => ({ ...prev, [field]: value }))
  }

  function onThemeChange(theme) {
    setForm((prev) => ({
      ...prev,
      theme,
      allowed_blocks: [...(ALLOWED_BY_THEME[theme] || [])],
      default_blocks_json: JSON.stringify(
        (DEFAULT_BLOCKS_BY_THEME[theme] || (() => []))(),
        null,
        2
      ),
    }))
  }

  function toggleAllowed(type) {
    setForm((prev) => {
      const has = prev.allowed_blocks.includes(type)
      const allowed_blocks = has
        ? prev.allowed_blocks.filter((t) => t !== type)
        : [...prev.allowed_blocks, type]
      return { ...prev, allowed_blocks }
    })
  }

  async function save(e) {
    e.preventDefault()
    if (!apiReady) {
      toast.error('Templates API is not available yet')
      return
    }
    let default_blocks
    try {
      default_blocks = JSON.parse(form.default_blocks_json || '[]')
      if (!Array.isArray(default_blocks)) throw new Error('must be an array')
    } catch {
      toast.error('Default blocks must be valid JSON array')
      return
    }

    const name = form.name.trim()
    if (!name) {
      toast.error('Name is required')
      return
    }

    const body = {
      name,
      label: name,
      description: form.description.trim(),
      theme: form.theme,
      key: form.theme,
      allowed_blocks: form.allowed_blocks,
      default_blocks,
    }

    setSaving(true)
    try {
      if (editingId) {
        await admin.templates.patch(editingId, body)
        toast.ok('Template saved')
      } else {
        const id = (form.id || '').trim() || slugify(name)
        if (id && !['ba_content', 'panorama_gallery', 'text_content'].includes(id)) {
          body.id = id
        }
        await admin.templates.create(body)
        toast.ok('Template created')
      }
      closeForm()
      await load()
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  async function createFrom(tmpl) {
    const title = tmpl.name || tmpl.label || 'Untitled'
    const slug = slugify(title)
    const theme = tmpl.theme || tmpl.key || tmpl.id
    try {
      const data = await admin.pages.create({
        title,
        slug,
        template: theme,
        is_homepage: false,
        seo: {},
      })
      const page = data.page || data
      const starters = Array.isArray(tmpl.default_blocks) ? tmpl.default_blocks : []
      const blocks = starters.map((b, i) => ({
        type: b.type,
        position: i,
        data: b.data != null ? b.data : newBlock(b.type).data,
      }))
      if (blocks.length) {
        try {
          await admin.pages.putBlocks(page.id, blocks)
        } catch (e) {
          toast.error(`Page created; blocks: ${e.message}`)
          navigate(`/pages/${page.id}`)
          return
        }
      }
      toast.ok(`Created “${title}”`)
      navigate(`/pages/${page.id}`)
    } catch (e) {
      toast.error(e.message)
    }
  }

  const editing = editingId
    ? templates.find((t) => t.id === editingId)
    : null
  const themeLocked = Boolean(editing?.is_system)

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Templates</h1>
          <p className="muted">
            Page blueprints — use to create a page, or edit metadata and defaults
          </p>
        </div>
        <div className="bar tight">
          <button type="button" onClick={openCreate} disabled={!apiReady && !loading}>
            Create template
          </button>
        </div>
      </header>

      {!apiReady && !loading && (
        <p className="muted">
          Templates API not ready — showing built-in starters (Use still works). Create/Edit
          will enable when <code>/api/admin/templates</code> is available.
        </p>
      )}

      {formOpen && (
        <section className="section">
          <h2>{editingId ? 'Edit template' : 'Create template'}</h2>
          <form className="stack" onSubmit={save}>
            <label>
              Name
              <input
                value={form.name}
                onChange={(e) => patchForm('name', e.target.value)}
                required
              />
            </label>
            {!editingId && (
              <label>
                Id (optional)
                <input
                  value={form.id}
                  onChange={(e) => patchForm('id', e.target.value)}
                  placeholder="auto from name"
                />
              </label>
            )}
            {editingId && (
              <p className="muted">
                Id: <code>{editingId}</code>
                {editing?.is_system ? ' · system' : ''}
              </p>
            )}
            <label>
              Theme engine
              <select
                value={form.theme}
                onChange={(e) => onThemeChange(e.target.value)}
                disabled={themeLocked}
              >
                {THEME_OPTIONS.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.label} ({t.id})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Description
              <textarea
                rows={2}
                value={form.description}
                onChange={(e) => patchForm('description', e.target.value)}
              />
            </label>
            <fieldset className="block-checkboxes">
              <legend>Allowed blocks</legend>
              <div className="bar tight">
                {BLOCK_PALETTE.map((b) => (
                  <label key={b.type} className="check">
                    <input
                      type="checkbox"
                      checked={form.allowed_blocks.includes(b.type)}
                      onChange={() => toggleAllowed(b.type)}
                    />
                    {b.label}
                  </label>
                ))}
              </div>
            </fieldset>
            <label>
              Default blocks (JSON)
              <textarea
                className="code-area"
                rows={8}
                value={form.default_blocks_json}
                onChange={(e) => patchForm('default_blocks_json', e.target.value)}
                spellCheck={false}
              />
            </label>
            <div className="bar tight">
              <button type="submit" disabled={saving || !apiReady}>
                {saving ? 'Saving…' : editingId ? 'Save' : 'Create'}
              </button>
              <button type="button" className="secondary" onClick={closeForm}>
                Cancel
              </button>
            </div>
          </form>
        </section>
      )}

      <section className="section">
        <h2>All templates {loading ? '…' : `(${templates.length})`}</h2>
        <div className="template-grid">
          {templates.map((t) => (
            <article key={t.id} className="template-card">
              <h2>{t.name}</h2>
              <p className="muted">{t.description || '—'}</p>
              <p>
                <span className="badge">{t.theme || t.id}</span>
                {t.is_system ? <span className="badge">system</span> : null}
              </p>
              {t.allowed_blocks?.length > 0 && (
                <p className="muted small">
                  Blocks: {t.allowed_blocks.join(', ')}
                </p>
              )}
              <div className="row-actions">
                <button type="button" onClick={() => createFrom(t)}>
                  Use
                </button>
                <button
                  type="button"
                  className="secondary"
                  onClick={() => openEdit(t)}
                  disabled={!apiReady}
                >
                  Edit
                </button>
              </div>
            </article>
          ))}
        </div>
        {!loading && !templates.length && (
          <p className="muted">No templates yet.</p>
        )}
      </section>
    </div>
  )
}

function normalizeTemplate(raw) {
  const theme = raw.theme || raw.key || raw.id
  let default_blocks = raw.default_blocks
  if (typeof default_blocks === 'string') {
    try {
      default_blocks = JSON.parse(default_blocks)
    } catch {
      default_blocks = []
    }
  }
  if (!Array.isArray(default_blocks)) default_blocks = []
  return {
    id: raw.id || theme,
    theme,
    key: raw.key || theme,
    name: raw.name || raw.label || raw.title || theme,
    label: raw.label || raw.name || '',
    description: raw.description || '',
    allowed_blocks: Array.isArray(raw.allowed_blocks) ? raw.allowed_blocks : [],
    default_blocks,
    is_system: Boolean(raw.is_system),
    sort_order: raw.sort_order ?? 0,
  }
}

function slugify(s) {
  return (
    String(s)
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 64) || 'page'
  )
}
