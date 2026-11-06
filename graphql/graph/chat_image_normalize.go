package graph

import (
	"bytes"
	"encoding/binary"
	"errors"
	"golang.org/x/image/webp"
	"image"
	"image/jpeg"
	"image/png"
)

const maxChatImagePixels int64 = 20_000_000

func normalizeChatImage(contentType string, data []byte) ([]byte, error) {
	switch contentType {
	case "image/jpeg":
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || !validChatImageDimensions(cfg.Width, cfg.Height) {
			return nil, errors.New("chat image dimensions are invalid or exceed 20 megapixels")
		}
		orientation := jpegExifOrientation(data)
		if orientation < 1 || orientation > 8 {
			return nil, errors.New("chat image orientation metadata is invalid")
		}
		decoded, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, errors.New("chat image cannot be decoded")
		}
		decoded = orientImage(decoded, orientation)
		var output bytes.Buffer
		if err := jpeg.Encode(&output, decoded, &jpeg.Options{Quality: 90}); err != nil {
			return nil, errors.New("chat image could not be normalized")
		}
		return output.Bytes(), nil
	case "image/png":
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || !validChatImageDimensions(cfg.Width, cfg.Height) {
			return nil, errors.New("chat image dimensions are invalid or exceed 20 megapixels")
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, errors.New("chat image cannot be decoded")
		}
		var output bytes.Buffer
		if err := png.Encode(&output, decoded); err != nil {
			return nil, errors.New("chat image could not be normalized")
		}
		return output.Bytes(), nil
	case "image/webp":
		return normalizeWebP(data)
	default:
		return nil, errors.New("chat image type is unsupported")
	}
}

func validChatImageDimensions(width, height int) bool {
	return width > 0 && height > 0 && int64(width) <= maxChatImagePixels/int64(height)
}

func jpegExifOrientation(data []byte) int {
	for offset := 2; offset+4 <= len(data) && data[offset] == 0xff; {
		marker := data[offset+1]
		offset += 2
		if marker == 0xd8 || marker == 0xd9 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if offset+2 > len(data) {
			break
		}
		length := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if length < 2 || offset+length > len(data) {
			break
		}
		segment := data[offset+2 : offset+length]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			if orientation := tiffOrientation(segment[6:]); orientation != 0 {
				return orientation
			}
		}
		if marker == 0xda {
			break
		}
		offset += length
	}
	return 1
}

func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 0
	}
	order := binary.ByteOrder(binary.LittleEndian)
	switch string(data[:2]) {
	case "II":
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(data[2:4]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(data[4:8]))
	if ifd < 0 || ifd+2 > len(data) {
		return 0
	}
	count := int(order.Uint16(data[ifd : ifd+2]))
	if count > (len(data)-ifd-2)/12 {
		return 0
	}
	for i := 0; i < count; i++ {
		entry := data[ifd+2+i*12 : ifd+14+i*12]
		if order.Uint16(entry[:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			return int(order.Uint16(entry[8:10]))
		}
	}
	return 0
}

func orientImage(src image.Image, orientation int) image.Image {
	if orientation == 1 {
		return src
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return dst
}

func normalizeWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" || int(binary.LittleEndian.Uint32(data[4:8]))+8 != len(data) {
		return nil, errors.New("chat WebP container is invalid")
	}
	orientation := 1
	offset := 12
	for offset+8 <= len(data) {
		kind := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		end := offset + 8 + size
		if end < offset+8 || end > len(data) {
			return nil, errors.New("chat WebP chunk is malformed")
		}
		payload := data[offset+8 : end]
		switch kind {
		case "VP8X":
			if len(payload) < 10 {
				return nil, errors.New("chat WebP extended header is malformed")
			}
			if payload[0]&0x02 != 0 {
				return nil, errors.New("animated WebP chat images are not supported")
			}
		case "ANIM", "ANMF":
			return nil, errors.New("animated WebP chat images are not supported")
		case "EXIF":
			exif := bytes.TrimPrefix(payload, []byte("Exif\x00\x00"))
			if value := tiffOrientation(exif); value != 0 {
				orientation = value
			}
		}
		offset = end + size%2
		if offset > len(data) {
			return nil, errors.New("chat WebP padding is malformed")
		}
	}
	if offset != len(data) {
		return nil, errors.New("chat WebP chunk is malformed")
	}
	if orientation != 1 {
		return nil, errors.New("WebP image has a non-default orientation; re-save it upright before upload")
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validChatImageDimensions(cfg.Width, cfg.Height) {
		return nil, errors.New("chat WebP dimensions are invalid or exceed 20 megapixels")
	}
	decoded, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("chat WebP pixels cannot be decoded")
	}
	var output bytes.Buffer
	if err := png.Encode(&output, decoded); err != nil {
		return nil, errors.New("chat WebP could not be normalized")
	}
	return output.Bytes(), nil
}
