package vnpay

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	commandPay    = "pay"
	version       = "2.1.0"
	currency      = "VND"
	expiresIn     = 15 * time.Minute
	vietnamOffset = 7 * 60 * 60
)

type Config struct {
	TmnCode    string
	HashSecret string
	PaymentURL string
	Now        func() time.Time
}

type Client struct {
	tmnCode    string
	hashSecret string
	paymentURL *url.URL
	now        func() time.Time
}

type CheckoutRequest struct {
	TransactionRef string
	AmountVND      int64
	OrderInfo      string
	ReturnURL      string
	ClientIP       string
}

type Callback struct {
	TransactionRef        string
	ProviderTransactionID string
	AmountVND             int64
	Success               bool
	ResponseCode          string
	TransactionStatus     string
}

func NewClient(config Config) (*Client, error) {
	if strings.TrimSpace(config.TmnCode) == "" {
		return nil, errors.New("VNPAY terminal code is required")
	}
	if strings.TrimSpace(config.HashSecret) == "" {
		return nil, errors.New("VNPAY hash secret is required")
	}
	paymentURL, err := url.ParseRequestURI(config.PaymentURL)
	if err != nil || paymentURL.Scheme != "https" || paymentURL.Host == "" {
		return nil, errors.New("VNPAY payment URL must be an absolute HTTPS URL")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Client{tmnCode: config.TmnCode, hashSecret: config.HashSecret, paymentURL: paymentURL, now: now}, nil
}

func (c *Client) CreateCheckoutURL(request CheckoutRequest) (string, error) {
	if strings.TrimSpace(request.TransactionRef) == "" {
		return "", errors.New("VNPAY transaction reference is required")
	}
	if request.AmountVND <= 0 {
		return "", errors.New("VNPAY amount must be positive")
	}
	if strings.TrimSpace(request.OrderInfo) == "" {
		return "", errors.New("VNPAY order information is required")
	}
	returnURL, err := url.ParseRequestURI(request.ReturnURL)
	if err != nil || (returnURL.Scheme != "http" && returnURL.Scheme != "https") || returnURL.Host == "" {
		return "", errors.New("VNPAY return URL must be an absolute HTTP(S) URL")
	}
	clientIP := net.ParseIP(request.ClientIP)
	if clientIP == nil || clientIP.To4() == nil {
		return "", errors.New("VNPAY requires an IPv4 client address")
	}
	if request.AmountVND > (1<<63-1)/100 {
		return "", errors.New("VNPAY amount is too large")
	}

	// VNPAY interprets vnp_CreateDate/vnp_ExpireDate as Vietnam local time,
	// while containers commonly run with UTC as their system timezone.
	createdAt := c.now().In(time.FixedZone("Asia/Ho_Chi_Minh", vietnamOffset))
	values := url.Values{
		"vnp_Amount":     {strconv.FormatInt(request.AmountVND*100, 10)},
		"vnp_Command":    {commandPay},
		"vnp_CreateDate": {createdAt.Format("20060102150405")},
		"vnp_CurrCode":   {currency},
		"vnp_ExpireDate": {createdAt.Add(expiresIn).Format("20060102150405")},
		"vnp_IpAddr":     {clientIP.To4().String()},
		"vnp_Locale":     {"vn"},
		"vnp_OrderInfo":  {request.OrderInfo},
		"vnp_OrderType":  {"other"},
		"vnp_ReturnUrl":  {returnURL.String()},
		"vnp_TmnCode":    {c.tmnCode},
		"vnp_TxnRef":     {request.TransactionRef},
		"vnp_Version":    {version},
	}
	values.Set("vnp_SecureHash", c.sign(values))

	checkoutURL := *c.paymentURL
	checkoutURL.RawQuery = values.Encode()
	return checkoutURL.String(), nil
}

func (c *Client) VerifyIPN(values url.Values) (*Callback, error) {
	if !hmac.Equal([]byte(values.Get("vnp_SecureHash")), []byte(c.sign(values))) {
		return nil, errors.New("invalid VNPAY checksum")
	}
	if values.Get("vnp_TmnCode") != c.tmnCode {
		return nil, errors.New("unexpected VNPAY terminal code")
	}
	transactionRef := strings.TrimSpace(values.Get("vnp_TxnRef"))
	if transactionRef == "" {
		return nil, errors.New("missing VNPAY transaction reference")
	}
	amount, err := strconv.ParseInt(values.Get("vnp_Amount"), 10, 64)
	if err != nil || amount <= 0 || amount%100 != 0 {
		return nil, errors.New("invalid VNPAY amount")
	}
	responseCode := values.Get("vnp_ResponseCode")
	transactionStatus := values.Get("vnp_TransactionStatus")
	return &Callback{
		TransactionRef:        transactionRef,
		ProviderTransactionID: values.Get("vnp_TransactionNo"),
		AmountVND:             amount / 100,
		Success:               responseCode == "00" && transactionStatus == "00",
		ResponseCode:          responseCode,
		TransactionStatus:     transactionStatus,
	}, nil
}

func (c *Client) sign(values url.Values) string {
	payload := url.Values{}
	for key, items := range values {
		if key == "vnp_SecureHash" || key == "vnp_SecureHashType" {
			continue
		}
		for _, item := range items {
			payload.Add(key, item)
		}
	}
	mac := hmac.New(sha512.New, []byte(c.hashSecret))
	_, _ = mac.Write([]byte(payload.Encode()))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) String() string {
	return fmt.Sprintf("VNPAY client for terminal %s", c.tmnCode)
}
