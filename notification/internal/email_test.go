package internal

import (
	"strings"
	"testing"
)

func TestComposeEmailCreatesVietnameseHTMLAlternative(t *testing.T) {
	message := composeEmail("shop@example.com", "customer@example.com", "Cập nhật đơn hàng", "Tổng tiền: 34.999.750 ₫")
	for _, expected := range []string{
		"Content-Type: multipart/alternative",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"GOSHOP",
		"Tổng tiền: 34.999.750 ₫",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("email message does not contain %q", expected)
		}
	}
}
