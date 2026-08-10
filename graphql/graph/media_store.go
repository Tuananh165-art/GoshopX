package graph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/Tuananh165art/GoshopX/graphql/config"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const maxProductMediaBytes = 10 << 20

type mediaStore struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func newMediaStore() (*mediaStore, error) {
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
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		exists, existsErr := s.client.BucketExists(ctx, s.bucket)
		if existsErr != nil || !exists {
			return "", "", err
		}
	}
	hash := sha256.New()
	reader := io.TeeReader(io.LimitReader(upload.File, maxProductMediaBytes+1), hash)
	objectName := fmt.Sprintf("products/%s/%s%s", productID, uuid.NewString(), ext)
	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, upload.Size, minio.PutObjectOptions{ContentType: upload.ContentType})
	if err != nil {
		return "", "", err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if s.publicURL == "" {
		return objectName, checksum, nil
	}
	return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, objectName), checksum, nil
}
