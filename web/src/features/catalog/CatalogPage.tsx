import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import type { Product } from '../../domain/product'
import { getCatalog } from '../../shared/api/catalog'
import { ErrorState, LoadingGrid } from '../../components/ui/AsyncStates'
import { ProductCard } from './components/ProductCard'

export function CatalogPage({ add }: { add: (product: Product, quantity?: number) => Promise<void> }) {
  const [params] = useSearchParams()
  const query = params.get('q') || ''
  const selected = params.get('category') || ''
  const [products, setProducts] = useState<Product[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [sort, setSort] = useState('popular')
  const [page, setPage] = useState(1)

  const load = () => {
    setState('loading')
    getCatalog(query, selected)
      .then(result => { setProducts(result); setCategories([...new Set(result.map(product => product.category))].sort()); setPage(1); setState('ready') })
      .catch(() => setState('error'))
  }
  useEffect(load, [query, selected])
  const sorted = useMemo(() => [...products].sort((a, b) => sort === 'price-asc' ? a.price - b.price : sort === 'price-desc' ? b.price - a.price : b.rating - a.rating), [products, sort])
  const pageCount = Math.max(1, Math.ceil(sorted.length / 16))
  const shown = sorted.slice((page - 1) * 16, page * 16)

  if (state === 'error') return <ErrorState message="Không thể tải dữ liệu sản phẩm. Kết nối của bạn có thể chưa sẵn sàng." onRetry={load}/>
  return <main>
    <section className="catalog-intro page-enter"><p className="eyebrow">KHÁM PHÁ CÓ CHỌN LỌC</p><h1>Khám phá sản phẩm<br/><em>phù hợp hôm nay.</em></h1><p></p></section>
    <section className="category-row" aria-label="Danh mục"><Link className={!selected ? 'active' : ''} to={query ? `/search?q=${query}` : '/'}>Tất cả</Link>{categories.slice(0, 10).map(category => <Link key={category} className={selected === category ? 'active' : ''} to={`/search?category=${encodeURIComponent(category)}`}>{category.replaceAll('-', ' ')}</Link>)}</section>
    <section className="catalog-bar"><div><b>{query ? `Kết quả cho “${query}”` : 'Gợi ý dành cho bạn'}</b><span>{state === 'loading' ? ' Đang tải...' : ` · ${sorted.length} sản phẩm · trang ${page}/${pageCount}`}</span></div><label>Sắp xếp <select value={sort} onChange={event => { setSort(event.target.value); setPage(1) }}><option value="popular">Đánh giá cao</option><option value="price-asc">Giá thấp đến cao</option><option value="price-desc">Giá cao đến thấp</option></select></label></section>
    {state === 'loading' ? <LoadingGrid/> : shown.length ? <><section className="grid">{shown.map(product => <ProductCard key={product.id} product={product} onAdd={add}/>)}</section><nav className="pagination" aria-label="Phân trang sản phẩm">{Array.from({ length: pageCount }, (_, index) => index + 1).map(number => <button key={number} className={number === page ? 'active' : ''} onClick={() => setPage(number)}>{number}</button>)}</nav></> : <section className="state"><h2>Không tìm thấy sản phẩm phù hợp</h2><p>Thử từ khóa hoặc danh mục khác.</p><Link className="button" to="/">Xem tất cả sản phẩm</Link></section>}
  </main>
}
