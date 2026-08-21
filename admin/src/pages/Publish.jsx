import { useEffect, useState } from 'react'
import { admin } from '../api'
import { useToast } from '../toast'

export default function Publish() {
  const toast = useToast()
  const [history, setHistory] = useState([])
  const [status, setStatus] = useState('idle')
  const [lastResult, setLastResult] = useState(null)
  const [confirm, setConfirm] = useState(false)
  const [note, setNote] = useState('')
  const [pages, setPages] = useState([])
  const [settings, setSettings] = useState(null)

  async function loadHistory() {
    try {
      const data = await admin.publishHistory()
      setHistory(data.history || data.items || [])
    } catch (e) {
      toast.error(e.message)
      setHistory([])
    }
  }

  useEffect(() => {
    loadHistory()
    admin.pages.list().then((d) => setPages(d.pages || d.items || [])).catch(() => {})
    admin.settings.get().then((d) => setSettings(d.settings || d)).catch(() => {})
  }, [])

  async function runPublish() {
    if (!confirm) {
      toast.error('Confirm the checklist first')
      return
    }
    setStatus('publishing')
    try {
      const res = await admin.publish({ note })
      setLastResult(res)
      setStatus(res.status || 'success')
      toast.ok(res.message || 'Publish started')
      loadHistory()
    } catch (e) {
      setStatus('failed')
      setLastResult({ error: e.message })
      toast.error(e.message)
    }
  }

  const homepage = pages.find((p) => p.is_homepage)

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Publish</h1>
          <p className="muted">Generate static site and push to GitHub Pages</p>
        </div>
      </header>

      <section className="section">
        <h2>Summary</h2>
        <ul className="checklist">
          <li>Pages in CMS: <strong>{pages.length}</strong></li>
          <li>Homepage: <strong>{homepage?.title || homepage?.slug || 'not set'}</strong></li>
          <li>
            Site name:{' '}
            <strong>{settings?.site_name || '—'}</strong>
          </li>
          <li>
            Favicon:{' '}
            <strong>{settings?.favicon_media_id ? 'set' : 'missing'}</strong>
          </li>
        </ul>
        <label className="check">
          <input type="checkbox" checked={confirm} onChange={(e) => setConfirm(e.target.checked)} />
          I confirm SEO defaults, favicon, and homepage look correct
        </label>
        <label>
          Note (optional)
          <input value={note} onChange={(e) => setNote(e.target.value)} placeholder="What changed" />
        </label>
        <div className="bar">
          <button type="button" onClick={runPublish} disabled={status === 'publishing'}>
            {status === 'publishing' ? 'Publishing…' : 'Publish site'}
          </button>
          <span className={`badge status-${status}`}>{status}</span>
        </div>
        {lastResult && (
          <pre className="result-box">{JSON.stringify(lastResult, null, 2)}</pre>
        )}
      </section>

      <section className="section">
        <h2>History</h2>
        {!history.length && <p className="muted">No publish history yet (or endpoint unavailable).</p>}
        <ul className="list">
          {history.map((h) => (
            <li key={h.id || h.created_at}>
              <div>
                <strong>{h.status || 'ok'}</strong>{' '}
                <span className="muted">{h.created_at || ''}</span>
                {h.note ? <div>{h.note}</div> : null}
                {h.commit_sha ? <div className="muted">{h.commit_sha}</div> : null}
                {h.error ? <div className="error">{h.error}</div> : null}
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
