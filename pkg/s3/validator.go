package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ValidationResult struct {
	IsValid        bool
	Message        string
	CheckedAt      time.Time
	ResponseTimeMs int64
}

type S3Validator struct {
	endpoint  string
	region    string
	bucket    string
	accessKey string
	secretKey string
	newClient func(ctx context.Context) (s3ListObjectsClient, error)
}

type s3ListObjectsClient interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// NewS3Validator creates a new S3 validator instance
func NewS3Validator(endpoint, region, bucket, accessKey, secretKey string) *S3Validator {
	v := &S3Validator{
		endpoint:  endpoint,
		region:    region,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
	}
	v.newClient = v.defaultClientBuilder
	return v
}

// ValidateKeys checks if the provided AWS credentials are valid by attempting
// to list objects in the S3 bucket
func (v *S3Validator) ValidateKeys(ctx context.Context, timeout time.Duration) *ValidationResult {
	result := &ValidationResult{
		CheckedAt: time.Now(),
	}

	start := time.Now()
	defer func() {
		result.ResponseTimeMs = time.Since(start).Milliseconds()
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := v.newClient(ctx)
	if err != nil {
		result.IsValid = false
		result.Message = fmt.Sprintf("Failed to create AWS config: %v", err)
		return result
	}

	// Try to list objects (minimal operation to validate credentials)
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(v.bucket),
		MaxKeys: aws.Int32(1), // Only fetch 1 object to minimize latency
	}

	_, err = client.ListObjectsV2(ctx, input)
	if err != nil {
		result.IsValid = false
		result.Message = fmt.Sprintf("S3 validation failed: %v", err)
		return result
	}

	result.IsValid = true
	result.Message = "AWS credentials are valid"
	return result
}

// HealthCheck performs a lightweight health check to S3
func (v *S3Validator) HealthCheck(ctx context.Context, timeout time.Duration) bool {
	result := v.ValidateKeys(ctx, timeout)
	return result.IsValid
}

func (v *S3Validator) defaultClientBuilder(ctx context.Context) (s3ListObjectsClient, error) {
	// Create S3 config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(v.region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			v.accessKey,
			v.secretKey,
			"", // session token (optional)
		)),
	)
	if err != nil {
		return nil, err
	}

	// Apply custom endpoint if provided
	if v.endpoint != "" {
		cfg.BaseEndpoint = aws.String(v.endpoint)
	}

	return s3.NewFromConfig(cfg), nil
}
