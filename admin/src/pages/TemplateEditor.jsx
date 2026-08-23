import { useEffect, useMemo, useState } from 'react'
import { admin } from '../api'
import {
  ALLOWED_BLOCKS_BY_THEME,
  BLOCK_PALETTE,
  FORM_TEMPLATES,
  RATE_FORM_KEYS,
  bannerFormKey,
  formKeyFromTemplateId,
  formTemplateId,
  formTemplateName,
  paletteForAllowed,
  newBlock,
  rateBannerData,
  RATE_CAPTIONS,
} from '../blockTypes'
import {
  FORM_BLOCK_PALETTE,
  formBlockLabel,
  newFormBlock,
  normalizeCanvasBlocks,
  optionList,
  serializeCanvasBlocks,
  showIfToString,
} from '../formBlockTypes'
import MediaPicker from '../components/MediaPicker'
import { useToast } from '../toast'

export default function TemplateEditor({ tmpl, apiReady, onCancel, onSaved }) {
  const toast = useToast()
  const isForm = tmpl.kind === 'form'
  const [name, setName] = useState(tmpl.name || '')
  const [description, setDescription] = useState(tmpl.description || '')
  const [theme] = useState(tmpl.theme || tmpl.key || 'ba_content')
  const [allowed, setAllowed] = useState(
    Array.isArray(tmpl.allowed_blocks) && tmpl.allowed_blocks.length
      ? [...tmpl.allowed_blocks]
      : [...(ALLOWED_BLOCKS_BY_THEME[tmpl.theme] || [])]
  )
  const [blocks, setBlocks] = useState(() => normalizeCanvasBlocks(tmpl.default_blocks))
  const [selectedId, setSelectedId] = useState(null)
  const [tab, setTab] = useState('canvas')
  const [source, setSource] = useState(tmpl.source || '')
  const [fileSource, setFileSource] = useState(tmpl.file_source || '')
  const [saving, setSaving] = useState(false)
  const [formTemplates, setFormTemplates] = useState(FORM_TEMPLATES)
  const [picker, setPicker] = useState(null)
  const [mediaIndex, setMediaIndex] = useState({})

  const selected = useMemo(
    () => blocks.find((b) => b.id === selectedId) || null,
    [blocks, selectedId]
  )

  const palette = useMemo(() => {
    if (isForm) return FORM_BLOCK_PALETTE
    if (!allowed.length) {
      return paletteForAllowed(ALLOWED_BLOCKS_BY_THEME[theme] || ALLOWED_BLOCKS_BY_THEME.text_content)
    }
    return paletteForAllowed(allowed)
  }, [isForm, allowed, theme])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const data = await admin.templates.get(tmpl.id)
        const full = data.template || data
        if (cancelled || !full) return
        setName(full.name || tmpl.name || '')
        setDescription(full.description || '')
        const nextBlocks = normalizeCanvasBlocks(full.default_blocks || full.fields)
        setBlocks(nextBlocks)
        setSelectedId(nextBlocks[0]?.id || null)
        if (Array.isArray(full.allowed_blocks) && full.allowed_blocks.length) {
          setAllowed([...full.allowed_blocks])
        } else if (ALLOWED_BLOCKS_BY_THEME[full.theme || theme]) {
          setAllowed([...(ALLOWED_BLOCKS_BY_THEME[full.theme || theme] || [])])
        }
        const file = full.file_source || ''
        const stored = full.source || ''
        setFileSource(file)
        setSource(stored || file)
      } catch {
        setSelectedId((prev) => prev || blocks[0]?.id || null)
      }
      try {
        const tdata = await admin.templates.list()
        const list = (tdata.templates || tdata.items || []).filter((t) => t.kind === 'form')
        if (list.length) {
          setFormTemplates(
            list.map((t) => ({
              id: t.id,
              name: t.name || t.label,
              form_key: t.form_key || formKeyFromTemplateId(t.id),
              kind: 'form',
            }))
          )
        }
      } catch {
        /* optional */
      }
      try {
        const mdata = await admin.media.list()
        const list = mdata.media || mdata.items || []
        const map = {}
        for (const m of list) map[m.id] = m
        if (!cancelled) setMediaIndex(map)
      } catch {
        /* optional */
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tmpl.id])

  function addBlock(type) {
    const b = isForm ? newFormBlock(type) : newBlock(type)
    if (!isForm && type === 'rate_banner') {
      const used = new Set(blocks.filter((x) => x.type === 'rate_banner').map((x) => bannerFormKey(x.data)))
      const next = RATE_FORM_KEYS.find((k) => !used.has(k))
      if (!next) {
        toast.error('Each category can appear only once')
        return
      }
      b.data = rateBannerData(next)
    }
    setBlocks((prev) => [...prev, b])
    setSelectedId(b.id)
    setTab('canvas')
  }

  function moveBlock(id, dir) {
    setBlocks((prev) => {
      const i = prev.findIndex((b) => b.id === id)
      const j = i + dir
      if (i < 0 || j < 0 || j >= prev.length) return prev
      const next = [...prev]
      ;[next[i], next[j]] = [next[j], next[i]]
      return next
    })
  }

  function removeBlock(id) {
    setBlocks((prev) => prev.filter((b) => b.id !== id))
    if (selectedId === id) setSelectedId(null)
  }

  function updateBlockData(id, patch) {
    setBlocks((prev) =>
      prev.map((b) => (b.id === id ? { ...b, data: { ...b.data, ...patch } } : b))
    )
  }

  function toggleAllowed(type) {
    setAllowed((prev) =>
      prev.includes(type) ? prev.filter((t) => t !== type) : [...prev, type]
    )
  }

  async function save() {
    if (!apiReady) {
      toast.error('Templates API is not available yet')
      return
    }
    const trimmed = name.trim()
    if (!trimmed) {
      toast.error('Name is required')
      return
    }
    const default_blocks = serializeCanvasBlocks(blocks)
    const body = {
      name: trimmed,
      label: trimmed,
      description: description.trim(),
      default_blocks,
    }
    if (isForm) {
      body.kind = 'form'
      body.fields = default_blocks
    } else {
      body.kind = 'page'
      body.theme = theme
      body.key = theme
      body.allowed_blocks = allowed
      body.source = source === fileSource ? '' : source
    }
    setSaving(true)
    try {
      const res = await admin.templates.patch(tmpl.id, body)
      if (res.generate_error) {
        toast.error(`Saved, but preview failed: ${res.generate_error}`)
      } else {
        toast.ok('Template saved')
      }
      onSaved(res.template || res)
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="editor-page template-editor">
      <div className="editor-toolbar">
        <div className="editor-toolbar-left">
          <button type="button" className="secondary" onClick={onCancel}>
            ← Templates
          </button>
          <input
            className="title-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Template name"
          />
          <span className="badge">{isForm ? 'Form' : 'Page'}</span>
          {tmpl.is_system ? <span className="badge">system</span> : null}
          {isForm && tmpl.form_key ? (
            <span className="badge">{tmpl.form_key}</span>
          ) : (
            <span className="badge">{theme}</span>
          )}
        </div>
        <div className="bar tight">
          <button type="button" onClick={save} disabled={saving || !apiReady}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>

      {!isForm && (
        <div className="tabs template-tabs">
          <button type="button" className={tab === 'canvas' ? 'active' : ''} onClick={() => setTab('canvas')}>
            Canvas
          </button>
          <button type="button" className={tab === 'code' ? 'active' : ''} onClick={() => setTab('code')}>
            Code
          </button>
          <button type="button" className={tab === 'settings' ? 'active' : ''} onClick={() => setTab('settings')}>
            Settings
          </button>
        </div>
      )}

      {isForm && (
        <p className="muted template-editor-lead">
          Edit form steps as blocks. Labels and options save with the template; rates preview regenerates on Save.
          {tmpl.form_key ? (
            <>
              {' '}
              POST <code>form=rates_{tmpl.form_key}</code>
            </>
          ) : null}
        </p>
      )}

      {(isForm || tab === 'canvas') && (
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
            {!blocks.length && <p className="empty-canvas">Add blocks from the palette.</p>}
            {blocks.map((b, idx) => (
              <div
                key={b.id}
                className={`block-card ${selectedId === b.id ? 'selected' : ''}`}
                onClick={() => setSelectedId(b.id)}
              >
                <div className="block-card-head">
                  <strong>{isForm ? formBlockLabel(b.type) : labelForPageType(b.type)}</strong>
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
                <CanvasPreview block={b} isForm={isForm} formTemplates={formTemplates} />
              </div>
            ))}
          </section>
          <aside className="inspector">
            {isForm && (
              <div className="inspector-body" style={{ marginBottom: '1rem' }}>
                <label>
                  Description
                  <textarea rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
                </label>
              </div>
            )}
            {selected ? (
              isForm ? (
                <FormFieldInspector
                  block={selected}
                  onChange={(patch) => updateBlockData(selected.id, patch)}
                  onDelete={() => removeBlock(selected.id)}
                />
              ) : (
                <PageBlockInspector
                  block={selected}
                  mediaIndex={mediaIndex}
                  formTemplates={formTemplates}
                  usedFormKeys={new Set(blocks.filter((x) => x.type === 'rate_banner').map((x) => bannerFormKey(x.data)))}
                  onChange={(patch) => updateBlockData(selected.id, patch)}
                  onDelete={() => removeBlock(selected.id)}
                  onPick={(spec) => setPicker({ ...spec, blockId: selected.id })}
                />
              )
            ) : (
              <p className="muted inspector-body">Select a block to edit labels, options, and POST keys.</p>
            )}
          </aside>
        </div>
      )}

      {!isForm && tab === 'code' && (
        <div className="template-code-wrap">
          <p className="muted">
            Generate template (<code>.gohtml</code>) for engine <code>{theme}</code>. Must keep{' '}
            <code>{`{{define "${theme}"}}`}</code>. If this matches the file, the built-in template is used.
            A bad override falls back to the embedded file when generating.
          </p>
          <textarea
            className="template-code"
            spellCheck={false}
            value={source}
            onChange={(e) => setSource(e.target.value)}
          />
        </div>
      )}

      {!isForm && tab === 'settings' && (
        <section className="section template-settings">
          <label>
            Description
            <textarea rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
          </label>
          <p className="muted">
            Id <code>{tmpl.id}</code> and engine <code>{theme}</code> stay fixed on system templates.
          </p>
          <fieldset className="block-checkboxes">
            <legend>Allowed blocks</legend>
            <div className="bar tight">
              {BLOCK_PALETTE.map((b) => (
                <label key={b.type} className="check">
                  <input
                    type="checkbox"
                    checked={allowed.includes(b.type)}
                    onChange={() => toggleAllowed(b.type)}
                  />
                  {b.label}
                </label>
              ))}
            </div>
          </fieldset>
        </section>
      )}

      {picker && (
        <MediaPicker
          open
          title={picker.title || 'Pick image'}
          kindFilter={picker.kind || ''}
          onClose={() => setPicker(null)}
          onSelect={(item) => {
            if (picker.blockId && picker.field) {
              updateBlockData(picker.blockId, { [picker.field]: item.id })
            }
            setPicker(null)
            setMediaIndex((prev) => ({ ...prev, [item.id]: item }))
          }}
        />
      )}
    </div>
  )
}

