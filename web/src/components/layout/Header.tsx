import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { isAdminAccount, useAuth } from '../../features/auth/AuthContext'

export function Header({ cartCount }: { cartCount: number }) {
  const [term, setTerm] = useState('')
  const navigate = useNavigate()
  const { account, authenticated, logout } = useAuth()
  function submit(event: FormEvent) { event.preventDefault(); navigate(`/search?q=${encodeURIComponent(term)}`) }
  async function signOut() { await logout(); navigate('/') }
  return <>
    <div className="utility"><span>📍 Giao đến: Việt Nam</span><span>Trợ giúp · Thông báo · Tiếng Việt</span></div>
    <header className="header"><div className="header-main">
      <Link className="brand" to="/">GOSHOP<span>X</span></Link>
      <form className="search" onSubmit={submit}><input aria-label="Tìm sản phẩm" value={term} onChange={e => setTerm(e.target.value)} placeholder="Tìm thương hiệu, danh mục, sản phẩm..."/><button>Tìm kiếm</button></form>
      <nav>{authenticated ? <><Link className="account-link" to="/account/orders">{account?.avatarUrl ? <img className="header-avatar" src={account.avatarUrl} alt="" /> : <span className="header-avatar header-avatar-placeholder">{(account?.name || 'U').slice(0, 1).toUpperCase()}</span>}<span>{account?.name || 'Tài khoản'}</span></Link><button className="header-action" onClick={() => void signOut()}>Đăng xuất</button></> : <><Link to="/login">Đăng nhập</Link><Link to="/register">Đăng ký</Link></>}<Link to="/cart" aria-label={`Giỏ hàng (${cartCount})`}>Giỏ hàng <b>{cartCount}</b></Link>{isAdminAccount(account) && <Link to="/admin">Quản trị</Link>}</nav>
    </div></header>
    <nav className="category-nav" aria-label="Điều hướng mua sắm"><div><Link to="/">Trang chủ</Link><Link to="/search?category=smartphones">Điện thoại</Link><Link to="/search?category=laptops">Laptop</Link><Link to="/search?category=mens-shirts">Thời trang</Link><Link to="/search?category=beauty">Làm đẹp</Link><Link className="deal-link" to="/assistant">Trợ lý sản phẩm</Link><Link className="deal-link" to="/search?sort=popular">Ưu đãi trong ngày</Link></div></nav>
  </>
}
