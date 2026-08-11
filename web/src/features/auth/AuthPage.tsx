import { FormEvent, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { graphql } from '../../shared/api/graphql'
import { isAdminAccount, useAuth } from './AuthContext'

const REQUEST_RESET = `mutation RequestPasswordReset($email: String!) { requestPasswordReset(email: $email) }`
const RESET_PASSWORD = `mutation ResetPassword($email: String!, $otp: String!, $newPassword: String!) { resetPassword(email: $email, otp: $otp, newPassword: $newPassword) }`

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const [error, setError] = useState('')
  const [resetEmail, setResetEmail] = useState('')
  const [resetStep, setResetStep] = useState<'request' | 'verify'>('request')
  const [resetPending, setResetPending] = useState(false)
  const navigate = useNavigate()
  const { authenticate, loginWithGoogle, pending } = useAuth()
  const googleButton = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const clientID = import.meta.env.VITE_GOOGLE_CLIENT_ID
    if (mode !== 'login' || !googleButton.current || !clientID) return

    let cancelled = false
    const renderGoogleButton = () => {
      const google = (window as any).google
      if (cancelled || !google || !googleButton.current) return false

      google.accounts.id.initialize({
        client_id: clientID,
        callback: async (response: { credential: string }) => {
          try { setError(''); const account = await loginWithGoogle(response.credential); navigate(isAdminAccount(account) ? '/admin' : '/account/orders') }
          catch (cause) { setError(cause instanceof Error ? cause.message : 'Đăng nhập Google thất bại. Vui lòng thử lại.') }
        },
      })
      googleButton.current.replaceChildren()
      google.accounts.id.renderButton(googleButton.current, { theme: 'outline', size: 'large', width: 320, text: 'continue_with' })
      return true
    }

    if (renderGoogleButton()) return () => { cancelled = true }

    let script = document.querySelector<HTMLScriptElement>('script[src="https://accounts.google.com/gsi/client"]')
    if (!script) {
      script = document.createElement('script')
      script.src = 'https://accounts.google.com/gsi/client'
      script.async = true
      document.head.appendChild(script)
    }
    script.addEventListener('load', renderGoogleButton, { once: true })
    return () => {
      cancelled = true
      script?.removeEventListener('load', renderGoogleButton)
    }
  }, [loginWithGoogle, mode, navigate])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const input = Object.fromEntries(form.entries()) as Record<string, string>
    if (!input.email.includes('@') || input.password.length < 8 || (mode === 'register' && !input.name.trim())) {
      setError('Nhập đầy đủ thông tin và mật khẩu tối thiểu 8 ký tự.')
      return
    }
    try { setError(''); const account = await authenticate(mode, input); navigate(isAdminAccount(account) ? '/admin' : '/account/orders') }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Đăng nhập thất bại. Vui lòng thử lại.') }
  }

  async function requestReset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!resetEmail.includes('@')) { setError('Nhập email hợp lệ.'); return }
    try {
      setResetPending(true); setError('')
      await graphql(REQUEST_RESET, { email: resetEmail })
      setResetStep('verify')
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Không thể gửi OTP. Vui lòng thử lại.') }
    finally { setResetPending(false) }
  }

  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const otp = String(form.get('otp') || '')
    const newPassword = String(form.get('newPassword') || '')
    if (!/^\d{6}$/.test(otp) || newPassword.length < 8) { setError('OTP gồm 6 chữ số và mật khẩu mới tối thiểu 8 ký tự.'); return }
    try {
      setResetPending(true); setError('')
      await graphql(RESET_PASSWORD, { email: resetEmail, otp, newPassword })
      setResetStep('request')
      setResetEmail('')
      navigate('/login', { replace: true, state: { message: 'Đổi mật khẩu thành công. Hãy đăng nhập lại.' } })
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'OTP không đúng hoặc đã hết hạn.') }
    finally { setResetPending(false) }
  }

  if (mode === 'login' && resetStep !== 'request') return <main className="auth page-enter"><form onSubmit={resetPassword}><p className="eyebrow">GOSHOPX ACCOUNT</p><h1>Đổi mật khẩu</h1><p>Nhập OTP đã được gửi tới {resetEmail}.</p><label>OTP 6 số<input name="otp" inputMode="numeric" maxLength={6} required /></label><label>Mật khẩu mới<input name="newPassword" type="password" minLength={8} autoComplete="new-password" required /></label>{error && <p role="alert" className="error">{error}</p>}<button className="primary" disabled={resetPending}>{resetPending ? 'Đang xử lý…' : 'Đổi mật khẩu'}</button><p><button type="button" className="text-button" onClick={() => { setResetStep('request'); setError('') }}>Gửi lại OTP</button></p></form></main>

  return <main className="auth page-enter"><form onSubmit={mode === 'login' ? submit : submit}><p className="eyebrow">GOSHOPX ACCOUNT</p><h1>{mode === 'login' ? 'Chào mừng trở lại' : 'Tạo tài khoản'}</h1><p>{mode === 'login' ? 'Đăng nhập để giữ hàng và xem đơn của bạn.' : 'Đăng ký tài khoản khách hàng GoshopX.'}</p>{mode === 'register' && <label>Họ và tên<input name="name" required autoComplete="name" /></label>}<label>Email<input name="email" type="email" required autoComplete="email" /></label><label>Mật khẩu<input name="password" type="password" minLength={8} required autoComplete={mode === 'login' ? 'current-password' : 'new-password'} /></label>{error && <p role="alert" className="error">{error}</p>}<button className="primary" disabled={pending}>{pending ? 'Đang xử lý…' : mode === 'login' ? 'Đăng nhập' : 'Đăng ký'}</button>{mode === 'login' && <><button type="button" className="forgot-link" onClick={() => { setResetStep('request'); setError('') }}>Quên mật khẩu?</button>{import.meta.env.VITE_GOOGLE_CLIENT_ID && <><div className="auth-divider">hoặc</div><div ref={googleButton} /></>}</>}{mode === 'login' ? <p>Chưa có tài khoản? <Link to="/register">Đăng ký</Link></p> : <p>Đã có tài khoản? <Link to="/login">Đăng nhập</Link></p>}</form>{mode === 'login' && resetStep === 'request' && <form className="forgot-panel" onSubmit={requestReset}><h2>Gửi OTP đổi mật khẩu</h2><label>Email<input type="email" value={resetEmail} onChange={event => setResetEmail(event.target.value)} placeholder="email@example.com" required /></label><button className="primary" disabled={resetPending}>{resetPending ? 'Đang gửi…' : 'Gửi OTP về email'}</button></form>}</main>
}
