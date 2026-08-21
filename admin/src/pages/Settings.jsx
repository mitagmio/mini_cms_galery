import { useEffect, useState } from 'react'
import { admin, apiUrl } from '../api'
import { mediaUrl } from '../blockTypes'
import MediaPicker from '../components/MediaPicker'
import { useToast } from '../toast'

const EMPTY = {
  site_name: '',
  canonical_base: 'https://www.sheyanova.art',
  domain: 'sheyanova.art',
  default_title_suffix: ' — Daria Sheyanova',
  default_description: '',
  robots: 'index,follow',
  favicon_media_id: null,
  og_image_media_id: null,
  social: {
    instagram: '',
    behance: '',
    linkedin: '',
  },
  nav_visible: true,
}

export default function Settings() {
  const toast = useToast()
  const [settings, setSettings] = useState(EMPTY)
  const [nav, setNav] = useState([])
  const [mediaMap, setMediaMap] = useState({})
  const [picker, setPicker] = useState(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const s = await admin.settings.get()
        if (!cancelled) {
          const raw = s.settings || s
          setSettings({
            ...EMPTY,
            ...raw,
            social: { ...EMPTY.social, ...(raw.social || {}) },
          })
        }
      } catch (e) {
        toast.error(e.message)
      }
      try {
        const n = await admin.nav.get()
        if (!cancelled) setNav(n.nav || n.items || n.tree || [])
      } catch (e) {
        /* optional */
      }
      try {
        const m = await admin.media.list()
        const map = {}
        for (const item of m.media || m.items || []) map[item.id] = item
        if (!cancelled) setMediaMap(map)
      } catch {
        /* ignore */
      }
    })()
    return () => {
      cancelled = true
    }
  }, [toast])

  function patch(field, value) {
    setSettings((prev) => ({ ...prev, [field]: value }))
  }

  function patchSocial(field, value) {
    setSettings((prev) => ({
      ...prev,
      social: { ...(prev.social || {}), [field]: value },
    }))
  }

  async function save(e) {
    e.preventDefault()
    setSaving(true)
    try {
      await admin.settings.put(settings)
      if (nav?.length) {
        try {
          await admin.nav.put({ nav })
        } catch (err) {
          toast.error(`Nav: ${err.message}`)
        }
      }
      toast.ok('Settings saved')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  function updateNavItem(idx, patchObj) {
    setNav((prev) => prev.map((item, i) => (i === idx ? { ...item, ...patchObj } : item)))
  }

  const fav = mediaMap[settings.favicon_media_id]
  const og = mediaMap[settings.og_image_media_id]

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Settings</h1>
          <p className="muted">SEO defaults, favicon, site name, social</p>
        </div>
      </header>

      <form className="section stack" onSubmit={save}>
        <h2>Brand &amp; SEO</h2>
        <label>
          Site name
          <input value={settings.site_name || ''} onChange={(e) => patch('site_name', e.target.value)} />
        </label>
        <label>
          Canonical base
          <input
            value={settings.canonical_base || ''}
            onChange={(e) => patch('canonical_base', e.target.value)}
          />
        </label>
        <label>
          Default title suffix
          <input
            value={settings.default_title_suffix || ''}
            onChange={(e) => patch('default_title_suffix', e.target.value)}
          />
        </label>
        <label>
          Default description
          <textarea
            rows={3}
            value={settings.default_description || ''}
            onChange={(e) => patch('default_description', e.target.value)}
          />
        </label>
        <label>
          Robots
          <input value={settings.robots || ''} onChange={(e) => patch('robots', e.target.value)} />
        </label>

        <div className="field-row">
          <span>Favicon</span>
          {fav ? (
            <img className="mini-thumb" src={apiUrl(mediaUrl(fav))} alt="favicon" />
          ) : (
            <span className="muted">none</span>
          )}
          <button
            type="button"
            className="secondary"
            onClick={() => setPicker({ field: 'favicon_media_id', kind: 'favicon', title: 'Pick favicon' })}
          >
            Pick from media
          </button>
        </div>
        <div className="field-row">
          <span>Default OG image</span>
          {og ? (
            <img className="mini-thumb" src={apiUrl(mediaUrl(og))} alt="og" />
          ) : (
            <span className="muted">none</span>
          )}
          <button
            type="button"
            className="secondary"
            onClick={() => setPicker({ field: 'og_image_media_id', title: 'Pick OG image' })}
          >
            Pick from media
          </button>
        </div>

        <h2>Social</h2>
        <label>
          Instagram
          <input
            value={settings.social?.instagram || ''}
            onChange={(e) => patchSocial('instagram', e.target.value)}
          />
        </label>
        <label>
          Behance
          <input
            value={settings.social?.behance || ''}
            onChange={(e) => patchSocial('behance', e.target.value)}
          />
        </label>
        <label>
          LinkedIn
          <input
            value={settings.social?.linkedin || ''}
            onChange={(e) => patchSocial('linkedin', e.target.value)}
          />
        </label>

        <h2>Nav visibility</h2>
        {!nav.length && (
          <p className="muted">
            Nav items come from <code>/api/admin/nav</code>. Empty until API returns a tree — page homepage /
            nav labels still work from the page editor.
          </p>
        )}
        <ul className="list">
          {nav.map((item, idx) => (
            <li key={item.id || idx}>
              <label className="check">
                <input
                  type="checkbox"
                  checked={item.visible !== false}
                  onChange={(e) => updateNavItem(idx, { visible: e.target.checked })}
                />
                {item.label || item.title || item.page_id || `Item ${idx + 1}`}
              </label>
            </li>
          ))}
        </ul>

        <button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save settings'}
        </button>
      </form>

      <MediaPicker
        open={Boolean(picker)}
        title={picker?.title || 'Pick media'}
        kindFilter={picker?.kind || ''}
        onClose={() => setPicker(null)}
        onSelect={(m) => {
          if (!picker) return
          patch(picker.field, m.id)
          setMediaMap((prev) => ({ ...prev, [m.id]: m }))
        }}
      />
    </div>
  )
}
