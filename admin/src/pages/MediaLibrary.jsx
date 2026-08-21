import { useEffect, useState } from 'react'
import { admin, apiUrl } from '../api'
import { mediaUrl } from '../blockTypes'
import { useToast } from '../toast'

export default function MediaLibrary() {
  const toast = useToast()
  const [items, setItems] = useState([])
  const [kind, setKind] = useState('')
  const [q, setQ] = useState('')
  const [file, setFile] = useState(null)
  const [title, setTitle] = useState('')
  const [uploadKind, setUploadKind] = useState('portfolio')
  const [loading, setLoading] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const data = await admin.media.list({ kind: kind || undefined, q: q || undefined })
      setItems(data.media || data.items || [])
    } catch (e) {
      setItems([])
      toast.error(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [kind])

  async function upload(e) {
    e.preventDefault()
    if (!file) {
      toast.error('Choose a file')
      return
    }
    const fd = new FormData()
    fd.append('file', file)
    fd.append('title', title)
    fd.append('kind', uploadKind)
    try {
      await admin.media.upload(fd)
      setFile(null)
      setTitle('')
      toast.ok('Uploaded')
      load()
    } catch (err) {
      toast.error(err.message)
    }
  }

  async function remove(id) {
    if (!confirm('Delete this media item?')) return
    try {
      await admin.media.remove(id)
      toast.ok('Deleted')
      load()
    } catch (e) {
      toast.error(e.message)
    }
  }

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Media</h1>
          <p className="muted">Upload and manage images for BA pairs and galleries</p>
        </div>
      </header>

      <section className="section">
        <h2>Upload</h2>
        <form className="bar" onSubmit={upload}>
          <label>
            File
            <input type="file" accept="image/*" onChange={(e) => setFile(e.target.files?.[0] || null)} />
          </label>
          <label>
            Title
            <input value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label>
            Kind
            <select value={uploadKind} onChange={(e) => setUploadKind(e.target.value)}>
              <option value="portfolio">portfolio</option>
              <option value="before">before</option>
              <option value="after">after</option>
              <option value="slide">slide</option>
              <option value="favicon">favicon</option>
            </select>
          </label>
          <button type="submit">Upload</button>
        </form>
      </section>

      <section className="section">
        <div className="bar">
          <label>
            Filter kind
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="">all</option>
              <option value="before">before</option>
              <option value="after">after</option>
              <option value="slide">slide</option>
              <option value="portfolio">portfolio</option>
              <option value="favicon">favicon</option>
            </select>
          </label>
          <label>
            Search
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="title" />
          </label>
          <button type="button" className="secondary" onClick={load}>
            Refresh
          </button>
        </div>
        <h2>
          Library {loading ? '…' : `(${items.length})`}
        </h2>
        <div className="grid">
          {items.map((m) => {
            const src = apiUrl(mediaUrl(m))
            return (
              <div key={m.id} className="card">
                {src ? <img src={src} alt={m.title || m.id} /> : <div className="thumb-empty">—</div>}
                <div className="meta">
                  {m.kind || '—'} · {m.title || m.id}
                </div>
                <button type="button" className="secondary danger small" onClick={() => remove(m.id)}>
                  Delete
                </button>
              </div>
            )
          })}
        </div>
        {!loading && !items.length && <p className="muted">No media yet.</p>}
      </section>
    </div>
  )
}
