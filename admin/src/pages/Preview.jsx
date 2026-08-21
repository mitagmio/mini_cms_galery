import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { admin, apiUrl } from '../api'
import { useToast } from '../toast'

export default function Preview() {
  const toast = useToast()
  const [pages, setPages] = useState([])
  const [pageId, setPageId] = useState('')
  const [src, setSrc] = useState('/preview/')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const data = await admin.pages.list()
        if (cancelled) return
        const list = data.pages || data.items || []
        setPages(list)
        const home = list.find((p) => p.is_homepage) || list[0]
        if (home) setPageId(String(home.id))
      } catch (e) {
        toast.error(e.message)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [toast])

  async function generateAndShow() {
    setBusy(true)
    try {
      if (pageId) {
        try {
          const res = await admin.preview(pageId)
          const url = res.preview_url || res.url
          if (url) {
            setSrc(apiUrl(url))
            toast.ok('Preview ready')
            return
          }
        } catch (e) {
          toast.error(e.message)
        }
      }
      await admin.generate(pageId ? { page_id: pageId } : {})
      setSrc(`/preview/?t=${Date.now()}`)
      toast.ok('Draft site generated → /preview/')
    } catch (e) {
      toast.error(e.message)
      setSrc('/preview/')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page preview-page">
      <header className="page-head">
        <div>
          <h1>Site preview</h1>
          <p className="muted">Desktop iframe of generated draft at /preview/</p>
        </div>
        <div className="bar">
          <label>
            Page
            <select value={pageId} onChange={(e) => setPageId(e.target.value)}>
              <option value="">(whole site)</option>
              {pages.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.title || p.slug}
                </option>
              ))}
            </select>
          </label>
          <button type="button" onClick={generateAndShow} disabled={busy}>
            {busy ? 'Working…' : 'Generate & refresh'}
          </button>
          <Link className="button secondary" to="/publish">
            Publish
          </Link>
        </div>
      </header>

      <div className="preview-frame-wrap">
        <iframe className="preview-frame site" title="site preview" src={src} />
      </div>
    </div>
  )
}
