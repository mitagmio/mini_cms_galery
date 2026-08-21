import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { admin } from '../api'
import { useToast } from '../toast'

export default function Dashboard() {
  const toast = useToast()
  const [pages, setPages] = useState([])
  const [media, setMedia] = useState([])
  const [history, setHistory] = useState([])
  const [settings, setSettings] = useState(null)
  const [forceImport, setForceImport] = useState(false)
  const [importing, setImporting] = useState(false)

  async function load() {
    const tasks = [
      admin.pages.list().catch((e) => {
        toast.error(e.message)
        return null
      }),
      admin.media.list().catch(() => null),
      admin.publishHistory().catch(() => null),
      admin.settings.get().catch(() => null),
    ]
    const [p, m, h, s] = await Promise.all(tasks)
    setPages(p?.pages || p?.items || [])
    setMedia(m?.media || m?.items || [])
    setHistory(h?.history || h?.items || [])
    setSettings(s?.settings || s || null)
  }

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      const tasks = [
        admin.pages.list().catch((e) => {
          if (!cancelled) toast.error(e.message)
          return null
        }),
        admin.media.list().catch(() => null),
        admin.publishHistory().catch(() => null),
        admin.settings.get().catch(() => null),
      ]
      const [p, m, h, s] = await Promise.all(tasks)
      if (cancelled) return
      setPages(p?.pages || p?.items || [])
      setMedia(m?.media || m?.items || [])
      setHistory(h?.history || h?.items || [])
      setSettings(s?.settings || s || null)
    })()
    return () => {
      cancelled = true
    }
  }, [toast])

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

  const last = history[0]
  const homepage = pages.find((p) => p.is_homepage)

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Dashboard</h1>
          <p className="muted">Format-like CMS for sheyanova.art</p>
        </div>
        <div className="bar">
          <Link className="button" to="/pages">
            Pages
          </Link>
          <Link className="button secondary" to="/publish">
            Publish
          </Link>
        </div>
      </header>

      <div className="stat-row">
        <div className="stat">
          <span className="stat-label">Pages</span>
          <strong>{pages.length}</strong>
        </div>
        <div className="stat">
          <span className="stat-label">Media</span>
          <strong>{media.length}</strong>
        </div>
        <div className="stat">
          <span className="stat-label">Homepage</span>
          <strong>{homepage?.title || homepage?.slug || '—'}</strong>
        </div>
        <div className="stat">
          <span className="stat-label">Last publish</span>
          <strong>{last?.created_at || last?.status || 'never'}</strong>
        </div>
      </div>

      <section className="section">
        <h2>Site</h2>
        <p>
          {settings?.site_name || 'Site name not set'} ·{' '}
          <span className="muted">{settings?.canonical_base || settings?.domain || 'configure in Settings'}</span>
        </p>
        <div className="quick-links">
          <Link to="/media">Media library</Link>
          <Link to="/settings">SEO &amp; branding</Link>
          <Link to="/templates">Templates</Link>
          <Link to="/preview">Preview site</Link>
        </div>
      </section>

      <section className="section">
        <h2>Import from front</h2>
        <p className="muted">
          Pull page blocks and media from the static <code>front/</code> tree into the CMS. Without force,
          pages that already have blocks are skipped.
        </p>
        <label className="check">
          <input
            type="checkbox"
            checked={forceImport}
            onChange={(e) => setForceImport(e.target.checked)}
          />
          Force overwrite existing blocks (<code>?force=1</code>)
        </label>
        <div className="bar tight">
          <button type="button" onClick={runImport} disabled={importing}>
            {importing ? 'Importing…' : 'Import from front'}
          </button>
        </div>
      </section>

      <section className="section">
        <h2>Recent pages</h2>
        {!pages.length && <p className="muted">No pages yet — create one from Pages or Templates.</p>}
        <ul className="list">
          {pages.slice(0, 8).map((p) => (
            <li key={p.id}>
              <Link to={`/pages/${p.id}`}>
                {p.title || p.slug} <span className="muted">/{p.slug}</span>
              </Link>
              <span className="badge">{p.template || p.theme || '—'}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
