package utils

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rki-mai/wb-landing-builder/config"
)

// S3BlobStorage сохраняет bundle в S3-совместимом bucket (MinIO, AWS S3).
type S3BlobStorage struct {
	client *s3.Client
	bucket string
}

// NewS3BlobStorage создаёт клиент S3 и проверяет доступность bucket.
func NewS3BlobStorage(ctx context.Context, cfg config.S3Config) (*S3BlobStorage, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	client := s3.NewFromConfig(aws.Config{
		Region:      cfg.Region,
		Credentials: creds,
	}, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	storage := &S3BlobStorage{
		client: client,
		bucket: cfg.Bucket,
	}

	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)})
	if err != nil {
		return storage, fmt.Errorf("head bucket %q: %w", cfg.Bucket, err)
	}

	return storage, nil
}

func (s *S3BlobStorage) objectKey(bundleKey, blobPath string) string {
	return path.Join(bundleKey, blobPath)
}

func (s *S3BlobStorage) PutBundle(ctx context.Context, bundleKey string, blobs []Blob) error {
	for _, blob := range blobs {
		input := &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(s.objectKey(bundleKey, blob.Path)),
			Body:        bytes.NewReader(blob.Content),
			ContentType: aws.String(blob.ContentType),
		}
		if _, err := s.client.PutObject(ctx, input); err != nil {
			return fmt.Errorf("put object %s: %w", blob.Path, err)
		}
	}
	return nil
}

func (s *S3BlobStorage) DeleteBundle(ctx context.Context, bundleKey string) error {
	prefix := bundleKey
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var continuation *string
	for {
		listOut, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuation,
		})
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}

		if len(listOut.Contents) == 0 {
			return nil
		}

		objects := make([]types.ObjectIdentifier, 0, len(listOut.Contents))
		for _, obj := range listOut.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: obj.Key})
		}

		_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return fmt.Errorf("delete objects: %w", err)
		}

		if !aws.ToBool(listOut.IsTruncated) {
			return nil
		}
		continuation = listOut.NextContinuationToken
	}
}

func (s *S3BlobStorage) URI(bundleKey string) string {
	return fmt.Sprintf("s3://%s/%s/", s.bucket, bundleKey)
}
