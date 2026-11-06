import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { graphql } from '../../shared/api/graphql'
import { useAuth } from '../auth/AuthContext'
import { useCart } from '../cart/CartContext'
import { salePrice, vnd } from '../../shared/lib/format'

export function CheckoutPage() {
  const [couponCode, setCouponCode] = useState(() => new URLSearchParams(window.location.search).get('coupon') || '')
  const [quote, setQuote] = useState<{ id: string; subtotalVnd: string; discountVnd: string; totalPriceVnd: string; expiresAt: string; cartVersion: string } | null>(null)
  const [quoteLoading, setQuoteLoading] = useState(false)
  const [quoteError, setQuoteError] = useState('')
  const { authenticated } = useAuth()
  const { cart, cartVersion, expiresAt, loading, error, checkout, checkoutCOD } = useCart()
  const total = cart.reduce((sum, line) => sum + salePrice(line.price, line.discountPercentage) * line.quantity, 0)
  const expired = expiresAt ? Date.parse(expiresAt) <= Date.now() : false

  const previewCoupon = useCallback(async () => {
    setQuoteLoading(true)
    setQuoteError('')
    setQuote(null)
    try {
      const result = await graphql<{ previewCartCheckout: { id: string; subtotalVnd: string; discountVnd: string; totalPriceVnd: string; expiresAt: string; cartVersion: string } }>(
        `query PreviewCartCheckout($couponCode: String) { previewCartCheckout(couponCode: $couponCode) { id subtotalVnd discountVnd totalPriceVnd expiresAt cartVersion } }`,
        { couponCode: couponCode.trim() || null },
      )
      setQuote(result.previewCartCheckout)
    } catch (reason) {
      setQuoteError(reason instanceof Error ? reason.message : 'Không thể kiểm tra mã giảm giá.')
    } finally {
      setQuoteLoading(false)
    }
  }, [couponCode])
  useEffect(() => {
    if (!authenticated || cart.length === 0) return
    const timer = window.setTimeout(() => void previewCoupon(), 250)
    return () => window.clearTimeout(timer)
  }, [authenticated, cart.length, cartVersion, previewCoupon])
  if (!authenticated) return <main className="state checkout page-enter"><p className="eyebrow">THANH TOÃN AN TOÃ€N</p><h1>ÄÄƒng nháº­p Ä‘á»ƒ tiáº¿p tá»¥c</h1><p>PhiÃªn giá»¯ hÃ ng vÃ  Ä‘Æ¡n hÃ ng chá»‰ thuá»™c tÃ i khoáº£n Ä‘Ã£ xÃ¡c thá»±c.</p><Link className="button" to="/login">ÄÄƒng nháº­p & tiáº¿p tá»¥c</Link></main>
  if (!cart.length) return <main className="state checkout page-enter"><h1>KhÃ´ng cÃ³ sáº£n pháº©m Ä‘á»ƒ thanh toÃ¡n</h1><p>HÃ£y thÃªm sáº£n pháº©m vÃ o giá» trÆ°á»›c khi báº¯t Ä‘áº§u checkout.</p><Link className="button" to="/">KhÃ¡m phÃ¡ sáº£n pháº©m</Link></main>

  async function beginPayment() {
    try {
      if (!quote) throw new Error('Hãy chờ báo giá mới nhất trước khi thanh toán.')
      await checkout(`${window.location.origin}/checkout/success`, quote.id, quote.cartVersion)
    } catch {
      // CartContext already exposes the actionable error in the checkout page.
    }
  }



  async function beginCOD() {
    try {
      if (!quote) throw new Error('Hãy chờ báo giá mới nhất trước khi thanh toán.')
      await checkoutCOD(quote.id, quote.cartVersion)
      window.location.assign('/account/orders')
    } catch {
      // CartContext exposes the actionable error in the checkout page.
    }
  }

  return <main className="checkout page-enter">
    <p className="eyebrow">CHECKOUT / XÃC NHáº¬N</p>
    <h1>Kiá»ƒm tra Ä‘Æ¡n hÃ ng</h1>
    <p className="muted">GiÃ¡, tá»“n kho vÃ  phiÃªn giá»¯ hÃ ng Ä‘Æ°á»£c backend xÃ¡c nháº­n láº¡i khi táº¡o Ä‘Æ¡n. ThÃ´ng tin tháº» chá»‰ nháº­p táº¡i cá»•ng thanh toÃ¡n Ä‘Æ°á»£c phÃª duyá»‡t.</p>
    {error && <p className="error" role="alert">{error}</p>}
    {expired && <p className="notice" role="alert">PhiÃªn giá»¯ hÃ ng Ä‘Ã£ háº¿t háº¡n. Quay láº¡i giá» hÃ ng Ä‘á»ƒ táº£i láº¡i tá»“n kho trÆ°á»›c khi thá»­ láº¡i.</p>}
    <section className="checkout-grid">
      <div className="checkout-lines"><h2>Sáº£n pháº©m</h2>{cart.map(line => <article className="checkout-line" key={line.id}><img src={line.thumbnail} alt=""/><div><b>{line.title}</b><p>{line.quantity} Ã— {vnd(salePrice(line.price, line.discountPercentage))}</p></div><strong>{vnd(salePrice(line.price, line.discountPercentage) * line.quantity)}</strong></article>)}</div>
      <aside className="summary"><h2>Tá»•ng káº¿t</h2><label htmlFor="checkout-coupon">MÃ£ giáº£m giÃ¡</label><div className="coupon-row"><input id="checkout-coupon" autoComplete="off" maxLength={64} value={couponCode} onChange={event => { setCouponCode(event.target.value); setQuote(null); setQuoteError('') }} placeholder="Nháº­p mÃ£ (vÃ­ dá»¥ LAB10)"/><button type="button" className="secondary-action" disabled={quoteLoading || expired} onClick={() => void previewCoupon()}>{quoteLoading ? 'Äang kiá»ƒm traâ€¦' : 'Kiá»ƒm tra mÃ£'}</button></div>{quoteError && <p className="error" role="alert">{quoteError}</p>}<p className="notice">BÃ¡o giÃ¡ chá»‰ Ä‘á»ƒ xem trÆ°á»›c, khÃ´ng giá»¯ tá»“n kho hoáº·c lÆ°á»£t coupon. Order kiá»ƒm tra láº¡i giÃ¡, phiÃªn giá» vÃ  Ä‘iá»u kiá»‡n khi táº¡o Ä‘Æ¡n.</p><p><span>Táº¡m tÃ­nh</span><b>{vnd(quote ? Number(quote.subtotalVnd) : total)}</b></p>{quote && <p><span>Giáº£m giÃ¡</span><b>âˆ’{vnd(Number(quote.discountVnd))}</b></p>}<p><span>PhÃ­ váº­n chuyá»ƒn</span><span>TÃ­nh táº¡i cá»•ng thanh toÃ¡n</span></p>{quote && <p className="muted">BÃ¡o giÃ¡ háº¿t háº¡n: {new Date(quote.expiresAt).toLocaleTimeString()}</p>}<hr/><p className="total"><span>Tá»•ng dá»± kiáº¿n</span><b>{vnd(quote ? Number(quote.totalPriceVnd) : total)}</b></p><button className="primary" disabled={loading || expired || quoteLoading || !quote} onClick={() => void beginCOD()}>{loading ? 'Äang Ä‘áº·t Ä‘Æ¡nâ€¦' : expired ? 'PhiÃªn Ä‘Ã£ háº¿t háº¡n' : 'Thanh toÃ¡n COD'}</button><button className="button secondary-action" disabled={loading || expired || quoteLoading || !quote} onClick={() => void beginPayment()}>{loading ? 'Äang chuyá»ƒn tá»›i VNPAYâ€¦' : expired ? 'PhiÃªn Ä‘Ã£ háº¿t háº¡n' : 'Thanh toÃ¡n VNPAY'}</button><Link className="button secondary-action" to="/cart">Quay láº¡i giá» hÃ ng</Link></aside>
    </section>
  </main>
}
