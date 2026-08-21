import { createContext, useContext, useMemo, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'

const LINKS = [
  { to: '/', end: true, label: 'Dashboard' },
  { to: '/pages', label: 'Pages' },
  { to: '/nav', label: 'Menu' },
  { to: '/media', label: 'Media' },
  { to: '/templates', label: 'Templates' },
  { to: '/settings', label: 'Settings' },
  { to: '/preview', label: 'Preview' },
  { to: '/publish', label: 'Publish' },
]

const PreviewChromeContext = createContext({ setPreviewChrome: () => {} })

export function usePreviewChrome() {
  return useContext(PreviewChromeContext)
}

export default function Layout() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [previewChrome, setPreviewChrome] = useState(null)
  const isPreview = /^\/preview\/?$/.test(location.pathname)
  const chromeApi = useMemo(() => ({ setPreviewChrome }), [])

  return (
    <PreviewChromeContext.Provider value={chromeApi}>
      <div className={`app-shell${isPreview ? ' app-shell--preview' : ''}`}>
        <header className={`topbar${isPreview ? ' topbar--preview' : ''}`}>
          <div className="brand">
            <span className="brand-mark">DS</span>
            <div>
              <strong>Sheyanova</strong>
              <span className="muted">Admin</span>
            </div>
          </div>
          <nav className="nav">
            {LINKS.map((l) => (
              <NavLink
                key={l.to}
                to={l.to}
                end={l.end}
                className={({ isActive }) => (isActive ? 'active' : undefined)}
              >
                {l.label}
              </NavLink>
            ))}
          </nav>
          {isPreview && previewChrome ? (
            <div className="preview-chrome">{previewChrome}</div>
          ) : null}
          <button
            type="button"
            className="secondary"
            onClick={() => {
              logout()
              navigate('/login')
            }}
          >
            Log out
          </button>
        </header>
        <main className={`main${isPreview ? ' main--preview' : ''}`}>
          <Outlet />
        </main>
      </div>
    </PreviewChromeContext.Provider>
  )
}
