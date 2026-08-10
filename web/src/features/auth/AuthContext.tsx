import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { graphql } from '../../shared/api/graphql'

type AuthMode = 'login' | 'register'
export type Account = { id: number; name: string; email: string; avatarUrl: string; phone: string; shippingAddress: string; roleId: number; role: string; status: string }
export function isAdminAccount(account: Pick<Account, 'roleId'> | null | undefined) {
  return Number(account?.roleId) === 1
}
type AuthContextValue = {
  account: Account | null
  authenticated: boolean
  pending: boolean
  authenticate: (mode: AuthMode, input: Record<string, string>) => Promise<Account>
  loginWithGoogle: (credential: string) => Promise<Account>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)
export const meFields = 'id name email avatarUrl phone shippingAddress roleId role status'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<Account | null>(null)
  const [pending, setPending] = useState(true)

  useEffect(() => {
    let active = true
    graphql<{ me: Account | null }>(`query Me { me { ${meFields} } }`)
      .then(({ me }) => { if (active) setAccount(me) })
      .catch(() => { if (active) setAccount(null) })
      .finally(() => { if (active) setPending(false) })
    return () => { active = false }
  }, [])

  async function authenticate(mode: AuthMode, input: Record<string, string>) {
    setPending(true)
    try {
      const query = mode === 'login'
        ? 'mutation Login($email: String!, $password: String!) { login(account: { email: $email, password: $password }) { token } }'
        : 'mutation Register($name: String!, $email: String!, $password: String!) { register(account: { name: $name, email: $email, password: $password }) { token } }'
      await graphql(query, input)
      const session = await graphql<{ me: Account | null }>(`query Me { me { ${meFields} } }`)
      if (!session.me) throw new Error('Phiên đăng nhập chưa được thiết lập. Vui lòng thử lại.')
      setAccount(session.me)
      return session.me
    } finally {
      setPending(false)
    }
  }

  async function loginWithGoogle(credential: string) {
    setPending(true)
    try {
      await graphql('mutation LoginWithGoogle($credential: String!) { loginWithGoogle(account: { credential: $credential }) { token } }', { credential })
      const session = await graphql<{ me: Account | null }>(`query Me { me { ${meFields} } }`)
      if (!session.me) throw new Error('Phiên đăng nhập chưa được thiết lập. Vui lòng thử lại.')
      setAccount(session.me)
      return session.me
    } finally {
      setPending(false)
    }
  }

  async function logout() {
    setPending(true)
    try {
      await graphql<{ logout: boolean }>('mutation Logout { logout }')
      setAccount(null)
    } finally {
      setPending(false)
    }
  }

  const value = useMemo(() => ({ account, authenticated: Boolean(account), pending, authenticate, loginWithGoogle, logout }), [account, pending])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
