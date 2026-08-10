import { useEffect, useState, type ChangeEvent, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { graphql } from '../../shared/api/graphql'
import { vnd } from '../../shared/lib/format'
import { meFields, useAuth } from '../auth/AuthContext'

type Order = { id: number; createdAt: string; totalPrice: number; products: Array<{ id: string; name: string; quantity: number; price: number }> }
type ProfileForm = { name: string; email: string; avatarUrl: string; phone: string; shippingAddress: string }

export function OrderHistoryPage() {
  const { account, authenticated } = useAuth()
  const [orders, setOrders] = useState<Order[]>([])
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [message, setMessage] = useState('')
  const [form, setForm] = useState<ProfileForm>({ name: '', email: '', avatarUrl: '', phone: '', shippingAddress: '' })
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState('')

  useEffect(() => {
    if (!authenticated || !account) {
      setState('ready')
      return
    }
    setForm({ name: account.name, email: account.email, avatarUrl: account.avatarUrl || '', phone: account.phone || '', shippingAddress: account.shippingAddress || '' })
    graphql<{ myOrders: Order[] }>('query MyOrders { myOrders { id createdAt totalPrice products { id name quantity price } } }')
      .then(result => { setOrders(result.myOrders); setState('ready') })
      .catch(cause => { setMessage(cause instanceof Error ? cause.message : 'Không thể tải đơn hàng.'); setState('error') })
  }, [authenticated, account])

  function chooseAvatar(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/') || file.size > 2 * 1024 * 1024) {
      setSaved('Avatar phải là ảnh và nhỏ hơn 2 MB.')
      event.target.value = ''
      return
    }
    const reader = new FileReader()
    reader.onload = () => setForm(current => ({ ...current, avatarUrl: String(reader.result) }))
    reader.readAsDataURL(file)
  }

  async function saveProfile(event: FormEvent) {
    event.preventDefault()
    setSaving(true)
    setSaved('')
    try {
      await graphql<{ updateMyProfile: unknown }>(`mutation UpdateMyProfile($input: ProfileInput!) { updateMyProfile(input: $input) { ${meFields} } }`, { input: form })
      setSaved('Đã lưu thông tin tài khoản.')
      window.location.reload()
    } catch (cause) {
      setSaved(cause instanceof Error ? cause.message : 'Không thể lưu thông tin.')
    } finally {
      setSaving(false)
    }
  }

  if (!authenticated) return <main className="state"><h1>Đơn hàng của tôi</h1><p>Đăng nhập để xem lịch sử đơn hàng của chính bạn.</p><Link className="button" to="/login">Đăng nhập</Link></main>
  if (state === 'loading') return <main className="state"><p>Đang tải đơn hàng…</p></main>
  if (state === 'error') return <main className="state"><h1>Không thể tải đơn hàng</h1><p>{message}</p></main>

  return (
    <main className="orders page-enter">
      <p className="eyebrow">TÀI KHOẢN</p>
      <h1>Thông tin tài khoản</h1>
      <section className="profile-card">
        <div className="profile-avatar">
          {form.avatarUrl ? <img src={form.avatarUrl} alt={`Avatar của ${form.name}`} /> : <span>{form.name.slice(0, 1).toUpperCase() || 'U'}</span>}
        </div>
        <form className="profile-form" onSubmit={saveProfile}>
          <label>Tên người dùng<input value={form.name} onChange={event => setForm({ ...form, name: event.target.value })} required /></label>
          <label>Gmail / email<input type="email" value={form.email} onChange={event => setForm({ ...form, email: event.target.value })} required /></label>
          <label>Avatar URL<input value={form.avatarUrl.startsWith('data:') ? '' : form.avatarUrl} placeholder="Google avatar hoặc URL ảnh riêng" onChange={event => setForm({ ...form, avatarUrl: event.target.value })} /></label>
          <label className="avatar-upload">Tải avatar từ máy<input type="file" accept="image/png,image/jpeg,image/webp,image/gif" onChange={chooseAvatar} /></label>
          <div className="profile-actions"><button type="button" className="button secondary" onClick={() => setForm({ ...form, avatarUrl: '' })}>Xóa avatar</button></div>
          <label>Số điện thoại<input value={form.phone} onChange={event => setForm({ ...form, phone: event.target.value })} /></label>
          <label>Địa chỉ giao hàng<textarea value={form.shippingAddress} onChange={event => setForm({ ...form, shippingAddress: event.target.value })} rows={3} /></label>
          <button className="button" disabled={saving}>{saving ? 'Đang lưu…' : 'Lưu thông tin'}</button>
          {saved && <p className="form-message">{saved}</p>}
        </form>
      </section>
      <h2>Lịch sử đơn hàng</h2>
      {!orders.length && <p>Chưa có đơn hàng.</p>}
      {orders.map(order => <article className="order-card" key={order.id}><div><b>Đơn #{order.id}</b><span>{new Date(order.createdAt).toLocaleDateString('vi-VN')}</span></div>{order.products.map(item => <p key={item.id}>{item.name} × {item.quantity}<b>{vnd(item.price * item.quantity)}</b></p>)}<strong>{vnd(order.totalPrice)}</strong></article>)}
    </main>
  )
}
