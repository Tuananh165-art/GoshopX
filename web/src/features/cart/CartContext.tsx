import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { CartLine, Product } from '../../domain/product'
import { graphql } from '../../shared/api/graphql'
import { useAuth } from '../auth/AuthContext'

type CartContextValue = {
  cart: CartLine[]
  expiresAt: string | null
  loading: boolean
  error: string
  add: (product: Product, quantity?: number) => Promise<void>
  update: (productId: string, quantity: number) => Promise<void>
  remove: (productId: string) => Promise<void>
  clear: () => Promise<void>
  checkout: (redirectUrl: string) => Promise<void>
  checkoutCOD: () => Promise<number>
  refresh: () => Promise<void>
}

const CartContext = createContext<CartContextValue | null>(null)
const cartFields = `accountId expiresAt items { quantity reservationId reservedUntil product { id name description price discountPercentage rating stock brand thumbnail images tags availabilityStatus shippingInformation returnPolicy warrantyInformation reviews { rating comment date reviewerName reviewerEmail } } }`

type ServerProduct = Product & { name?: string; categoryId?: string; id: number | string }
type ServerCartValue = { expiresAt: string; items: Array<{ quantity: number; reservedUntil: string; product: ServerProduct }> }
type ServerCart = { myCart: ServerCartValue }

type CartMutationResult = { cart?: ServerCartValue }

const toLines = (value: ServerCartValue): CartLine[] => value.items.map(item => ({
  ...item.product,
  id: Number(item.product.id),
  title: item.product.title || item.product.name || 'Sản phẩm',
  category: item.product.category || item.product.categoryId || '',
  quantity: item.quantity,
  reservedUntil: item.reservedUntil,
}))

export function CartProvider({ children }: { children: ReactNode }) {
  const { authenticated, pending: authPending } = useAuth()
  const [cart, setCart] = useState<CartLine[]>([])
  const [expiresAt, setExpiresAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const result = await graphql<ServerCart>(`query MyCart { myCart { ${cartFields} } }`)
      setCart(toLines(result.myCart))
      setExpiresAt(result.myCart.expiresAt || null)
      setError('')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Không thể tải giỏ hàng.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (authPending) return
    if (authenticated) void refresh()
    else {
      setCart([])
      setExpiresAt(null)
    }
  }, [authPending, authenticated, refresh])

  const mutate = async (query: string, variables: Record<string, unknown>) => {
    setLoading(true)
    try {
      const result = await graphql<CartMutationResult>(query, variables)
      if (result.cart) {
        setCart(toLines(result.cart))
        setExpiresAt(result.cart.expiresAt || null)
      }
      setError('')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Không thể cập nhật giỏ hàng.')
      throw cause
    } finally {
      setLoading(false)
    }
  }

  const value = useMemo<CartContextValue>(() => ({
    cart,
    expiresAt,
    loading,
    error,
    refresh,
    add: (product, quantity = 1) => mutate(
      `mutation AddCart($productId: String!, $quantity: Int!) { cart: addCartItem(productId: $productId, quantity: $quantity) { ${cartFields} } }`,
      { productId: String(product.id), quantity },
    ),
    update: (productId, quantity) => mutate(
      `mutation UpdateCart($productId: String!, $quantity: Int!) { cart: updateCartItemQuantity(productId: $productId, quantity: $quantity) { ${cartFields} } }`,
      { productId, quantity },
    ),
    remove: productId => mutate(
      `mutation RemoveCart($productId: String!) { cart: removeCartItem(productId: $productId) { ${cartFields} } }`,
      { productId },
    ),
    clear: async () => {
      setLoading(true)
      try {
        await graphql<{ clearCart: boolean }>('mutation ClearCart { clearCart }')
        setCart([])
        setExpiresAt(null)
        setError('')
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : 'Không thể xóa giỏ hàng.')
        throw cause
      } finally {
        setLoading(false)
      }
    },
    checkout: async redirectUrl => {
      setLoading(true)
      try {
        const result = await graphql<{ checkoutCart: { url: string } }>(
          'mutation Checkout($redirectUrl: String!) { checkoutCart(redirectUrl: $redirectUrl) { url } }',
          { redirectUrl },
        )
        setError('')
        window.location.assign(result.checkoutCart.url)
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : 'Không thể bắt đầu thanh toán. Vui lòng thử lại.')
        throw cause
      } finally {
        setLoading(false)
      }
    },
    checkoutCOD: async () => {
      setLoading(true)
      try {
        const result = await graphql<{ checkoutCartCOD: { orderId: number } }>(
          'mutation CheckoutCOD { checkoutCartCOD { orderId status } }',
        )
        setCart([])
        setExpiresAt(null)
        setError('')
        return result.checkoutCartCOD.orderId
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : 'Không thể đặt hàng COD. Vui lòng thử lại.')
        throw cause
      } finally {
        setLoading(false)
      }
    },
  }), [cart, expiresAt, loading, error, refresh])

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>
}

export function useCart() {
  const value = useContext(CartContext)
  if (!value) throw new Error('useCart must be used inside CartProvider')
  return value
}
