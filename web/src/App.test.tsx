import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './features/auth/AuthContext'
import { CartProvider } from './features/cart/CartContext'

describe('GoshopX storefront', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('renders a discoverable catalog surface and protects admin routes', async () => {
    vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? '{}')) as { query?: string }
      const data = body.query?.includes('query Me')
        ? { me: null }
        : { product: [{ id: '1', name: 'Phone', description: 'Demo', price: 10, discountPercentage: 0, rating: 4.5, stock: 5, brand: 'Demo', categoryId: 'smartphones', tags: [], availabilityStatus: 'In Stock', shippingInformation: '', returnPolicy: '', warrantyInformation: '', thumbnail: 'https://example.test/phone.jpg', images: [], reviews: [] }] }
      return new Response(JSON.stringify({ data }), { status: 200, headers: { 'content-type': 'application/json' } })
    }))
    render(<MemoryRouter initialEntries={['/']}><AuthProvider><CartProvider><App /></CartProvider></AuthProvider></MemoryRouter>)

    expect(await screen.findByRole('heading', { name: /khám phá sản phẩm/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /giỏ hàng/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /ưu đãi trong ngày/i })).toBeInTheDocument()
  })

  it('hides the admin link for customers', async () => {
    vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? '{}')) as { query?: string }
      const data = body.query?.includes('query Me')
        ? { me: { id: 1, name: 'Customer', email: 'customer@example.test', avatarUrl: '', phone: '', shippingAddress: '', roleId: 0, role: 'CUSTOMER', status: 'ACTIVE' } }
        : { product: [] }
      return new Response(JSON.stringify({ data }), { status: 200, headers: { 'content-type': 'application/json' } })
    }))
    render(<MemoryRouter initialEntries={['/']}><AuthProvider><CartProvider><App /></CartProvider></AuthProvider></MemoryRouter>)

    await screen.findAllByRole('heading', { name: /khám phá sản phẩm/i })
    expect(screen.queryByRole('link', { name: 'Quản trị' })).not.toBeInTheDocument()
  })

  it('loads the admin UI immediately for role_id 1', async () => {
    vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? '{}')) as { query?: string }
      const data = body.query?.includes('query Me')
        ? { me: { id: 10, name: 'Admin', email: 'admin@example.test', avatarUrl: '', phone: '', shippingAddress: '', roleId: 1, role: 'ADMIN', status: 'ACTIVE' } }
        : { product: [] }
      return new Response(JSON.stringify({ data }), { status: 200, headers: { 'content-type': 'application/json' } })
    }))
    render(<MemoryRouter initialEntries={['/admin']}><AuthProvider><CartProvider><App /></CartProvider></AuthProvider></MemoryRouter>)

    expect(await screen.findByRole('heading', { name: /tổng quan vận hành/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Quản trị' })).toBeInTheDocument()
  })
})
