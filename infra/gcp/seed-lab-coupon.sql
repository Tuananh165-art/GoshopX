-- Run once after Order migration 002_coupon is applied.
-- Re-running updates the lab coupon's rules/expiry, but does not reset redemptions.
INSERT INTO coupons (
    code,
    percent_basis_point,
    fixed_amount_vnd,
    minimum_subtotal_vnd,
    maximum_discount_vnd,
    maximum_redemptions,
    per_account_limit,
    starts_at,
    expires_at,
    active
) VALUES (
    'LAB10',
    1000,
    0,
    1000000,
    100000,
    50,
    1,
    NOW(),
    NOW() + INTERVAL '48 hours',
    TRUE
)
ON CONFLICT (code) DO UPDATE SET
    percent_basis_point = EXCLUDED.percent_basis_point,
    fixed_amount_vnd = EXCLUDED.fixed_amount_vnd,
    minimum_subtotal_vnd = EXCLUDED.minimum_subtotal_vnd,
    maximum_discount_vnd = EXCLUDED.maximum_discount_vnd,
    maximum_redemptions = EXCLUDED.maximum_redemptions,
    per_account_limit = EXCLUDED.per_account_limit,
    starts_at = EXCLUDED.starts_at,
    expires_at = EXCLUDED.expires_at,
    active = EXCLUDED.active;
