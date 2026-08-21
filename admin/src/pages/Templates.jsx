import { useNavigate } from 'react-router-dom'
import { admin } from '../api'
import { TEMPLATES, newBlock } from '../blockTypes'
import { useToast } from '../toast'

const STARTERS = [
  {
    ...TEMPLATES[0],
    name: 'BA page',
    blocks: () => [
      newBlock('comparison_slider'),
      newBlock('comparison_slider'),
    ],
  },
  {
    ...TEMPLATES[1],
    name: 'Gallery',
    blocks: () => [newBlock('gallery_image'), newBlock('gallery_image'), newBlock('gallery_image')],
  },
  {
    ...TEMPLATES[2],
    name: 'Blank / text',
    blocks: () => [newBlock('rich_text')],
  },
]

export default function Templates() {
  const toast = useToast()
  const navigate = useNavigate()

  async function createFrom(starter) {
    const title = starter.name
    const slug = slugify(title)
    try {
      const data = await admin.pages.create({
        title,
        slug,
        template: starter.id,
        is_homepage: false,
        seo: {},
      })
      const page = data.page || data
      const blocks = starter.blocks().map((b, i) => ({
        type: b.type,
        position: i,
        data: b.data,
      }))
      try {
        await admin.pages.putBlocks(page.id, blocks)
      } catch (e) {
        toast.error(`Page created; blocks: ${e.message}`)
        navigate(`/pages/${page.id}`)
        return
      }
      toast.ok(`Created “${title}”`)
      navigate(`/pages/${page.id}`)
    } catch (e) {
      toast.error(e.message)
    }
  }

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Templates</h1>
          <p className="muted">Starters for BA, gallery, and text pages</p>
        </div>
      </header>

      <div className="template-grid">
        {STARTERS.map((s) => (
          <article key={s.id} className="template-card">
            <h2>{s.name}</h2>
            <p className="muted">{s.description}</p>
            <p>
              <span className="badge">{s.id}</span>
            </p>
            <button type="button" onClick={() => createFrom(s)}>
              Use template
            </button>
          </article>
        ))}
      </div>
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
