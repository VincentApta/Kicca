import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react'
import { api } from './api'
import type { User } from './types'

type AuthState =
  | { phase: 'booting' }
  | { phase: 'anonymous' }
  | { phase: 'authenticated'; user: User }

const AuthCtx = createContext<{
  state: AuthState
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
} | null>(null)

export function useAuth() {
  const ctx = useContext(AuthCtx)
  if (!ctx) throw new Error('useAuth outside AuthProvider')
  return ctx
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ phase: 'booting' })

  useEffect(() => {
    const ctrl = new AbortController()
    api.me(ctrl.signal).then(
      ({ user }) => setState({ phase: 'authenticated', user }),
      () => setState({ phase: 'anonymous' }), // 401 or unreachable → login
    )
    return () => ctrl.abort()
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const { user } = await api.login(email, password)
    setState({ phase: 'authenticated', user })
  }, [])

  const logout = useCallback(async () => {
    await api.logout().catch(() => {})
    setState({ phase: 'anonymous' })
  }, [])

  return <AuthCtx.Provider value={{ state, login, logout }}>{children}</AuthCtx.Provider>
}
