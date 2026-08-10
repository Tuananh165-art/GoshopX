package vnpay_test

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/url"
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/payment/vnpay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHashSecret = "test-vnpay-hash-secret"

func TestCreateCheckoutURL_SignsRequiredVNPAYParameters(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{
		TmnCode:    "TESTCODE",
		HashSecret: testHashSecret,
		PaymentURL: "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html",
		Now:        func() time.Time { return time.Date(2026, 7, 30, 10, 15, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	checkoutURL, err := client.CreateCheckoutURL(vnpay.CheckoutRequest{
		TransactionRef: "payment-123",
		AmountVND:      125000,
		OrderInfo:      "Thanh toan don hang 42",
		ReturnURL:      "https://shop.example.test/checkout-complete",
		ClientIP:       "203.0.113.10",
	})
	require.NoError(t, err)

	parsed, err := url.Parse(checkoutURL)
	require.NoError(t, err)
	values := parsed.Query()
	assert.Equal(t, "pay", values.Get("vnp_Command"))
	assert.Equal(t, "2.1.0", values.Get("vnp_Version"))
	assert.Equal(t, "TESTCODE", values.Get("vnp_TmnCode"))
	assert.Equal(t, "12500000", values.Get("vnp_Amount"))
	assert.Equal(t, "payment-123", values.Get("vnp_TxnRef"))
	assert.Equal(t, "20260730171500", values.Get("vnp_CreateDate"))
	assert.Equal(t, "20260730173000", values.Get("vnp_ExpireDate"))
	assert.Equal(t, "203.0.113.10", values.Get("vnp_IpAddr"))
	assert.Equal(t, sign(testHashSecret, values), values.Get("vnp_SecureHash"))
}

func TestVerifyIPN_RejectsTamperedAmount(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: "TESTCODE", HashSecret: testHashSecret, PaymentURL: "https://example.test/pay"})
	require.NoError(t, err)

	values := url.Values{
		"vnp_Amount":            {"12500000"},
		"vnp_ResponseCode":      {"00"},
		"vnp_TransactionStatus": {"00"},
		"vnp_TmnCode":           {"TESTCODE"},
		"vnp_TxnRef":            {"payment-123"},
		"vnp_TransactionNo":     {"14000001"},
	}
	values.Set("vnp_SecureHash", sign(testHashSecret, values))

	callback, err := client.VerifyIPN(values)
	require.NoError(t, err)
	assert.True(t, callback.Success)
	assert.Equal(t, "payment-123", callback.TransactionRef)
	assert.Equal(t, int64(125000), callback.AmountVND)
	assert.Equal(t, "14000001", callback.ProviderTransactionID)

	values.Set("vnp_Amount", "12500100")
	_, err = client.VerifyIPN(values)
	assert.Error(t, err)
}

func sign(secret string, values url.Values) string {
	payload := url.Values{}
	for key, items := range values {
		if key == "vnp_SecureHash" || key == "vnp_SecureHashType" {
			continue
		}
		for _, item := range items {
			payload.Add(key, item)
		}
	}
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write([]byte(payload.Encode()))
	return hex.EncodeToString(mac.Sum(nil))
}
