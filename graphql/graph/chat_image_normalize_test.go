package graph

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePNGReencodesAndEnforcesPixelLimit(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	src.Set(0, 0, color.NRGBA{R: 200, A: 255})
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	got, err := normalizeChatImage("image/png", input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(got))
	if err != nil || cfg.Width != 4 || cfg.Height != 3 {
		t.Fatalf("normalized PNG config=%+v err=%v", cfg, err)
	}
	if validChatImageDimensions(5000, 4001) {
		t.Fatal("accepted dimensions above 20 megapixels")
	}
}

func TestNormalizeJPEGAppliesExifOrientationAndStripsMetadata(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 3))
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	oriented := injectJPEGExifOrientation(jpegBytes.Bytes(), 6)
	normalized, err := normalizeChatImage("image/jpeg", oriented)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(normalized))
	if err != nil || cfg.Width != 3 || cfg.Height != 2 {
		t.Fatalf("normalized JPEG config=%+v err=%v", cfg, err)
	}
	if bytes.Contains(normalized, []byte("Exif\x00\x00")) {
		t.Fatal("normalized JPEG retained EXIF metadata")
	}
}

func TestNormalizeWebPDecodesPixelsAndRejectsMalformedImage(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "chat.webp"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := normalizeWebP(input)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(got))
	if err != nil || cfg.Width != 75 || cfg.Height != 100 {
		t.Fatalf("normalized WebP config=%+v err=%v", cfg, err)
	}
	if _, err := normalizeWebP(extendedWebPWithMetadata()); err == nil {
		t.Fatal("accepted WebP with a malformed compressed pixel stream")
	}
	if _, err := normalizeWebP([]byte("RIFFxxxxWEBP")); err == nil {
		t.Fatal("accepted WebP container without image chunks")
	}
}

func injectJPEGExifOrientation(jpegData []byte, orientation uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff, []byte{'I', 'I'})
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 1)
	binary.LittleEndian.PutUint16(tiff[10:12], 0x0112)
	binary.LittleEndian.PutUint16(tiff[12:14], 3)
	binary.LittleEndian.PutUint32(tiff[14:18], 1)
	binary.LittleEndian.PutUint16(tiff[18:20], orientation)
	segment := append([]byte("Exif\x00\x00"), tiff...)
	result := append([]byte(nil), jpegData[:2]...)
	result = append(result, 0xff, 0xe1)
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(segment)+2))
	result = append(result, length...)
	result = append(result, segment...)
	return append(result, jpegData[2:]...)
}

func extendedWebPWithMetadata() []byte {
	var chunks bytes.Buffer
	writeWebPChunk(&chunks, "VP8X", []byte{0x2c, 0, 0, 0, 1, 0, 0, 2, 0, 0})
	writeWebPChunk(&chunks, "ICCP", []byte{1})
	writeWebPChunk(&chunks, "EXIF", []byte("Exif\x00\x00II"))
	writeWebPChunk(&chunks, "XMP ", []byte{2})
	writeWebPChunk(&chunks, "VP8 ", []byte{0, 0, 0, 0x9d, 0x01, 0x2a, 2, 0, 3, 0})
	result := append([]byte("RIFF\x00\x00\x00\x00WEBP"), chunks.Bytes()...)
	binary.LittleEndian.PutUint32(result[4:8], uint32(len(result)-8))
	return result
}

func writeWebPChunk(dst *bytes.Buffer, kind string, payload []byte) {
	_, _ = dst.WriteString(kind)
	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(payload)))
	_, _ = dst.Write(size)
	_, _ = dst.Write(payload)
	if len(payload)%2 != 0 {
		_ = dst.WriteByte(0)
	}
}
