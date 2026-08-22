import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { admin, getToken, setToken as persistToken } from './api'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [token, setTokenState] = useState(() => getToken())
  const [sessionReady, setSessionReady] = useState(() => !getToken())

  const setToken = useCallback((value) => {
    persistToken(value)
    setTokenState(value || '')
    if (!value) setSessionReady(true)
  }, [])

  const logout = useCallback(async () => {
    try {
      await admin.session.destroy()
    } catch {
      // still drop local token
    }
    persistToken('')
    setTokenState('')
    setSessionReady(true)
  }, [])

  useEffect(() => {
    if (!token) {
      setSessionReady(true)
      return
    }
    let cancelled = false
    setSessionReady(false)
    admin.session
      .create()
      .then(() => {
        if (!cancelled) setSessionReady(true)
      })
      .catch(() => {
        if (!cancelled) {
          persistToken('')
          setTokenState('')
          setSessionReady(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [token])

  const value = useMemo(
    () => ({
      token,
      isAuthed: Boolean(token),
      sessionReady,
      setToken,
      logout,
    }),
    [token, sessionReady, setToken, logout]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth outside AuthProvider')
  return ctx
}
