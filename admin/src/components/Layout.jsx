import { createContext, useContext, useEffect, useMemo, useState } from 'react'
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
  const [menuOpen, setMenuOpen] = useState(false)
  const isPreview = /^\/preview\/?$/.test(location.pathname)
  const chromeApi = useMemo(() => ({ setPreviewChrome }), [])

  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  useEffect(() => {
    if (!menuOpen) return undefined
    const onKey = (e) => {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [menuOpen])

  async function handleLogout() {
    setMenuOpen(false)
    await logout()
    navigate('/login')
  }

  function renderLinks() {
    return LINKS.map((l) => (
      <NavLink
        key={l.to}
        to={l.to}
        end={l.end}
        className={({ isActive }) => (isActive ? 'active' : undefined)}
        onClick={() => setMenuOpen(false)}
      >
        {l.label}
      </NavLink>
    ))
  }

  return (
    <PreviewChromeContext.Provider value={chromeApi}>
      <div className={`app-shell${isPreview ? ' app-shell--preview' : ''}${menuOpen ? ' app-shell--nav-open' : ''}`}>
        <header className={`topbar${isPreview ? ' topbar--preview' : ''}`}>
          <div className="brand">
            <span className="brand-mark">DS</span>
            <div>
              <strong>Sheyanova</strong>
              <span className="muted">Admin</span>
            </div>
          </div>
          <nav className="nav nav--desktop" aria-label="Admin">
            {renderLinks()}
          </nav>
          {isPreview && previewChrome ? (
            <div className="preview-chrome">{previewChrome}</div>
          ) : null}
          <button type="button" className="secondary topbar-logout" onClick={handleLogout}>
            Log out
          </button>
          <button
            type="button"
            className="nav-burger"
            aria-label={menuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={menuOpen}
            aria-controls="admin-nav-drawer"
            onClick={() => setMenuOpen((open) => !open)}
          >
            <span className="nav-burger-lines" aria-hidden="true" />
          </button>
        </header>

        <div
          className={`nav-backdrop${menuOpen ? ' is-open' : ''}`}
          aria-hidden={!menuOpen}
          onClick={() => setMenuOpen(false)}
        />
        <aside
          id="admin-nav-drawer"
          className={`nav-drawer${menuOpen ? ' is-open' : ''}`}
          aria-hidden={!menuOpen}
          aria-label="Admin menu"
        >
          <nav className="nav nav--drawer">{renderLinks()}</nav>
          <button type="button" className="secondary nav-drawer-logout" onClick={handleLogout}>
            Log out
          </button>
        </aside>

        <main className={`main${isPreview ? ' main--preview' : ''}`}>
          <Outlet />
        </main>
      </div>
    </PreviewChromeContext.Provider>
  )
}
