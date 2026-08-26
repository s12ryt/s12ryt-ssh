package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"s12ryt-ssh/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage against any S3-compatible API (AWS S3,
// Cloudflare R2, MinIO, etc.).
type S3Storage struct {
	client *s3.Client
	bucket string
}

// buildS3Config constructs an aws.Config for the given S3 profile using static
// credentials and a custom endpoint resolver.
func buildS3Config(p config.S3Profile) (aws.Config, error) {
	cfg := aws.Config{
		Region:      p.Region,
		Credentials: credentials.NewStaticCredentialsProvider(p.AccessKey, p.SecretKey, ""),
	}
	if p.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(p.Endpoint)
	}
	return cfg, nil
}

// NewS3Storage creates an S3Storage for the given profile.
func NewS3Storage(p config.S3Profile, opts ...S3Option) (*S3Storage, error) {
	cfg, err := buildS3Config(p)
	if err != nil {
		return nil, err
	}
	o := &s3Options{}
	for _, opt := range opts {
		opt(o)
	}
	clientOpts := []func(*s3.Options){
		func(o *s3.Options) { o.UsePathStyle = p.UsePathStyle },
	}
	if o.httpClient != nil {
		hc := o.httpClient
		clientOpts = append(clientOpts, func(o *s3.Options) { o.HTTPClient = hc })
	}
	client := s3.NewFromConfig(cfg, clientOpts...)
	return &S3Storage{client: client, bucket: p.Bucket}, nil
}

// S3Option configures an S3Storage.
type S3Option func(*s3Options)

type s3Options struct {
	httpClient aws.HTTPClient
}

// WithHTTPClient sets a custom HTTP client (used in tests for retry logic).
func WithHTTPClient(c aws.HTTPClient) S3Option {
	return func(o *s3Options) { o.httpClient = c }
}

// Put uploads data to key.
func (s *S3Storage) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

// Get downloads the object at key, or returns ErrNotFound on a 404.
func (s *S3Storage) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nf interface{ NotFound() bool }
		if errors.As(err, &nf) && nf.NotFound() {
			return nil, ErrNotFound
		}
		// fall back to error-string sniffing for fake servers
		var httpErr interface{ HTTPStatusCode() int }
		if errors.As(err, &httpErr) && httpErr.HTTPStatusCode() == 404 {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// List returns objects whose key starts with prefix.
func (s *S3Storage) List(ctx context.Context, prefix string) ([]Object, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	var objs []Object
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range out.Contents {
			objs = append(objs, Object{
				Key:      aws.ToString(c.Key),
				Size:     aws.ToInt64(c.Size),
				Modified: aws.ToTime(c.LastModified),
			})
		}
	}
	return objs, nil
}

// Delete removes key.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// Modified returns the modified time, defensive against zero.
func (o Object) ModifiedOrNow() time.Time {
	if o.Modified.IsZero() {
		return time.Now()
	}
	return o.Modified
}
