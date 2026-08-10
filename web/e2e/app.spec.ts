import { expect, test } from '@playwright/test'

test('storefront loads DummyJSON content and remains usable on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('http://127.0.0.1:5173/', { waitUntil: 'networkidle' })

  await expect(page.getByRole('heading', { name: /khám phá sản phẩm/i })).toBeVisible()
  await expect(page.locator('.product-card').first()).toBeVisible({ timeout: 15_000 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)
  await page.screenshot({ path: 'test-results/storefront-mobile.png', fullPage: true })
})

test('admin navigation renders a safe data-pending state', async ({ page }) => {
  await page.goto('http://127.0.0.1:5173/admin/inventory')

  await expect(page.getByRole('heading', { name: /kho & phiên giữ hàng/i })).toBeVisible()
  await expect(page.getByText(/chưa tải dữ liệu/i)).toBeVisible()
})
