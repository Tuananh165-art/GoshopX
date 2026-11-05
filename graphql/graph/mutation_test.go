package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
)

func TestCODCheckoutRemainsPendingUntilCollection(t *testing.T) {
	if codCheckoutStatus != "pending" {
		t.Fatalf("COD order status = %q, want pending", codCheckoutStatus)
	}
}

func TestEncodeChatImageCreatesDataURL(t *testing.T) {
	image := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	payload, err := encodeChatImage(graphql.Upload{
		File:        bytes.NewReader(image),
		Filename:    "product.png",
		Size:        int64(len(image)),
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL prefix: %q", payload[:min(len(payload), 32)])
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