function labelForPageType(type) {
  return BLOCK_PALETTE.find((b) => b.type === type)?.label || type
}

function CanvasPreview({ block, isForm, formTemplates }) {
  const d = block.data || {}
  if (isForm) {
    if (block.type === 'form_step') return <div className="muted">{d.title || d.label || 'Step'}</div>
    if (block.type === 'form_honeypot') return <div className="muted">{d.note || 'Hidden spam trap'}</div>
    if (block.type === 'form_help') return <div className="muted">{d.text || d.html || 'Help text'}</div>
    if (block.type === 'form_contact_footer') {
      return <div className="muted">{d.legend || 'Contact'} · name / email / phone</div>
    }
    const opts = optionList(d)
    return (
      <div className="muted">
        {d.label || d.title || block.type}
        {d.name ? ` · ${d.name}` : ''}
        {d.required ? ' · required' : ''}
        {opts.length ? ` · ${opts.length} options` : ''}
      </div>
    )
  }
  if (block.type === 'rate_banner') {
    return (
      <div className="muted">
        {d.caption || RATE_CAPTIONS[bannerFormKey(d)] || 'Rate banner'} ·{' '}
        {formTemplateName(bannerFormKey(d), formTemplates)}
      </div>
    )
  }
  if (block.type === 'rich_text') {
    const text = String(d.html || '')
      .replace(/<[^>]+>/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
    return <div className="muted">{text.slice(0, 120) || 'Rich text'}</div>
  }
  if (block.type === 'gallery_image') {
    return <div className="muted">{d.alt || d.caption || 'Gallery image'}</div>
  }
  return <div className="muted">{block.type}</div>
}

function FormFieldInspector({ block, onChange, onDelete }) {
  const d = block.data || {}
  const opts = optionList(d)
  const hasOptions = ['form_select', 'form_radio', 'form_checkbox', 'form_retouch_level'].includes(block.type)
  const hasName = !['form_step', 'form_help', 'form_honeypot', 'form_contact_footer'].includes(block.type)

  function setOpt(i, patch) {
    onChange({ options: opts.map((o, idx) => (idx === i ? { ...o, ...patch } : o)) })
  }

  if (block.type === 'form_step') {
    return (
      <div className="inspector-body">
        <label>
          Step title
          <input value={d.title || ''} onChange={(e) => onChange({ title: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  if (block.type === 'form_help') {
    return (
      <div className="inspector-body">
        <label>
          Help text
          <textarea rows={4} value={d.text || ''} onChange={(e) => onChange({ text: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  if (block.type === 'form_honeypot') {
    return (
      <div className="inspector-body">
        <p className="muted">{d.note || 'System spam trap. Not shown as a public field.'}</p>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  if (block.type === 'form_contact_footer') {
    return (
      <div className="inspector-body">
        <label>
          Legend
          <input value={d.legend || ''} onChange={(e) => onChange({ legend: e.target.value })} />
        </label>
        <label>
          Name label
          <input value={d.name_label || ''} onChange={(e) => onChange({ name_label: e.target.value })} />
        </label>
        <label>
          Contact label
          <input value={d.contact_label || ''} onChange={(e) => onChange({ contact_label: e.target.value })} />
        </label>
        <label>
          Email label
          <input value={d.email_label || ''} onChange={(e) => onChange({ email_label: e.target.value })} />
        </label>
        <label>
          Phone label
          <input value={d.phone_label || ''} onChange={(e) => onChange({ phone_label: e.target.value })} />
        </label>
        <label>
          Submit label
          <input value={d.submit_label || ''} onChange={(e) => onChange({ submit_label: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }

  return (
    <div className="inspector-body">
      {hasName && (
        <label>
          POST key (<code>name</code>)
          <input value={d.name || ''} onChange={(e) => onChange({ name: e.target.value })} />
        </label>
      )}
      <label>
        Label
        <input value={d.label || ''} onChange={(e) => onChange({ label: e.target.value })} />
      </label>
      {['form_text', 'form_textarea', 'form_date'].includes(block.type) && (
        <label>
          Placeholder
          <input value={d.placeholder || ''} onChange={(e) => onChange({ placeholder: e.target.value })} />
        </label>
      )}
      {(block.type === 'form_retouch_level' || block.type === 'form_checkbox' || block.type === 'form_radio') && (
        <label>
          Help
          <textarea rows={2} value={d.help || ''} onChange={(e) => onChange({ help: e.target.value })} />
        </label>
      )}
      {hasName && (
        <label className="check">
          <input
            type="checkbox"
            checked={Boolean(d.required)}
            onChange={(e) => onChange({ required: e.target.checked })}
          />
          Required
        </label>
      )}
      <label>
        Show if (e.g. <code>task=color,hair</code>)
        <input
          value={showIfToString(d)}
          onChange={(e) => onChange({ show_if: e.target.value })}
          placeholder="task=cut_model"
        />
      </label>
      {block.type === 'form_select' && (
        <label>
          Format mode
          <select value={d.format_mode || ''} onChange={(e) => onChange({ format_mode: e.target.value })}>
            <option value="">—</option>
            <option value="std">std</option>
            <option value="cut">cut</option>
          </select>
        </label>
      )}
      {hasOptions && (
        <div>
          <p className="muted" style={{ marginBottom: '0.4rem' }}>
            Options
          </p>
          {opts.map((o, i) => (
            <div key={i} className="option-row">
              <input
                value={o.label}
                placeholder="Label"
                onChange={(e) => {
                  const label = e.target.value
                  const value = o.value === o.label ? label : o.value
                  setOpt(i, { label, value })
                }}
              />
              <input
                value={o.value}
                placeholder="Value"
                onChange={(e) => setOpt(i, { value: e.target.value })}
              />
              {block.type === 'form_retouch_level' && (
                <>
                  <input
                    value={o.image || ''}
                    placeholder="Image URL"
                    onChange={(e) => setOpt(i, { image: e.target.value })}
                  />
                  <input
                    value={o.plaque || ''}
                    placeholder="Plaque"
                    onChange={(e) => setOpt(i, { plaque: e.target.value })}
                  />
                </>
              )}
              <button
                type="button"
                className="icon-btn danger"
                onClick={() => onChange({ options: opts.filter((_, idx) => idx !== i) })}
              >
                ×
              </button>
            </div>
          ))}
          <button
            type="button"
            className="secondary"
            onClick={() => onChange({ options: [...opts, { value: 'New option', label: 'New option' }] })}
          >
            Add option
          </button>
        </div>
      )}
      <button type="button" className="secondary danger" onClick={onDelete}>
        Delete block
      </button>
    </div>
  )
}

function PageBlockInspector({
  block,
  formTemplates,
  usedFormKeys,
  onChange,
  onDelete,
  onPick,
}) {
  const d = block.data || {}
  if (block.type === 'rich_text') {
    return (
      <div className="inspector-body">
        <label>
          HTML
          <textarea rows={10} value={d.html || ''} onChange={(e) => onChange({ html: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  if (block.type === 'gallery_image' || block.type === 'comparison_slider') {
    const fields =
      block.type === 'gallery_image'
        ? [{ field: 'media_id', label: 'Image' }]
        : [
            { field: 'before_media_id', label: 'Before', kind: 'before' },
            { field: 'after_media_id', label: 'After', kind: 'after' },
          ]
    return (
      <div className="inspector-body">
        {fields.map((f) => (
          <div key={f.field} className="field-row">
            <span>{f.label}</span>
            <button
              type="button"
              className="secondary"
              onClick={() => onPick({ field: f.field, kind: f.kind, title: `Pick ${f.label}` })}
            >
              Pick
            </button>
          </div>
        ))}
        <label>
          Caption
          <input
            value={d.caption || d.alt || ''}
            onChange={(e) =>
              onChange(
                block.type === 'gallery_image'
                  ? { alt: e.target.value, caption: e.target.value }
                  : { caption: e.target.value }
              )
            }
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
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  if (block.type === 'rate_banner') {
    const currentKey = bannerFormKey(d)
    const currentId = d.form_template_id || formTemplateId(currentKey)
    return (
      <div className="inspector-body">
        <label>
          Form template
          <select
            value={currentId}
            onChange={(e) => {
              const id = e.target.value
              const key = formKeyFromTemplateId(id)
              if (usedFormKeys.has(key) && id !== currentId) return
              const patch = { form_template_id: id, form_key: key }
              if (!d.caption || d.caption === RATE_CAPTIONS[currentKey]) {
                patch.caption = RATE_CAPTIONS[key] || d.caption
              }
              onChange(patch)
            }}
          >
            {formTemplates.map((t) => {
              const key = t.form_key || formKeyFromTemplateId(t.id)
              const taken = usedFormKeys.has(key) && t.id !== currentId
              return (
                <option key={t.id} value={t.id} disabled={taken}>
                  {t.name || t.label || key}
                </option>
              )
            })}
          </select>
        </label>
        <div className="field-row">
          <span>Image</span>
          <button type="button" className="secondary" onClick={() => onPick({ field: 'media_id', title: 'Pick banner image' })}>
            Pick
          </button>
        </div>
        <label>
          Caption
          <input value={d.caption || ''} onChange={(e) => onChange({ caption: e.target.value })} />
        </label>
        <label>
          Price
          <input value={d.price || ''} onChange={(e) => onChange({ price: e.target.value })} />
        </label>
        <label>
          Currency
          <input value={d.currency || ''} onChange={(e) => onChange({ currency: e.target.value })} />
        </label>
        <button type="button" className="secondary danger" onClick={onDelete}>
          Delete block
        </button>
      </div>
    )
  }
  return (
    <div className="inspector-body">
      <p className="muted">{block.type}</p>
      <button type="button" className="secondary danger" onClick={onDelete}>
        Delete block
      </button>
    </div>
  )
}
