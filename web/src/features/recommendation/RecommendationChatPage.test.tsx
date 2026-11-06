import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'

const { graphql } = vi.hoisted(() => ({ graphql: vi.fn() }))
vi.mock('../../shared/api/graphql', () => ({ graphql, graphqlUpload: vi.fn() }))

import { RecommendationChatPage } from './RecommendationChatPage'

afterEach(() => cleanup())

describe('RecommendationChatPage history deletion', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.localStorage.setItem('goshopx-recommendation-chat-session', 'session-1')
    window.localStorage.setItem('goshopx-recommendation-chat-history', JSON.stringify([
      { role: 'user', text: 'Tìm laptop' },
    ]))
    graphql.mockReset()
  })

  it('deletes the server session before clearing local history', async () => {
    graphql.mockResolvedValue({ deleteChatSession: true })
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    const clearButton = await screen.findByRole('button', { name: /lịch sử/i })
    await waitFor(() => expect(clearButton).toBeEnabled())
    fireEvent.click(clearButton)

    await waitFor(() => expect(window.localStorage.getItem('goshopx-recommendation-chat-history')).toBeNull())
    expect(graphql).toHaveBeenCalledWith(expect.stringContaining('deleteChatSession'), { sessionId: 'session-1' })
    expect(window.localStorage.getItem('goshopx-recommendation-chat-session')).not.toBe('session-1')
  })

  it('keeps local history when the server refuses deletion', async () => {
    graphql.mockResolvedValue({ deleteChatSession: false })
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    fireEvent.click(await screen.findByRole('button', { name: /lịch sử/i }))
    await screen.findByText(/chưa xác nhận/i)
    expect(window.localStorage.getItem('goshopx-recommendation-chat-history')).not.toBeNull()
  })

  it('restores an unexpired server cart proposal after a page refresh', async () => {
    const request = { accountId: 7, expectedCartVersion: '4', idempotencyKey: 'proposal-key', items: [{ productId: 'p-1', quantity: 1 }] }
    window.localStorage.setItem('goshopx-recommendation-chat-cart-proposal', JSON.stringify(request))
    graphql.mockResolvedValueOnce({ myCart: { accountId: 7 } }).mockResolvedValueOnce({ createCartProposal: {
      id: 'proposal-1', baseVersion: '4', expiresAt: new Date(Date.now() + 60_000).toISOString(), items: request.items,
    } })
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    expect(await screen.findByTestId('cart-proposal-preview')).toBeInTheDocument()
    expect(graphql).toHaveBeenNthCalledWith(2, expect.stringContaining('createCartProposal'), request)
  })

  it('confirms a cart proposal, requests an Order quote, and carries the coupon into checkout', async () => {
    const request = { accountId: 7, expectedCartVersion: '4', idempotencyKey: 'proposal-key', items: [{ productId: 'p-1', quantity: 1 }] }
    const proposal = { id: 'proposal-1', baseVersion: '4', expiresAt: new Date(Date.now() + 60_000).toISOString(), items: request.items }
    graphql.mockResolvedValueOnce({ myCart: { accountId: 7 } })
      .mockResolvedValueOnce({ createCartProposal: proposal })
      .mockResolvedValueOnce({ confirmCartProposal: { version: '5', items: [] } })
      .mockResolvedValueOnce({ previewCartCheckout: {
        id: 'quote-1', subtotalVnd: '2000000', discountVnd: '100000', totalPriceVnd: '1900000',
        couponCode: 'LAB10', expiresAt: new Date(Date.now() + 60_000).toISOString(), cartVersion: '5',
      } })
    window.localStorage.setItem('goshopx-recommendation-chat-cart-proposal', JSON.stringify(request))
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    await screen.findByTestId('cart-proposal-preview')
    fireEvent.change(screen.getByRole('textbox', { name: /mã giảm giá tùy chọn/i }), { target: { value: 'LAB10' } })
    fireEvent.click(screen.getByRole('button', { name: /xác nhận và xem báo giá/i }))

    const checkout = await screen.findByRole('link', { name: /tiếp tục checkout/i })
    expect(checkout).toHaveAttribute('href', '/checkout?coupon=LAB10')
    expect(graphql).toHaveBeenNthCalledWith(4, expect.stringContaining('previewCartCheckout'), { couponCode: 'LAB10' })
    expect(await screen.findByText(/1\.900\.000/)).toBeInTheDocument()
  })

  it('does not replay a stored proposal into another signed-in account', async () => {
    window.localStorage.setItem('goshopx-recommendation-chat-cart-proposal', JSON.stringify({
      accountId: 7, expectedCartVersion: '4', idempotencyKey: 'proposal-key', items: [{ productId: 'p-1', quantity: 1 }],
    }))
    graphql.mockResolvedValueOnce({ myCart: { accountId: 99 } })
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    await waitFor(() => expect(graphql).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(window.localStorage.getItem('goshopx-recommendation-chat-cart-proposal')).toBeNull())
  })

  it('renders internal knowledge citations as labels and external sources as links', async () => {
    graphql.mockResolvedValue({ recommendChat: {
      answer: 'Theo chính sách đã xuất bản.', products: [], nextSuggestions: [], warnings: [],
      requestId: 'req-1', sessionId: 'session-1', sources: [
        { title: 'Tài liệu: Chính sách đổi trả', url: '', publishedAt: '' },
        { title: 'Tiêu chí chọn laptop', url: 'https://example.com/guide', publishedAt: '' },
      ],
    } })
    render(<MemoryRouter><RecommendationChatPage /></MemoryRouter>)

    fireEvent.change(screen.getByRole('textbox', { name: /câu hỏi sản phẩm/i }), { target: { value: 'Chính sách đổi trả?' } })
    fireEvent.click(screen.getByRole('button', { name: /gửi câu hỏi/i }))

    expect(await screen.findByText('Tài liệu: Chính sách đổi trả')).toBeInTheDocument()
    expect(screen.getByText('Tài liệu: Chính sách đổi trả').closest('a')).toBeNull()
    expect(screen.getByRole('link', { name: /tiêu chí chọn laptop/i })).toHaveAttribute('href', 'https://example.com/guide')
  })
})
