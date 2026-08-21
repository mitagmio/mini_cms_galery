import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { admin } from '../api'
import { TEMPLATES, templateLabel } from '../blockTypes'
import { useToast } from '../toast'

export default function PagesList() {
  const toast = useToast()
  const navigate = useNavigate()
  const [pages, setPages] = useState([])
  const [loading, setLoading] = useState(true)
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [template, setTemplate] = useState('ba_content')
  const [forceImport, setForceImport] = useState(false)
  const [importing, setImporting] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const data = await admin.pages.list()
      setPages(data.pages || data.items || [])
    } catch (e) {
      setPages([])
      toast.error(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function create(e) {
    e.preventDefault()
    try {
      const data = await admin.pages.create({
        title: title.trim() || 'Untitled',
        slug: slug.trim() || slugify(title || 'page'),
        template,
        is_homepage: false,
        seo: {},
      })
      const page = data.page || data
      toast.ok('Page created')
      navigate(`/pages/${page.id}`)
    } catch (err) {
      toast.error(err.message)
    }
  }

  async function remove(id) {
    if (!confirm('Delete this page?')) return
    try {
      await admin.pages.remove(id)
      toast.ok('Deleted')
      load()
    } catch (e) {
      toast.error(e.message)
    }
  }

  async function runImport() {
    if (forceImport && !confirm('Force import will replace existing page blocks. Continue?')) {
      return
    }
    setImporting(true)
    try {
      const data = await admin.importFront(forceImport)
      const imp = data.import || data
      const updated = imp.pages_updated ?? 0
      const skipped = imp.pages_skipped ?? 0
      const mediaCreated = imp.media_created ?? 0
      toast.ok(
        `Import done: ${updated} page(s) updated, ${skipped} skipped, ${mediaCreated} media created`
      )
      await load()
    } catch (e) {
      toast.error(e.message)
    } finally {
      setImporting(false)
    }
  }

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Pages</h1>
          <p className="muted">Create and edit site pages</p>
        </div>
        <div className="bar tight">
          <label className="check">
            <input
              type="checkbox"
              checked={forceImport}
              onChange={(e) => setForceImport(e.target.checked)}
            />
            Force
          </label>
          <button type="button" className="secondary" onClick={runImport} disabled={importing}>
            {importing ? 'Importing…' : 'Import from front'}
          </button>
        </div>
      </header>

      <section className="section">
        <h2>Create page</h2>
        <form className="bar" onSubmit={create}>
          <label>
            Title
            <input value={title} onChange={(e) => setTitle(e.target.value)} required />
          </label>
          <label>
            Slug
            <input
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="auto from title"
            />
          </label>
          <label>
            Template
            <select value={template} onChange={(e) => setTemplate(e.target.value)}>
              {TEMPLATES.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.label}
                </option>
              ))}
            </select>
          </label>
          <button type="submit">Create</button>
        </form>
      </section>

      <section className="section">
        <h2>All pages {loading ? '…' : `(${pages.length})`}</h2>
        <ul className="list">
          {pages.map((p) => (
            <li key={p.id}>
              <div>
                <Link to={`/pages/${p.id}`}>
                  <strong>{p.title || 'Untitled'}</strong>
                </Link>
                <div className="muted">
                  /{p.slug} · {templateLabel(p.template || p.theme)}
                  {p.is_homepage ? ' · homepage' : ''}
                </div>
              </div>
              <div className="row-actions">
                <Link className="button secondary" to={`/pages/${p.id}`}>
                  Edit
                </Link>
                <button type="button" className="secondary danger" onClick={() => remove(p.id)}>
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
        {!loading && !pages.length && <p className="muted">No pages yet.</p>}
      </section>
    </div>
  )
}

function slugify(s) {
  return String(s)
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 64) || 'page'
}
