package graph

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/Tuananh165art/GoshopX/graphql/config"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	gcstorage "google.golang.org/api/storage/v1"
)

const maxProductMediaBytes = 10 << 20
const maxKnowledgeDocumentBytes = 20 << 20

func isSupportedKnowledgeDocument(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".docx", ".txt", ".md", ".json", ".csv":
		return true
	default:
		return false
	}
}

type mediaStore struct {
	client    *minio.Client
	gcs       *gcstorage.Service
	bucket    string
	publicURL string
}

func newMediaStore() (*mediaStore, error) {
	if config.GCSMediaBucket != "" {
		client, err := gcstorage.NewService(context.Background())
		if err != nil {
			return nil, err
		}
		return &mediaStore{gcs: client, bucket: config.GCSMediaBucket}, nil
	}
	if config.MinIOEndpoint == "" || config.MinIOAccessKey == "" || config.MinIOSecretKey == "" || config.MinIOBucket == "" {
		return nil, nil
	}
	client, err := minio.New(config.MinIOEndpoint, &minio.Options{Creds: credentials.NewStaticV4(config.MinIOAccessKey, config.MinIOSecretKey, ""), Secure: strings.EqualFold(config.MinIOUseSSL, "true")})
	if err != nil {
		return nil, err
	}
	return &mediaStore{client: client, bucket: config.MinIOBucket, publicURL: strings.TrimRight(config.MinIOPublicURL, "/")}, nil
}

func (s *mediaStore) uploadProductMedia(ctx context.Context, productID string, upload graphql.Upload) (string, string, error) {
	if upload.File == nil || upload.Size <= 0 || upload.Size > maxProductMediaBytes {
		return "", "", errors.New("media file must be between 1 byte and 10 MiB")
	}
	ext := strings.ToLower(filepath.Ext(upload.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		return "", "", errors.New("only jpg, jpeg, png, and webp media is supported")
	}
	reader, contentType, err := validateProductMedia(upload.File, ext)
	if err != nil {
		return "", "", err
	}
	if s.gcs != nil {
		objectName := fmt.Sprintf("products/%s%s", uuid.NewString(), ext)
		data, err := io.ReadAll(io.LimitReader(reader, maxProductMediaBytes+1))
		if err != nil {
			return "", "", err
		}
		if int64(len(data)) != upload.Size || len(data) > maxProductMediaBytes {
			return "", "", errors.New("media size does not match the declared size")
		}
		request := s.gcs.Objects.Insert(s.bucket, &gcstorage.Object{Name: objectName, ContentType: contentType}).Media(bytes.NewReader(data)).Context(ctx)
		if _, err := request.Do(); err != nil {
			return "", "", err
		}
		checksum := sha256.Sum256(data)
		return "/media/" + objectName, hex.EncodeToString(checksum[:]), nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		exists, existsErr := s.client.BucketExists(ctx, s.bucket)
		if existsErr != nil || !exists {
			return "", "", err
		}
	}
	hash := sha256.New()
	reader = io.TeeReader(io.LimitReader(reader, maxProductMediaBytes+1), hash)
	objectName := fmt.Sprintf("products/%s/%s%s", productID, uuid.NewString(), ext)
	_, err = s.client.PutObject(ctx, s.bucket, objectName, reader, upload.Size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", "", err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if s.publicURL == "" {
		return objectName, checksum, nil
	}
	return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, objectName), checksum, nil
}

