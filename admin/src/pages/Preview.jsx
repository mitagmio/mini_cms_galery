import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { admin } from '../api'
import { useToast } from '../toast'
import { usePreviewChrome } from '../components/Layout'

export default function Preview() {
  const toast = useToast()
  const { setPreviewChrome } = usePreviewChrome()
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

  function previewPathForPage(id, list) {
    const p = (list || pages).find((x) => String(x.id) === String(id))
    if (!p) return `/preview/?t=${Date.now()}`
    if (p.is_homepage) return `/preview/?t=${Date.now()}`
    const slug = String(p.slug || '').replace(/^\/+|\/+$/g, '')
    return slug ? `/preview/${slug}/?t=${Date.now()}` : `/preview/?t=${Date.now()}`
  }

  async function generateAndShow(explicitPageId) {
    const id = explicitPageId != null ? explicitPageId : pageId
    setBusy(true)
    try {
      await admin.generate({})
      setSrc(previewPathForPage(id))
      toast.ok('Preview refreshed')
    } catch (e) {
      toast.error(e.message)
      setSrc('/preview/')
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    if (!pages.length) return
    generateAndShow(pageId || pages[0]?.id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pages.length])

  useEffect(() => {
    setPreviewChrome(
      <div className="preview-chrome-bar">
        <label className="preview-chrome-label">
          Page
          <select
            value={pageId}
            onChange={(e) => {
              const id = e.target.value
              setPageId(id)
              if (id) setSrc(previewPathForPage(id))
              else setSrc(`/preview/?t=${Date.now()}`)
            }}
          >
            <option value="">(home)</option>
            {pages.map((p) => (
              <option key={p.id} value={p.id}>
                {p.title || p.slug}
              </option>
            ))}
          </select>
        </label>
        <button type="button" onClick={() => generateAndShow()} disabled={busy}>
          {busy ? 'Working…' : 'Generate'}
        </button>
        <Link className="button secondary" to="/publish">
          Publish
        </Link>
      </div>,
    )
    return () => setPreviewChrome(null)
  }, [pages, pageId, busy, setPreviewChrome])

  return (
    <div className="preview-fullscreen">
      <iframe className="preview-frame-full" title="site preview" src={src} />
    </div>
  )
}
