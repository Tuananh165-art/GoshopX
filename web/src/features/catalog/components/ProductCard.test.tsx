import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { ProductCard } from './ProductCard'

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
  images: [],
}

describe('ProductCard', () => {
  it('explains an add-to-cart authorization failure and offers login', async () => {
    const user = userEvent.setup()
    const onAdd = vi.fn().mockRejectedValue(new Error('UserId not found in context'))

    render(<MemoryRouter><ProductCard product={product} onAdd={onAdd}/></MemoryRouter>)
    await user.click(screen.getByRole('button', { name: /thêm giỏ/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/đăng nhập để thêm sản phẩm/i)
    expect(screen.getByRole('link', { name: /đăng nhập/i })).toHaveAttribute('href', '/login')
  })
})
