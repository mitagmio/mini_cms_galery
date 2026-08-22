import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { admin, setToken as persistToken } from '../api'
import { useAuth } from '../auth'
import { useToast } from '../toast'

export default function Login() {
  const { isAuthed, setToken } = useAuth()
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const toast = useToast()

  if (isAuthed) return <Navigate to="/" replace />

  async function submit(e) {
    e.preventDefault()
    const token = value.trim()
    if (!token) {
      toast.error('Enter admin token')
      return
    }
    setBusy(true)
    persistToken(token)
    try {
      await admin.session.create()
      setToken(token)
      toast.ok('Signed in')
      navigate('/', { replace: true })
    } catch {
      persistToken('')
      toast.error('Invalid token')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={submit}>
        <h1>Sheyanova Admin</h1>
        <p className="muted">Paste the Bearer admin token to continue.</p>
        <label>
          Admin token
          <input
            type="password"
            autoComplete="current-password"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="ADMIN_TOKEN"
            disabled={busy}
          />
        </label>
        <button type="submit" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