func (s *mediaStore) storeChatImage(ctx context.Context, dataURL string) (string, error) {
	if s == nil || s.gcs == nil {
		return "", errors.New("temporary chat image storage is unavailable")
	}
	metadata, encoded, ok := strings.Cut(dataURL, ",")
	if !ok {
		return "", errors.New("chat image payload is invalid")
	}
	contentType := strings.TrimPrefix(strings.TrimSuffix(metadata, ";base64"), "data:")
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[contentType]
	if extension == "" {
		return "", errors.New("chat image type is unsupported")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > maxChatImageBytes {
		return "", errors.New("chat image payload is invalid or exceeds 5 MiB")
	}
	if _, detectedType, err := validateProductMedia(bytes.NewReader(data), extension); err != nil || detectedType != contentType {
		return "", errors.New("chat image bytes do not match the declared image type")
	}
	objectName := fmt.Sprintf("chat-upload/%s%s", uuid.NewString(), extension)
	_, err = s.gcs.Objects.Insert(s.bucket, &gcstorage.Object{Name: objectName, ContentType: contentType}).Media(bytes.NewReader(data)).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return "gs://" + s.bucket + "/" + objectName, nil
}

func (s *mediaStore) uploadKnowledgeDocument(ctx context.Context, upload graphql.Upload) (string, string, string, error) {
	if s == nil || s.gcs == nil {
		return "", "", "", errors.New("private GCS document storage is not configured")
	}
	if upload.File == nil || upload.Size <= 0 || upload.Size > maxKnowledgeDocumentBytes {
		return "", "", "", errors.New("document must be between 1 byte and 20 MiB")
	}
	ext := strings.ToLower(filepath.Ext(upload.Filename))
	allowedTypes := map[string]map[string]bool{
		".pdf": {"application/pdf": true}, ".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true},
		".txt": {"text/plain": true}, ".md": {"text/markdown": true, "text/plain": true},
		".json": {"application/json": true, "text/json": true}, ".csv": {"text/csv": true, "application/csv": true, "text/plain": true},
	}
	if !allowedTypes[ext][strings.ToLower(strings.TrimSpace(strings.Split(upload.ContentType, ";")[0]))] {
		return "", "", "", errors.New("unsupported document type or content type")
	}
	data, err := io.ReadAll(io.LimitReader(upload.File, maxKnowledgeDocumentBytes+1))
	if err != nil || len(data) != int(upload.Size) || len(data) > maxKnowledgeDocumentBytes {
		return "", "", "", errors.New("document size does not match its payload")
	}
	if err := validateKnowledgeDocumentBytes(ext, data); err != nil {
		return "", "", "", err
	}
	hash := sha256.Sum256(data)
	objectName := fmt.Sprintf("knowledge-upload/%s%s", uuid.NewString(), ext)
	if _, err := s.gcs.Objects.Insert(s.bucket, &gcstorage.Object{Name: objectName, ContentType: strings.Split(upload.ContentType, ";")[0]}).Media(bytes.NewReader(data)).Context(ctx).Do(); err != nil {
		return "", "", "", err
	}
	return "gs://" + s.bucket + "/" + objectName, hex.EncodeToString(hash[:]), filepath.Base(upload.Filename), nil
}

func (s *mediaStore) deleteKnowledgeUpload(ctx context.Context, uri string) {
	if s == nil || s.gcs == nil || !strings.HasPrefix(uri, "gs://"+s.bucket+"/knowledge-upload/") {
		return
	}
	objectName := strings.TrimPrefix(uri, "gs://"+s.bucket+"/")
	_ = s.gcs.Objects.Delete(s.bucket, objectName).Context(ctx).Do()
}

func validateKnowledgeDocumentBytes(ext string, data []byte) error {
	valid := false
	switch ext {
	case ".pdf":
		valid = bytes.HasPrefix(data, []byte("%PDF-"))
	case ".docx":
		valid = len(data) >= 4 && bytes.Equal(data[:2], []byte{'P', 'K'})
	case ".txt", ".md", ".json", ".csv":
		valid = !bytes.Contains(data, []byte{0}) && strings.ToValidUTF8(string(data), "") == string(data)
	}
	if !valid {
		return errors.New("document bytes do not match the declared format")
	}
	return nil
}

func validateProductMedia(reader io.Reader, ext string) (io.Reader, string, error) {
	header := make([]byte, 12)
	n, err := io.ReadFull(reader, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, "", err
	}
	header = header[:n]
	contentType := ""
	switch {
	case len(header) >= 3 && bytes.Equal(header[:3], []byte{0xff, 0xd8, 0xff}):
		contentType = "image/jpeg"
	case len(header) >= 8 && bytes.Equal(header[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		contentType = "image/png"
	case len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP":
		contentType = "image/webp"
	}
	expected := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}[ext]
	if contentType == "" || contentType != expected {
		return nil, "", errors.New("media bytes do not match the filename image type")
	}
	return io.MultiReader(bytes.NewReader(header), reader), contentType, nil
}

func (s *mediaStore) getProductMedia(ctx context.Context, objectName string) (*http.Response, error) {
	if s == nil || s.gcs == nil || !validProductMediaObject(objectName) {
		return nil, errors.New("media object not found")
	}
	return s.gcs.Objects.Get(s.bucket, objectName).Context(ctx).Download()
}

func validProductMediaObject(objectName string) bool {
	parts := strings.Split(objectName, "/")
	if len(parts) != 2 || parts[0] != "products" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(parts[1]))
	if _, err := uuid.Parse(strings.TrimSuffix(parts[1], ext)); err != nil || !map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}[ext] {
		return false
	}
	return true
}
