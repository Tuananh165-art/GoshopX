import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import type { Product } from '../../domain/product'
import { addProductReview, canReviewProduct, getCatalogProduct } from '../../shared/api/catalog'
import { salePrice, vnd } from '../../shared/lib/format'

const stars = [1, 2, 3, 4, 5]

export function ProductDetailPage({ add }: { add: (product: Product, quantity?: number) => Promise<void> }) {
  const { id = '' } = useParams()
  const [product, setProduct] = useState<Product>()
  const [error, setError] = useState('')
  const [addError, setAddError] = useState('')
  const [quantity, setQuantity] = useState(1)
  const [image, setImage] = useState('')
  const [reviewRating, setReviewRating] = useState(5)
  const [reviewComment, setReviewComment] = useState('')
  const [reviewMessage, setReviewMessage] = useState('')
  const [reviewSubmitting, setReviewSubmitting] = useState(false)
  const [reviewEligible, setReviewEligible] = useState<boolean | null>(null)

  useEffect(() => {
    let active = true
    const load = () => Promise.all([getCatalogProduct(id), canReviewProduct(id)]).then(([value, eligible]) => {
      if (!active) return
      setProduct(value)
      setReviewEligible(eligible)
      setImage(current => current || value.images[0] || value.thumbnail)
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : 'Không thể tải sản phẩm') })
    void load()
    const timer = window.setInterval(() => void load(), 3000)
    return () => { active = false; window.clearInterval(timer) }
  }, [id])

  if (error) return <main className="state"><h1>Không thể mở sản phẩm</h1><p>{error}</p><Link className="button" to="/">Quay lại danh mục</Link></main>
  if (!product) return <main className="state"><p>Đang tải thông tin sản phẩm…</p></main>

  const addProduct = async () => {
    try {
      setAddError('')
      await add(product, quantity)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : ''
      setAddError(message.includes('UserId not found') ? 'Đăng nhập để thêm sản phẩm vào giỏ hàng.' : message || 'Không thể thêm sản phẩm vào giỏ hàng. Vui lòng thử lại.')
    }
  }

  const submitReview = async () => {
    if (!reviewEligible || !reviewComment.trim() || reviewSubmitting) return
    try {
      setReviewSubmitting(true)
      setReviewMessage('')
      await addProductReview(id, reviewRating, reviewComment.trim())
      const persisted = await getCatalogProduct(id)
      setProduct(persisted)
      setReviewComment('')
      setReviewRating(5)
      setReviewMessage('Đánh giá đã được đăng và đồng bộ ngay lập tức.')
    } catch (cause) {
      setReviewMessage(cause instanceof Error ? cause.message : 'Không thể đăng đánh giá.')
    } finally {
      setReviewSubmitting(false)
    }
  }

  const reviewComposer = reviewEligible === true ? <div className="review-form"><div><p className="eyebrow">ĐÃ THANH TOÁN</p><h3>Chia sẻ trải nghiệm của bạn</h3><p className="review-help">Bạn đã thanh toán sản phẩm này. Đánh giá của bạn giúp khách hàng khác lựa chọn tốt hơn.</p></div><div className="rating-picker" aria-label="Chọn số sao">{stars.map(star => <button type="button" key={star} className={star <= reviewRating ? 'selected' : ''} aria-label={`${star} sao`} onClick={() => setReviewRating(star)}>★</button>)}<span>{reviewRating}/5</span></div><textarea value={reviewComment} onChange={event => setReviewComment(event.target.value)} placeholder="Bạn cảm thấy thế nào về sản phẩm?" maxLength={1000} aria-label="Nội dung đánh giá"/><div className="review-form-footer"><small>{reviewComment.length}/1000 ký tự</small><button className="primary" disabled={!reviewComment.trim() || reviewSubmitting} onClick={() => void submitReview()}>{reviewSubmitting ? 'ĐANG ĐĂNG...' : 'GỬI ĐÁNH GIÁ'}</button></div>{reviewMessage && <p className="review-success" role="status">{reviewMessage}</p>}</div> : <div className="review-locked" role="status"><span className="review-lock-icon">🔒</span><div><b>Đánh giá sau khi thanh toán</b><p>Hoàn tất thanh toán COD hoặc VNPAY cho sản phẩm này để mở chức năng đánh giá và bình luận.</p></div><Link className="button review-buy-button" to="#product-buy">Mua sản phẩm</Link></div>

  return <main className="detail page-enter">
    <section className="gallery"><img className="hero-image" src={image} alt={product.title}/><div>{product.images.map(url => <button className={url === image ? 'selected' : ''} onClick={() => setImage(url)} key={url}><img src={url} alt=""/></button>)}</div></section>
    <section className="product-info" id="product-buy"><nav className="breadcrumbs"><Link to="/">Trang chủ</Link> / <Link to={`/search?category=${product.category}`}>{product.category}</Link></nav><h1>{product.title}</h1><p className="rating">★ {product.rating.toFixed(1)} · {product.reviews?.length || 0} đánh giá</p><div className="big-price"><strong>{vnd(salePrice(product.price, product.discountPercentage))}</strong><del>{vnd(product.price)}</del><span>GIẢM {Math.round(product.discountPercentage)}%</span></div><p>{product.description}</p><dl><div><dt>Thương hiệu</dt><dd>{product.brand || 'Đang cập nhật'}</dd></div><div><dt>Tình trạng</dt><dd>{product.availabilityStatus || (product.stock ? `Còn ${product.stock} sản phẩm` : 'Hết hàng')}</dd></div><div><dt>Vận chuyển</dt><dd>{product.shippingInformation || 'Vận chuyển tiêu chuẩn'}</dd></div></dl><div className="quantity"><label>Số lượng</label><button onClick={() => setQuantity(Math.max(1, quantity - 1))}>−</button><output>{quantity}</output><button onClick={() => setQuantity(Math.min(Math.max(1, product.stock), quantity + 1))}>+</button><small>Tối đa {product.stock}</small></div><button className="primary" disabled={!product.stock} onClick={() => void addProduct()}>Thêm {quantity} vào giỏ</button>{addError && <p className="error" role="alert">{addError} {addError.startsWith('Đăng nhập') && <Link to="/login">Đăng nhập</Link>}</p>}<p className="secure-note">Giá, tồn kho và phiên giữ hàng được GoshopX xác nhận lại tại checkout.</p></section>
    <section className="details review-section"><div className="review-heading"><div><p className="eyebrow">PHẢN HỒI THỰC TẾ</p><h2>Đánh giá từ người mua</h2></div><div className="review-summary"><strong>{product.rating.toFixed(1)}</strong><span>{stars.map(star => <span key={star} className={star <= Math.round(product.rating) ? 'star filled' : 'star'}>★</span>)}</span><small>{product.reviews?.length || 0} đánh giá</small></div></div><div className="review-list">{product.reviews?.length ? product.reviews.slice(0, 20).map((review, index) => <article className="review-card" key={`${review.date}-${review.reviewerEmail}-${index}`}><div className="review-card-top"><div className="review-author"><span className="review-avatar">{(review.reviewerName || 'K').slice(0, 1).toUpperCase()}</span><div><b>{review.reviewerName || 'Khách hàng'}</b><small>{review.date ? new Date(review.date).toLocaleDateString('vi-VN') : 'Người mua đã xác minh'}</small></div></div><span className="review-stars">{stars.map(star => <span key={star} className={star <= review.rating ? 'filled' : ''}>★</span>)}</span></div><p>{review.comment}</p></article>) : <div className="review-empty"><span>☆</span><b>Chưa có đánh giá nào</b><p>Hãy là người đầu tiên chia sẻ trải nghiệm sản phẩm.</p></div>}</div>{reviewEligible === null ? <div className="review-locked"><span className="review-lock-icon">…</span><div><b>Đang kiểm tra trạng thái thanh toán</b><p>Đang đồng bộ đơn hàng của bạn.</p></div></div> : reviewComposer}</section>
  </main>
}
