import { useEffect, useState } from 'react'
import { admin } from '../api'
import { useToast } from '../toast'

function stageLabel(job) {
  const stage = job?.stage || job?.history?.detail?.stage || ''
  switch (String(stage).toLowerCase()) {
    case 'queued':
      return 'Queued'
    case 'generate':
      return 'Generating site…'
    case 'push':
      return 'Pushing to GitHub…'
    case 'done':
      return 'Done'
    default:
      return stage || ''
  }
}

export default function Publish() {
  const toast = useToast()
  const [history, setHistory] = useState([])
  const [status, setStatus] = useState('idle')
  const [stage, setStage] = useState('')
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
    setStage('queued')
    setLastResult(null)
    try {
      const res = await admin.publish({ note })
      const jobId = res.job_id || res.history?.id
      setLastResult(res)
      setStage(res.stage || res.status || 'queued')
      toast.ok(res.message || 'Publish queued')
      if (!jobId) {
        setStatus(res.status || 'success')
        loadHistory()
        return
      }
      const final = await admin.waitPublishJob(jobId, {
        onUpdate: (job) => {
          setStage(stageLabel(job) || job.status)
          setLastResult(job)
          setStatus('publishing')
        },
      })
      setLastResult(final)
      const st = String(final.status || '').toLowerCase()
      if (st === 'ok' || st === 'stub') {
        setStatus('success')
        setStage(stageLabel(final) || 'Done')
        toast.ok(st === 'stub' ? 'Publish finished (git stubbed)' : 'Published')
      } else {
        setStatus('failed')
        setStage(stageLabel(final) || st)
        const err =
          final.history?.detail?.error ||
          final.job?.detail?.error ||
          final.error ||
          'Publish failed'
        toast.error(typeof err === 'string' ? err : 'Publish failed')
      }
      loadHistory()
    } catch (e) {
      setStatus('failed')
      setStage('')
      setLastResult({ error: e.message })
      toast.error(e.message)
    }
  }

  const homepage = pages.find((p) => p.is_homepage)
  const busy = status === 'publishing'

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Publish</h1>
          <p className="muted">Generate full static site and push to GitHub Pages (async job)</p>
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
          <button type="button" onClick={runPublish} disabled={busy}>
            {busy ? 'Publishing…' : 'Publish site'}
          </button>
          <span className={`badge status-${status}`}>{status}</span>
          {busy && stage ? <span className="muted">{stage}</span> : null}
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
                {h.detail?.stage ? <div className="muted">stage: {h.detail.stage}</div> : null}
                {h.commit_sha ? <div className="muted">{h.commit_sha}</div> : null}
                {h.error || h.detail?.error ? (
                  <div className="error">{h.error || h.detail.error}</div>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
