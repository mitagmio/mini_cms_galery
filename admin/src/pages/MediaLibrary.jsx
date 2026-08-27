import { useEffect, useRef, useState } from 'react'
import { admin, apiUrl } from '../api'
import { mediaThumbUrl } from '../blockTypes'
import { invalidateMediaListCache } from '../components/MediaPicker'
import { useToast } from '../toast'

const UPLOAD_CONCURRENCY = 3

async function mapPool(items, concurrency, fn) {
  const results = new Array(items.length)
  let next = 0
  async function worker() {
    while (next < items.length) {
      const i = next++
      results[i] = await fn(items[i], i)
    }
  }
  const n = Math.min(concurrency, items.length)
  await Promise.all(Array.from({ length: n }, () => worker()))
  return results
}

export default function MediaLibrary() {
  const toast = useToast()
  const fileInputRef = useRef(null)
  const [items, setItems] = useState([])
  const [kind, setKind] = useState('')
  const [q, setQ] = useState('')
  const [files, setFiles] = useState([])
  const [title, setTitle] = useState('')
  const [uploadKind, setUploadKind] = useState('portfolio')
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(null)

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
    if (!files.length) {
      toast.error('Choose file(s)')
      return
    }
    setUploading(true)
    setProgress({ done: 0, total: files.length })
    let ok = 0
    let fail = 0
    const baseTitle = title.trim()
    try {
      await mapPool(files, UPLOAD_CONCURRENCY, async (file, i) => {
        const fd = new FormData()
        fd.append('file', file)
        const fileTitle =
          files.length === 1
            ? baseTitle || file.name
            : baseTitle
              ? `${baseTitle} (${i + 1})`
              : file.name
        fd.append('title', fileTitle)
        fd.append('kind', uploadKind)
        try {
          await admin.media.upload(fd)
          ok++
        } catch {
          fail++
        } finally {
          setProgress((p) => (p ? { ...p, done: p.done + 1 } : p))
        }
      })
      setFiles([])
      setTitle('')
      if (fileInputRef.current) fileInputRef.current.value = ''
      invalidateMediaListCache()
      if (fail === 0) {
        toast.ok(ok === 1 ? 'Uploaded' : `Uploaded ${ok}`)
      } else if (ok === 0) {
        toast.error(`Upload failed (${fail})`)
      } else {
        toast.error(`Uploaded ${ok}, failed ${fail}`)
      }
      load()
    } finally {
      setUploading(false)
      setProgress(null)
    }
  }

  async function remove(id) {
    if (!confirm('Delete this media item?')) return
    try {
      await admin.media.remove(id)
      invalidateMediaListCache()
      toast.ok('Deleted')
      load()
    } catch (e) {
      toast.error(e.message)
    }
  }

  const fileLabel =
    files.length === 0
      ? 'No files'
      : files.length === 1
        ? files[0].name
        : `${files.length} files`

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
            File(s)
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              multiple
              disabled={uploading}
              onChange={(e) => setFiles(Array.from(e.target.files || []))}
            />
          </label>
          <span className="muted">{fileLabel}</span>
          <label>
            Title
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              disabled={uploading}
              placeholder={files.length > 1 ? 'optional base title' : ''}
            />
          </label>
          <label>
            Kind
            <select
              value={uploadKind}
              onChange={(e) => setUploadKind(e.target.value)}
              disabled={uploading}
            >
              <option value="portfolio">portfolio</option>
              <option value="before">before</option>
              <option value="after">after</option>
              <option value="slide">slide</option>
              <option value="favicon">favicon</option>
            </select>
          </label>
          <button type="submit" disabled={uploading || !files.length}>
            {uploading && progress
              ? `Uploading ${progress.done}/${progress.total}`
              : files.length > 1
                ? `Upload ${files.length}`
                : 'Upload'}
          </button>
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
            const src = apiUrl(mediaThumbUrl(m))
            return (
              <div key={m.id} className="card">
                {src ? <img src={src} alt={m.title || m.id} loading="lazy" /> : <div className="thumb-empty">—</div>}
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
