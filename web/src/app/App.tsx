import { lazy, Suspense } from 'react'
import { Link, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Header } from '../components/layout/Header'
import { Footer } from '../components/layout/Footer'
import { useCart } from '../features/cart/CartContext'
import { isAdminAccount, useAuth } from '../features/auth/AuthContext'
import { ChatBubble } from '../features/recommendation/ChatBubble'

const CatalogPage = lazy(() => import('../features/catalog/CatalogPage').then(module => ({ default: module.CatalogPage })))
const ProductDetailPage = lazy(() => import('../features/catalog/ProductDetailPage').then(module => ({ default: module.ProductDetailPage })))
const CartPage = lazy(() => import('../features/cart/CartPage').then(module => ({ default: module.CartPage })))
const CheckoutPage = lazy(() => import('../features/checkout/CheckoutPage').then(module => ({ default: module.CheckoutPage })))
const PaymentReturnPage = lazy(() => import('../features/checkout/PaymentReturnPage').then(module => ({ default: module.PaymentReturnPage })))
const AuthPage = lazy(() => import('../features/auth/AuthPage').then(module => ({ default: module.AuthPage })))
const AdminPage = lazy(() => import('../features/admin/AdminPage').then(module => ({ default: module.AdminPage })))
const OrderHistoryPage = lazy(() => import('../features/orders/OrderHistoryPage').then(module => ({ default: module.OrderHistoryPage })))
const RecommendationChatPage = lazy(() => import('../features/recommendation/RecommendationChatPage').then(module => ({ default: module.RecommendationChatPage })))
const PageFallback = () => <main className="state"><p>Đang tải trang…</p></main>
function AdminRoute({ section }: { section: string }) {
  const { account, pending } = useAuth()
  if (pending) return <PageFallback />
  if (!account) return <Navigate to="/login?returnTo=/admin" replace />
  if (!isAdminAccount(account)) return <main className="state"><h1>Không có quyền truy cập</h1><p>Tài khoản của bạn không có quyền quản trị.</p><Link className="button" to="/">Về cửa hàng</Link></main>
  return <AdminPage section={section}/>
}

const HIDE_CHAT_PREFIXES = ['/admin', '/login', '/register']
export default function App() {
  const { cart, add } = useCart()
  const location = useLocation()
  const hideChat = HIDE_CHAT_PREFIXES.some(prefix => location.pathname === prefix || location.pathname.startsWith(`${prefix}/`))
  return <><Header cartCount={cart.reduce((total, line) => total + line.quantity, 0)}/><Suspense fallback={<PageFallback/>}><Routes>
    <Route path="/" element={<CatalogPage add={add}/>}/><Route path="/search" element={<CatalogPage add={add}/>}/><Route path="/products/:id" element={<ProductDetailPage add={add}/>}/>
    <Route path="/cart" element={<CartPage/>}/><Route path="/checkout" element={<CheckoutPage/>}/><Route path="/assistant" element={<RecommendationChatPage/>}/>
    <Route path="/checkout/success" element={<PaymentReturnPage/>}/>
    <Route path="/login" element={<AuthPage mode="login"/>}/><Route path="/register" element={<AuthPage mode="register"/>}/><Route path="/account/orders" element={<OrderHistoryPage/>}/>
    <Route path="/admin" element={<AdminRoute section=""/>}/>{['catalog', 'orders', 'transactions', 'inventory', 'accounts', 'audit'].map(section => <Route key={section} path={`/admin/${section}`} element={<AdminRoute section={section}/>}/>)}
    <Route path="*" element={<Navigate to="/" replace/>}/>
  </Routes></Suspense><Footer/>{!hideChat && <ChatBubble/>}</>
}
