package graph

import (
	"bytes"
	"testing"
)

func TestValidateProductMedia(t *testing.T) {
	tests := []struct {
		name, ext, contentType string
		data                   []byte
		wantErr                bool
	}{
		{name: "jpeg", ext: ".jpg", contentType: "image/jpeg", data: []byte{0xff, 0xd8, 0xff, 0x00}},
		{name: "png", ext: ".png", contentType: "image/png", data: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}},
		{name: "webp", ext: ".webp", contentType: "image/webp", data: []byte("RIFFxxxxWEBP")},
		{name: "mismatched extension", ext: ".png", data: []byte{0xff, 0xd8, 0xff}, wantErr: true},
		{name: "non image", ext: ".jpg", data: []byte("<html>no</html>"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotType, err := validateProductMedia(bytes.NewReader(tt.data), tt.ext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateProductMedia() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && gotType != tt.contentType {
				t.Fatalf("content type = %q, want %q", gotType, tt.contentType)
			}
		})
	}
}

func TestValidProductMediaObject(t *testing.T) {
	if !validProductMediaObject("products/550e8400-e29b-41d4-a716-446655440000.png") {
		t.Fatal("expected generated product media object to be accepted")
	}
	for _, object := range []string{"products/../secret.png", "products/not-a-uuid.png", "other/550e8400-e29b-41d4-a716-446655440000.png", "products/550e8400-e29b-41d4-a716-446655440000.html"} {
		if validProductMediaObject(object) {
			t.Errorf("expected object %q to be rejected", object)
		}
	}
}
