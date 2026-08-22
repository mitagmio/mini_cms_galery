import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Login from './pages/Login'
import MediaLibrary from './pages/MediaLibrary'
import PageEditor from './pages/PageEditor'
import PagesList from './pages/PagesList'
import Preview from './pages/Preview'
import Publish from './pages/Publish'
import NavEditor from './pages/NavEditor'
import Settings from './pages/Settings'
import Templates from './pages/Templates'

function RequireAuth({ children }) {
  const { isAuthed, sessionReady } = useAuth()
  if (!isAuthed) return <Navigate to="/login" replace />
  if (!sessionReady) return <div className="login-page"><p className="muted">Signing in…</p></div>
  return children
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="pages" element={<PagesList />} />
        <Route path="pages/:id" element={<PageEditor />} />
        <Route path="nav" element={<NavEditor />} />
        <Route path="menu" element={<Navigate to="/nav" replace />} />
        <Route path="media" element={<MediaLibrary />} />
        <Route path="settings" element={<Settings />} />
        <Route path="templates" element={<Templates />} />
        <Route path="preview" element={<Preview />} />
        <Route path="publish" element={<Publish />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
