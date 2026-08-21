import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'

const LINKS = [
  { to: '/', end: true, label: 'Dashboard' },
  { to: '/pages', label: 'Pages' },
  { to: '/media', label: 'Media' },
  { to: '/templates', label: 'Templates' },
  { to: '/settings', label: 'Settings' },
  { to: '/preview', label: 'Preview' },
  { to: '/publish', label: 'Publish' },
]

export default function Layout() {
  const { logout } = useAuth()
  const navigate = useNavigate()

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">DS</span>
          <div>
            <strong>Sheyanova</strong>
            <span className="muted">Admin</span>
          </div>
        </div>
        <nav className="nav">
          {LINKS.map((l) => (
            <NavLink key={l.to} to={l.to} end={l.end} className={({ isActive }) => (isActive ? 'active' : undefined)}>
              {l.label}
            </NavLink>
          ))}
        </nav>
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
      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
