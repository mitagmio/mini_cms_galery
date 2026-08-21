import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'
import { useToast } from '../toast'

export default function Login() {
  const { isAuthed, setToken } = useAuth()
  const [value, setValue] = useState('')
  const navigate = useNavigate()
  const toast = useToast()

  if (isAuthed) return <Navigate to="/" replace />

  function submit(e) {
    e.preventDefault()
    const token = value.trim()
    if (!token) {
      toast.error('Enter admin token')
      return
    }
    setToken(token)
    toast.ok('Signed in')
    navigate('/', { replace: true })
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
          />
        </label>
        <button type="submit">Sign in</button>
      </form>
    </div>
  )
}
