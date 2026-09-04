import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { admin, apiUrl } from '../api'
import { mediaThumbUrl } from '../blockTypes'
import MediaPicker from '../components/MediaPicker'
import { useToast } from '../toast'

const EMPTY = {
  site_name: '',
  canonical_base: 'https://www.sheyanova.art',
  domain: 'sheyanova.art',
  default_title_suffix: ' — Daria Sheyanova',
  default_description: '',
  robots: 'noai, noimageai',
  favicon_media_id: null,
  og_image_media_id: null,
  contact_email: '',
  yandex_metrika_enabled: true,
  yandex_metrika_id: '95095785',
  gtm_enabled: true,
  gtm_container_id: 'GTM-K5DWKFDZ',
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
  const [mediaMap, setMediaMap] = useState({})
  const [picker, setPicker] = useState(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      let nextSettings = null
      try {
        const s = await admin.settings.get()
        if (!cancelled) {
          const raw = s.settings || s
          nextSettings = {
            ...EMPTY,
            ...raw,
            contact_email: raw.contact_email || raw.mailto_address || '',
            social: { ...EMPTY.social, ...(raw.social || {}) },
          }
          setSettings(nextSettings)
        }
      } catch (e) {
        toast.error(e.message)
      }
      if (cancelled || !nextSettings) return
      const ids = [nextSettings.favicon_media_id, nextSettings.og_image_media_id].filter(Boolean)
      if (!ids.length) return
      try {
        let list = []
        try {
          const m = await admin.media.list({ ids })
          list = m.media || m.items || []
        } catch {
          const fetched = await Promise.all(
            ids.map((id) => admin.media.get(id).catch(() => null))
          )
          list = fetched.map((r) => r?.media || r).filter(Boolean)
        }
        const map = {}
        for (const item of list) map[item.id] = item
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
      toast.ok('Settings saved')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  const fav = mediaMap[settings.favicon_media_id]
  const og = mediaMap[settings.og_image_media_id]

  return (
    <div className="page">
      <header className="page-head">
        <div>
          <h1>Settings</h1>
          <p className="muted">SEO defaults, favicon, site name, social, contact email, analytics</p>
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
            <img className="mini-thumb" src={apiUrl(mediaThumbUrl(fav))} alt="favicon" loading="lazy" />
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
            <img className="mini-thumb" src={apiUrl(mediaThumbUrl(og))} alt="og" loading="lazy" />
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

        <h2>Contact</h2>
        <label>
          Contact email
          <input
            type="email"
            value={settings.contact_email || ''}
            onChange={(e) => patch('contact_email', e.target.value)}
            placeholder="messages from the public contact form"
            autoComplete="email"
          />
        </label>
        <p className="muted">
          Public contact form on sheyanova.art sends messages here. Requires SMTP on the API
          (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM).
        </p>

        <h2>Analytics</h2>
        <label className="field-row">
          <input
            type="checkbox"
            checked={Boolean(settings.yandex_metrika_enabled)}
            onChange={(e) => patch('yandex_metrika_enabled', e.target.checked)}
          />
          <span>Connect Yandex.Metrika counter</span>
        </label>
        <label>
          Counter ID
          <input
            value={settings.yandex_metrika_id || ''}
            onChange={(e) => patch('yandex_metrika_id', e.target.value)}
            placeholder="95095785"
            inputMode="numeric"
            disabled={!settings.yandex_metrika_enabled}
          />
        </label>
        <p className="muted">
          When enabled, the counter is injected into published pages on sheyanova.art after
          Publish. Admin preview does not load Metrika. Toggle off and publish again to remove it.
        </p>
        <label className="field-row">
          <input
            type="checkbox"
            checked={Boolean(settings.gtm_enabled)}
            onChange={(e) => patch('gtm_enabled', e.target.checked)}
          />
          <span>Connect Google Tag Manager</span>
        </label>
        <label>
          Container ID
          <input
            value={settings.gtm_container_id || ''}
            onChange={(e) => patch('gtm_container_id', e.target.value)}
            placeholder="GTM-K5DWKFDZ"
            disabled={!settings.gtm_enabled}
          />
        </label>
        <p className="muted">
          When enabled, GTM head + body snippets are injected on published pages only (not admin
          preview). Default container: GTM-K5DWKFDZ.
        </p>

        <h2>Menu</h2>
        <p className="muted">
          Header links and dropdowns are edited in the <Link to="/nav">Menu</Link> section.
        </p>

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
