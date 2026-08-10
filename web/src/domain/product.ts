export type Review = { rating: number; comment: string; date?: string; reviewerName: string; reviewerEmail?: string }

export type Product = {
  id: number
  title: string
  description: string
  category: string
  price: number
  discountPercentage: number
  rating: number
  stock: number
  brand?: string
  thumbnail: string
  images: string[]
  tags?: string[]
  availabilityStatus?: string
  shippingInformation?: string
  returnPolicy?: string
  warrantyInformation?: string
  reviews?: Review[]
}

export type CartLine = Product & { quantity: number; reservedUntil?: string }
export type ProductResponse = { products: Product[]; total: number; skip: number; limit: number }
