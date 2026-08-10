export function salePrice(price: number, discountPercentage = 0) {
  const safePrice = Number.isFinite(price) ? price : 0
  const safeDiscount = Math.min(100, Math.max(0, Number.isFinite(discountPercentage) ? discountPercentage : 0))

  return safePrice * (1 - safeDiscount / 100)
}

export function vnd(value: number) {
  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
    maximumFractionDigits: 0,
  }).format(Number.isFinite(value) ? value : 0)
}
