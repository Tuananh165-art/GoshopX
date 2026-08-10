import { Link } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { useCart } from '../cart/CartContext'
import { salePrice, vnd } from '../../shared/lib/format'

export function CheckoutPage() {
  const { authenticated } = useAuth()
  const { cart, expiresAt, loading, error, checkout, checkoutCOD } = useCart()
  const total = cart.reduce((sum, line) => sum + salePrice(line.price, line.discountPercentage) * line.quantity, 0)
  const expired = expiresAt ? Date.parse(expiresAt) <= Date.now() : false

  if (!authenticated) return <main className="state checkout page-enter"><p className="eyebrow">THANH TOÁN AN TOÀN</p><h1>Đăng nhập để tiếp tục</h1><p>Phiên giữ hàng và đơn hàng chỉ thuộc tài khoản đã xác thực.</p><Link className="button" to="/login">Đăng nhập & tiếp tục</Link></main>
  if (!cart.length) return <main className="state checkout page-enter"><h1>Không có sản phẩm để thanh toán</h1><p>Hãy thêm sản phẩm vào giỏ trước khi bắt đầu checkout.</p><Link className="button" to="/">Khám phá sản phẩm</Link></main>

  async function beginPayment() {
    try {
      await checkout(`${window.location.origin}/checkout/success`)
    } catch {
      // CartContext already exposes the actionable error in the checkout page.
    }
  }

  async function beginCOD() {
    try {
      await checkoutCOD()
      window.location.assign('/account/orders')
    } catch {
      // CartContext exposes the actionable error in the checkout page.
    }
  }

  return <main className="checkout page-enter">
    <p className="eyebrow">CHECKOUT / XÁC NHẬN</p>
    <h1>Kiểm tra đơn hàng</h1>
    <p className="muted">Giá, tồn kho và phiên giữ hàng được backend xác nhận lại khi tạo đơn. Thông tin thẻ chỉ nhập tại cổng thanh toán được phê duyệt.</p>
    {error && <p className="error" role="alert">{error}</p>}
    {expired && <p className="notice" role="alert">Phiên giữ hàng đã hết hạn. Quay lại giỏ hàng để tải lại tồn kho trước khi thử lại.</p>}
    <section className="checkout-grid">
      <div className="checkout-lines"><h2>Sản phẩm</h2>{cart.map(line => <article className="checkout-line" key={line.id}><img src={line.thumbnail} alt=""/><div><b>{line.title}</b><p>{line.quantity} × {vnd(salePrice(line.price, line.discountPercentage))}</p></div><strong>{vnd(salePrice(line.price, line.discountPercentage) * line.quantity)}</strong></article>)}</div>
      <aside className="summary"><h2>Tổng kết</h2><p><span>Tạm tính</span><b>{vnd(total)}</b></p><p><span>Phí vận chuyển</span><span>Tính tại cổng thanh toán</span></p><hr/><p className="total"><span>Tổng dự kiến</span><b>{vnd(total)}</b></p><button className="primary" disabled={loading || expired} onClick={() => void beginCOD()}>{loading ? 'Đang đặt đơn…' : expired ? 'Phiên đã hết hạn' : 'Thanh toán COD'}</button><button className="button secondary-action" disabled={loading || expired} onClick={() => void beginPayment()}>{loading ? 'Đang chuyển tới VNPAY…' : expired ? 'Phiên đã hết hạn' : 'Thanh toán VNPAY'}</button><Link className="button secondary-action" to="/cart">Quay lại giỏ hàng</Link></aside>
    </section>
  </main>
}
