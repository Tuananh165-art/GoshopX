import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProductDetailPage } from './ProductDetailPage'

const product = {
  id: 14,
  title: 'Demo phone',
  description: 'Demo product',
  category: 'smartphones',
  price: 100,
  discountPercentage: 0,
  rating: 4.5,
  stock: 5,
  thumbnail: 'https://example.test/product.jpg',
  images: ['https://example.test/product.jpg'],
}

vi.mock('../../shared/api/catalog', () => ({ getCatalogProduct: vi.fn(), canReviewProduct: vi.fn() }))

beforeEach(async () => {
  const { getCatalogProduct, canReviewProduct } = await import('../../shared/api/catalog')
  vi.mocked(getCatalogProduct).mockResolvedValue(product)
  vi.mocked(canReviewProduct).mockResolvedValue(false)
})

describe('ProductDetailPage', () => {
  it('passes the selected quantity to the cart mutation callback', async () => {
    const user = userEvent.setup()
    const add = vi.fn().mockResolvedValue(undefined)
    render(<MemoryRouter initialEntries={['/products/14']}><Routes><Route path="/products/:id" element={<ProductDetailPage add={add}/>}/></Routes></MemoryRouter>)

    await screen.findByRole('heading', { name: 'Demo phone' })
    await user.click(screen.getByRole('button', { name: '+' }))
    await user.click(screen.getByRole('button', { name: /thêm 2 vào giỏ/i }))

    expect(add).toHaveBeenCalledWith(product, 2)
  })

  it('explains an add-to-cart authorization failure and offers login', async () => {
    const user = userEvent.setup()
    const add = vi.fn().mockRejectedValue(new Error('UserId not found in context'))
    render(<MemoryRouter initialEntries={['/products/14']}><Routes><Route path="/products/:id" element={<ProductDetailPage add={add}/>}/></Routes></MemoryRouter>)

    await screen.findByRole('heading', { name: 'Demo phone' })
    await user.click(screen.getByRole('button', { name: /thêm 1 vào giỏ/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/đăng nhập để thêm sản phẩm/i)
    expect(screen.getByRole('link', { name: /đăng nhập/i })).toHaveAttribute('href', '/login')
  })
})
