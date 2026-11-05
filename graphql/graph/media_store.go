package graph

import (
	"bytes"
	"context"
	"crypto/sha256"
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
