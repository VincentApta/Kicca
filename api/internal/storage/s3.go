// S3Store: one bucket for the whole app (S3_BUCKET), default AWS credential
// chain, optional S3_ENDPOINT for S3-compatible stores (minio later).
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

func newS3(bucket, region, endpoint string) *S3Store {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		// misconfig should surface at startup, not on the first upload
		panic(fmt.Sprintf("storage: aws config: %v", err))
	}
	var s3Opts []func(*s3.Options)
	if endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return &S3Store{client: s3.NewFromConfig(cfg, s3Opts...), bucket: bucket}
}

func (s *S3Store) Kind() string { return "s3" }

func (s *S3Store) Put(ctx context.Context, key, contentType string, size int64, r io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		Body:          r,
	})
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

func (s *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return out.Body, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// PresignURL mints a temporary GET URL (GitHub issue-body fallback when the
// user-attachments upload fails; S3 only — LocalStore has none).
func (s *S3Store) PresignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s3.NewPresignClient(s.client).PresignGetObject(ctx,
		&s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)},
		s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3 presign: %w", err)
	}
	return req.URL, nil
}
