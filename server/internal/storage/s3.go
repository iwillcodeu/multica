package storage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client    *s3.Client
	bucket    string
	cdnDomain string // if set, returned URLs use this instead of bucket name
}

// NewS3StorageFromEnv creates an S3Storage from environment variables.
// Returns nil if S3_BUCKET is not set.
//
// Environment variables:
//   - S3_BUCKET (required)
//   - S3_REGION (default: us-west-2; for Aliyun OSS use the OSS region id, e.g. oss-cn-hangzhou)
//   - S3_ENDPOINT (optional) — custom S3 API base URL for Aliyun OSS, MinIO, R2, etc.
//     Example OSS: https://oss-cn-hangzhou.aliyuncs.com
//   - S3_USE_PATH_STYLE=1 (optional) — path-style URLs; often needed for MinIO
//   - AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY (optional; highest priority when both set)
//   - MULTICA_CREDENTIALS_INI (optional) — path to an INI file with [credentials] and
//     alibaba_cloud_access_key_id / alibaba_cloud_access_key_secret (used when env keys are not both set)
func NewS3StorageFromEnv() *S3Storage {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		slog.Info("S3_BUCKET not set, file upload disabled")
		return nil
	}

	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-west-2"
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	accessKey := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if accessKey == "" || secretKey == "" {
		iniPath := strings.TrimSpace(os.Getenv("MULTICA_CREDENTIALS_INI"))
		if iniPath != "" {
			id, sec, err := ReadAlibabaCloudCredentialsFromINI(iniPath)
			if err != nil {
				slog.Error("MULTICA_CREDENTIALS_INI invalid", "path", iniPath, "error", err)
				return nil
			}
			accessKey, secretKey = id, sec
			slog.Info("loaded object storage credentials from INI", "path", iniPath)
		}
	}
	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		slog.Error("failed to load AWS config", "error", err)
		return nil
	}

	cdnDomain := os.Getenv("CLOUDFRONT_DOMAIN")
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	pathStyle := os.Getenv("S3_USE_PATH_STYLE") == "1" || strings.EqualFold(os.Getenv("S3_USE_PATH_STYLE"), "true")

	s3Opts := []func(*s3.Options){}
	if endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = pathStyle
			// Aliyun OSS and other S3-compatible APIs reject AWS SDK v2's default
			// aws-chunked + x-amz-checksum-* PutObject behavior (400 InvalidArgument).
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		})
	}

	client := s3.NewFromConfig(cfg, s3Opts...)

	slog.Info("S3 storage initialized", "bucket", bucket, "region", region, "endpoint", endpointOrDefault(endpoint), "cdn_domain", cdnDomain)
	return &S3Storage{
		client:    client,
		bucket:    bucket,
		cdnDomain: cdnDomain,
	}
}

func endpointOrDefault(endpoint string) string {
	if endpoint == "" {
		return "(aws default)"
	}
	return endpoint
}

// sanitizeFilename removes characters that could cause header injection in Content-Disposition.
func sanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		// Strip control chars, newlines, null bytes, quotes, semicolons, backslashes
		if r < 0x20 || r == 0x7f || r == '"' || r == ';' || r == '\\' || r == '\x00' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// KeyFromURL extracts the S3 object key from a CDN or bucket URL.
// e.g. "https://multica-static.copilothub.ai/abc123.png" → "abc123.png"
func (s *S3Storage) KeyFromURL(rawURL string) string {
	// Strip the "https://domain/" prefix.
	for _, prefix := range []string{
		"https://" + s.cdnDomain + "/",
		"https://" + s.bucket + "/",
	} {
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}
	// Fallback: take everything after the last "/".
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// Delete removes an object from S3. Errors are logged but not fatal.
func (s *S3Storage) Delete(ctx context.Context, key string) {
	if key == "" {
		return
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("s3 DeleteObject failed", "key", key, "error", err)
	}
}

// DeleteKeys removes multiple objects from S3. Best-effort, errors are logged.
func (s *S3Storage) DeleteKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		s.Delete(ctx, key)
	}
}

func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error) {
	safe := sanitizeFilename(filename)
	put := &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(data),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s"`, safe)),
		CacheControl:       aws.String("max-age=432000,public"),
	}
	// INTELLIGENT_TIERING is AWS-specific; omit for S3-compatible endpoints (OSS, MinIO, R2).
	if strings.TrimSpace(os.Getenv("S3_ENDPOINT")) == "" {
		put.StorageClass = types.StorageClassIntelligentTiering
	}
	_, err := s.client.PutObject(ctx, put)
	if err != nil {
		return "", fmt.Errorf("s3 PutObject: %w", err)
	}

	domain := s.bucket
	if s.cdnDomain != "" {
		domain = s.cdnDomain
	}
	link := fmt.Sprintf("https://%s/%s", domain, key)
	return link, nil
}
