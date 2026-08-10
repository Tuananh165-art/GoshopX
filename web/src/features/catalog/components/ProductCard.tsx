import type { Product } from '../../../domain/product'
import { salePrice, vnd } from '../../../shared/lib/format'
import { Link } from 'react-router-dom'
import { useState } from 'react'

export function ProductCard({ product, onAdd }: { product: Product; onAdd: (product: Product, quantity?: number) => Promise<void> }) {
  const [addError, setAddError] = useState('')
  const addProduct = async () => {
    try {
      setAddError('')
      await onAdd(product)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : ''
      setAddError(message.includes('UserId not found') ? 'Đăng nhập để thêm sản phẩm vào giỏ hàng.' : message || 'Không thể thêm sản phẩm vào giỏ hàng. Vui lòng thử lại.')
    }
  }
  return <article className="product-card motion-card">
    <Link to={`/products/${product.id}`} aria-label={`Xem ${product.title}`}>
      <div className="thumb"><img src={product.thumbnail} alt="" loading="lazy"/><span>-{Math.round(product.discountPercentage)}%</span></div>
      <h3>{product.title}</h3>
      <p className="rating">★ {product.rating.toFixed(1)} <small>· {product.stock} còn lại</small></p>
      <div className="price"><strong>{vnd(salePrice(product.price, product.discountPercentage))}</strong><del>{vnd(product.price)}</del></div>
    </Link>
    <button className="add" onClick={() => void addProduct()}>Thêm giỏ</button>
    {addError && <p className="card-error" role="alert">{addError} {addError.startsWith('Đăng nhập') && <Link to="/login">Đăng nhập</Link>}</p>}
  </article>
}
