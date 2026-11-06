package graph

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/99designs/gqlgen/graphql"
)

//go:embed testdata/chat.webp
var validChatWebP []byte

func TestValidateProductMedia(t *testing.T) {
	tests := []struct {
		name, ext, contentType string
		data                   []byte
		wantErr                bool
	}{
		{name: "jpeg", ext: ".jpg", contentType: "image/jpeg", data: []byte{0xff, 0xd8, 0xff, 0x00}},
		{name: "png", ext: ".png", contentType: "image/png", data: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}},
		{name: "webp", ext: ".webp", contentType: "image/webp", data: testWebP()},
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

func TestSupportedKnowledgeDocumentFormats(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{{"catalog.docx", true}, {"policy.TXT", true}, {"notes.md", true}, {"items.json", true}, {"items.csv", true}, {"manual.pdf", true}, {"archive.zip", false}} {
		if got := isSupportedKnowledgeDocument(test.name); got != test.want {
			t.Errorf("isSupportedKnowledgeDocument(%q) = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestEncodeChatImageValidatesWebPBytes(t *testing.T) {
	webp := testWebP()
	encoded, err := encodeChatImage(graphql.Upload{File: bytes.NewReader(webp), Size: int64(len(webp)), ContentType: "image/webp"})
	if err != nil || len(encoded) == 0 {
		t.Fatalf("webp upload failed: encoded=%q err=%v", encoded, err)
	}
	if _, err := encodeChatImage(graphql.Upload{File: bytes.NewReader([]byte("<html>")), Size: 6, ContentType: "image/webp"}); err == nil {
		t.Fatal("expected non-image bytes to be rejected")
	}
}

func testWebP() []byte {
	return append([]byte(nil), validChatWebP...)
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

func TestKnowledgeDocumentUploadBytesMatchExtension(t *testing.T) {
	if err := validateKnowledgeDocumentBytes(".pdf", []byte("%PDF-1.7")); err != nil {
		t.Fatalf("valid PDF signature rejected: %v", err)
	}
	if err := validateKnowledgeDocumentBytes(".docx", []byte{'P', 'K', 3, 4}); err != nil {
		t.Fatalf("valid DOCX container signature rejected: %v", err)
	}
	if err := validateKnowledgeDocumentBytes(".json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("valid UTF-8 JSON rejected: %v", err)
	}
	for _, tc := range []struct {
		ext  string
		data []byte
	}{
		{".pdf", []byte("plain text")}, {".docx", []byte("plain text")}, {".txt", []byte{'o', 'k', 0}},
	} {
		if err := validateKnowledgeDocumentBytes(tc.ext, tc.data); err == nil {
			t.Errorf("accepted invalid %s bytes", tc.ext)
		}
	}
}
