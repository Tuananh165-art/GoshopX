# VNPAY Sandbox payment integration

## Scope

Payment owns checkout amount calculation, transaction persistence, VNPAY checksum validation, idempotency, and business side effects. Kong only forwards the narrow IPN route. GraphQL remains the client contract and must not decide payment success.

Dodo-specific customer portal and automatic refund APIs are intentionally unsupported while VNPAY Sandbox is the selected provider. They must return an explicit unsupported error rather than simulate a successful provider action.

## Required runtime configuration

Never commit the issued secret. Configure these values in the local secret store or deployment environment:

```dotenv
VNPAY_TMN_CODE=<sandbox terminal id>
VNPAY_HASH_SECRET=<sandbox hash secret>
VNPAY_PAYMENT_URL=https://sandbox.vnpayment.vn/paymentv2/vpcpay.html
```

The browser return URL is supplied by the checkout request and must be an approved frontend URL. It is not the IPN endpoint.

## IPN setup

Configure VNPAY Merchant Sandbox with a publicly reachable HTTPS IPN URL:

```text
https://<public-domain>/webhook/payment
```

For local SIT, expose Kong port 8080 with an HTTPS tunnel and use its public HTTPS origin. `localhost` is not reachable by VNPAY. Kong accepts only `GET /webhook/payment` and proxies it to the private Payment HTTP listener.

Payment verifies before mutating state:

1. HMAC-SHA512 checksum over sorted VNPAY query parameters, excluding `vnp_SecureHash` and `vnp_SecureHashType`.
2. Terminal code, transaction reference, exact amount, and pending transaction state.
3. Both `vnp_ResponseCode` and `vnp_TransactionStatus` must equal `00` for a success transition.
4. Duplicate signed IPNs acknowledge success but must not republish events or repeat order/inventory/cart work.

The IPN response is VNPAY JSON (`RspCode` / `Message`), not a GraphQL response.

## Sandbox test flow

1. Start Account, Payment, GraphQL and Kong.
2. Confirm Payment has received product events and can create a signed checkout URL.
3. Open the checkout URL and use only VNPAY-issued sandbox test data.
4. Use the VNPAY SIT tool to deliver a signed IPN to the public URL.
5. Verify exactly one payment transaction and exactly one business transition/event for a repeated callback.

## Rollback

Set the provider-specific runtime configuration back to the prior adapter only after its signed webhook validation has been restored and regression tests pass. Do not change payment state directly in PostgreSQL as a rollback shortcut.
