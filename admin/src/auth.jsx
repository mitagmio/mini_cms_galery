import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { getToken, setToken as persistToken } from './api'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [token, setTokenState] = useState(() => getToken())

  const setToken = useCallback((value) => {
    persistToken(value)
    setTokenState(value || '')
  }, [])

  const logout = useCallback(() => setToken(''), [setToken])

  const value = useMemo(
    () => ({
      token,
      isAuthed: Boolean(token),
      setToken,
      logout,
    }),
    [token, setToken, logout]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth outside AuthProvider')
  return ctx
}
