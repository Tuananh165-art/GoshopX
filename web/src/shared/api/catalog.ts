import type { Product } from '../../domain/product'
import { graphql } from './graphql'

const fields = `id name description price discountPercentage rating stock brand categoryId tags availabilityStatus shippingInformation returnPolicy warrantyInformation thumbnail images reviews { rating comment date reviewerName reviewerEmail }`
type GraphProduct = Omit<Product, 'title' | 'category'> & { name: string; categoryId: string }
const mapProduct = (product: GraphProduct): Product => ({ ...product, title: product.name, category: product.categoryId })

export async function getCatalog(query = '', category = ''): Promise<Product[]> {
  const result = await graphql<{ product: GraphProduct[] }>(`query Catalog($query: String, $category: String) { product(pagination: { skip: 0, take: 100 }, query: $query, category: $category) { ${fields} } }`, { query: query || null, category: category || null })
  return result.product.map(mapProduct)
}
export async function getCatalogProduct(id: string): Promise<Product> {
  const result = await graphql<{ product: GraphProduct[] }>(`query Product($id: String!) { product(id: $id) { ${fields} } }`, { id })
  if (!result.product[0]) throw new Error('Sản phẩm không tồn tại hoặc đã bị gỡ.')
  return mapProduct(result.product[0])
}
export async function addProductReview(productId: string, rating: number, comment: string): Promise<Product> {
  const result = await graphql<{ addProductReview: GraphProduct }>(`mutation AddReview($productId: String!, $rating: Int!, $comment: String!) { addProductReview(productId: $productId, rating: $rating, comment: $comment) { ${fields} } }`, { productId, rating, comment })
  return mapProduct(result.addProductReview)
}
export async function canReviewProduct(productId: string): Promise<boolean> {
  const result = await graphql<{ canReviewProduct: boolean }>(`query CanReview($productId: String!) { canReviewProduct(productId: $productId) }`, { productId })
  return result.canReviewProduct
}

export async function getCatalogCategories(): Promise<string[]> {
  const result = await graphql<{ categories: Array<{ slug: string }> }>('query Categories { categories { slug } }')
  return result.categories.map(category => category.slug)
}
