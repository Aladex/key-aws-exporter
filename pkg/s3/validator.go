package s3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	errorTypeUnknown   = "unknown"
	errorTypeConfig    = "config_error"
	errorTypeTimeout   = "timeout"
	errorTypeCanceled  = "canceled"
	errorTypeNetwork   = "network"
	errorTypeForbidden = "access_denied"
	errorTypeNotFound  = "bucket_not_found"
)

type ValidationResult struct {
	IsValid        bool
	Message        string
	CheckedAt      time.Time
	ResponseTimeMs int64
	ErrorType      string
	Duration       time.Duration
}

type S3Validator struct {
	endpoint           string
	region             string
	bucket             string
	accessKey          string
	secretKey          string
	sessionToken       string
	usePathStyle       bool
	insecureSkipVerify bool

	client   s3ListObjectsClient
	clientMu sync.Mutex

	newClient func(ctx context.Context) (s3ListObjectsClient, error)
}

type s3ListObjectsClient interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// NewS3Validator creates a new S3 validator instance
func NewS3Validator(endpoint, region, bucket, accessKey, secretKey, sessionToken string, usePathStyle, insecureSkipVerify bool) *S3Validator {
	v := &S3Validator{
		endpoint:           endpoint,
		region:             region,
		bucket:             bucket,
		accessKey:          accessKey,
		secretKey:          secretKey,
		sessionToken:       sessionToken,
		usePathStyle:       usePathStyle,
		insecureSkipVerify: insecureSkipVerify,
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
		elapsed := time.Since(start)
		result.Duration = elapsed
		result.ResponseTimeMs = elapsed.Milliseconds()
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := v.getClient(ctx)
	if err != nil {
		result.IsValid = false
		result.Message = fmt.Sprintf("Failed to create AWS client: %v", err)
		result.ErrorType = errorTypeConfig
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
		result.ErrorType = classifyValidationError(err)
		return result
	}

	result.IsValid = true
	result.Message = "AWS credentials are valid"
	result.ErrorType = ""
	return result
}

// HealthCheck performs a lightweight health check to S3
func (v *S3Validator) HealthCheck(ctx context.Context, timeout time.Duration) bool {
	result := v.ValidateKeys(ctx, timeout)
	return result.IsValid
}

func (v *S3Validator) defaultClientBuilder(ctx context.Context) (s3ListObjectsClient, error) {
	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(v.region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			v.accessKey,
			v.secretKey,
			v.sessionToken,
		)),
	}

	var insecureTransport *http.Client
	if v.insecureSkipVerify {
		insecureTransport = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for MinIO/self-signed setups
			},
		}
		loadOptions = append(loadOptions, config.WithHTTPClient(insecureTransport))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}

	// Apply custom endpoint if provided
	if v.endpoint != "" {
		cfg.BaseEndpoint = aws.String(v.endpoint)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = v.usePathStyle
		if v.endpoint != "" {
			o.BaseEndpoint = aws.String(v.endpoint)
		}
		if v.insecureSkipVerify && insecureTransport != nil {
			o.HTTPClient = insecureTransport
		}
	}), nil
}

func (v *S3Validator) getClient(ctx context.Context) (s3ListObjectsClient, error) {
	v.clientMu.Lock()
	defer v.clientMu.Unlock()

	if v.client != nil {
		return v.client, nil
	}

	client, err := v.newClient(ctx)
	if err != nil {
		return nil, err
	}

	v.client = client
	return client, nil
}

func classifyValidationError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errorTypeTimeout
	}

	if errors.Is(err, context.Canceled) {
		return errorTypeCanceled
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return errorTypeTimeout
		}
		return errorTypeNetwork
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := strings.ToLower(apiErr.ErrorCode())
		switch code {
		case "accessdenied", "invalidaccesskeyid", "signaturedoesnotmatch":
			return errorTypeForbidden
		case "nosuchbucket", "nosuchbucketpolicy":
			return errorTypeNotFound
		case "expiredtoken":
			return "token_expired"
		case "slowdown", "throttling", "throttlingexception":
			return "throttled"
		case "requesttimeout":
			return errorTypeTimeout
		}
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case http.StatusForbidden:
			return errorTypeForbidden
		case http.StatusNotFound:
			return errorTypeNotFound
		case http.StatusGatewayTimeout:
			return errorTypeTimeout
		}
	}

	return errorTypeUnknown
}
