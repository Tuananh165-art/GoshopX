package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
)

func TestCODPaymentStatusIsImmediatelyPaid(t *testing.T) {
	if codPaymentStatus != "paid" {
		t.Fatalf("COD payment status = %q, want paid", codPaymentStatus)
	}
}

func TestEncodeChatImageCreatesDataURL(t *testing.T) {
	payload, err := encodeChatImage(graphql.Upload{
		File:        bytes.NewReader([]byte("image-bytes")),
		Filename:    "product.png",
		Size:        int64(len("image-bytes")),
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL prefix: %q", payload[:min(len(payload), 32)])
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
