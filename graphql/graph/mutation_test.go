package graph

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
)

func TestGuestChatCookieIsOwnerBoundAndTamperEvident(t *testing.T) {
	secret := "test-signing-key"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("guest-id"))
	value := "guest-id." + hex.EncodeToString(mac.Sum(nil))
	if id, ok := verifyGuestChatCookie(value, secret); !ok || id != "guest-id" {
		t.Fatal("valid signed guest cookie was rejected")
	}
	if _, ok := verifyGuestChatCookie(value+"x", secret); ok {
		t.Fatal("tampered guest cookie was accepted")
	}
}

func TestCODCheckoutRemainsPendingUntilCollection(t *testing.T) {
	if codCheckoutStatus != "pending" {
		t.Fatalf("COD order status = %q, want pending", codCheckoutStatus)
	}
}

func TestEncodeChatImageCreatesDataURL(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	payload, err := encodeChatImage(graphql.Upload{
		File:        bytes.NewReader(data.Bytes()),
		Filename:    "product.png",
		Size:        int64(data.Len()),
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL prefix: %q", payload[:min(len(payload), 32)])
	}
	encoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(payload, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(encoded))
	if err != nil || config.Width != 2 || config.Height != 3 {
		t.Fatalf("normalized image config = %+v, error = %v", config, err)
	}
}

func TestEncodeChatImageRejectsMismatchedContent(t *testing.T) {
	_, err := encodeChatImage(graphql.Upload{
		File:        bytes.NewReader([]byte("not an image")),
		Filename:    "product.png",
		Size:        12,
		ContentType: "image/png",
	})
	if err == nil {
		t.Fatal("expected image bytes to be validated")
	}
}

func TestEncodeChatImageRejectsUnsupportedType(t *testing.T) {
	_, err := encodeChatImage(graphql.Upload{
		File:        bytes.NewReader([]byte("not-image")),
		Filename:    "product.gif",
		Size:        9,
		ContentType: "image/gif",
	})
	if err == nil {
		t.Fatal("expected unsupported image type to be rejected")
	}
}
