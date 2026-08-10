import type { Product, ProductResponse } from '../../domain/product'

const baseUrl = import.meta.env.VITE_DUMMYJSON_URL || 'https://dummyjson.com'

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`)
  if (!response.ok) throw new Error('Không thể tải dữ liệu catalogue. Vui lòng thử lại.')
  return response.json() as Promise<T>
}

export const getProducts = (query = '', category = ''): Promise<ProductResponse> => {
  const path = query ? `/products/search?q=${encodeURIComponent(query)}&limit=100` : category ? `/products/category/${encodeURIComponent(category)}?limit=100` : '/products?limit=100'
  return request<ProductResponse>(path)
}

export const getProduct = (id: string): Promise<Product> => request<Product>(`/products/${encodeURIComponent(id)}`)
export const getCategories = (): Promise<string[]> => request<string[]>('/products/category-list')
