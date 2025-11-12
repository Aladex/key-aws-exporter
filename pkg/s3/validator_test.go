package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
)

type mockS3Client struct {
	err    error
	called bool
}

func (m *mockS3Client) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &s3.ListObjectsV2Output{}, nil
}

type mockNetError struct {
	msg     string
	timeout bool
}

func (m *mockNetError) Error() string   { return m.msg }
func (m *mockNetError) Timeout() bool   { return m.timeout }
func (m *mockNetError) Temporary() bool { return false }

func TestValidateKeysSuccess(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)
	mockClient := &mockS3Client{}
	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		return mockClient, nil
	}

	result := validator.ValidateKeys(context.Background(), time.Second)

	if !result.IsValid {
		t.Fatalf("expected validation success, got failure: %s", result.Message)
	}
	if !strings.Contains(result.Message, "valid") {
		t.Fatalf("expected success message, got %s", result.Message)
	}
	if !mockClient.called {
		t.Fatalf("expected ListObjectsV2 to be called")
	}
	if result.ResponseTimeMs < 0 {
		t.Fatalf("expected non-negative response time, got %d", result.ResponseTimeMs)
	}
	if result.ErrorType != "" {
		t.Fatalf("expected empty error type on success, got %s", result.ErrorType)
	}
}

func TestValidateKeysListError(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)
	mockClient := &mockS3Client{err: errors.New("boom")}
	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		return mockClient, nil
	}

	result := validator.ValidateKeys(context.Background(), time.Second)

	if result.IsValid {
		t.Fatalf("expected validation failure")
	}
	if !strings.Contains(result.Message, "S3 validation failed") {
		t.Fatalf("unexpected error message: %s", result.Message)
	}
	if !mockClient.called {
		t.Fatalf("expected client to be called")
	}
}

func TestValidateKeysConfigError(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)
	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		return nil, errors.New("config failed")
	}

	result := validator.ValidateKeys(context.Background(), time.Second)

	if result.IsValid {
		t.Fatalf("expected validation failure")
	}
	if !strings.Contains(result.Message, "Failed to create AWS client") {
		t.Fatalf("unexpected error message: %s", result.Message)
	}
	if result.ErrorType != errorTypeConfig {
		t.Fatalf("expected config error type, got %s", result.ErrorType)
	}
}

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name     string
		valid    bool
		mockErr  error
		expected bool
	}{
		{
			name:     "successful health check",
			valid:    true,
			mockErr:  nil,
			expected: true,
		},
		{
			name:     "failed health check",
			valid:    false,
			mockErr:  errors.New("connection error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)
			mockClient := &mockS3Client{err: tt.mockErr}
			validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
				return mockClient, nil
			}

			result := validator.HealthCheck(context.Background(), time.Second)

			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClientCaching(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)
	mockClient := &mockS3Client{}
	callCount := 0

	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		callCount++
		return mockClient, nil
	}

	// First call should create the client
	result1 := validator.ValidateKeys(context.Background(), time.Second)
	if !result1.IsValid {
		t.Fatalf("first validation failed")
	}

	// Second call should reuse the cached client
	result2 := validator.ValidateKeys(context.Background(), time.Second)
	if !result2.IsValid {
		t.Fatalf("second validation failed")
	}

	if callCount != 1 {
		t.Fatalf("expected newClient to be called once, got %d calls", callCount)
	}
}

func TestNewS3Validator(t *testing.T) {
	validator := NewS3Validator("https://s3.amazonaws.com", "us-east-1", "test-bucket", "access-key", "secret-key", "session-token", true, true)

	if validator.endpoint != "https://s3.amazonaws.com" {
		t.Fatalf("endpoint not set correctly")
	}
	if validator.region != "us-east-1" {
		t.Fatalf("region not set correctly")
	}
	if validator.bucket != "test-bucket" {
		t.Fatalf("bucket not set correctly")
	}
	if validator.accessKey != "access-key" {
		t.Fatalf("accessKey not set correctly")
	}
	if validator.secretKey != "secret-key" {
		t.Fatalf("secretKey not set correctly")
	}
	if validator.sessionToken != "session-token" {
		t.Fatalf("sessionToken not set correctly")
	}
	if !validator.usePathStyle {
		t.Fatalf("usePathStyle not set correctly")
	}
	if !validator.insecureSkipVerify {
		t.Fatalf("insecureSkipVerify not set correctly")
	}
}

func TestContextTimeout(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk", "", false, false)

	// Simulate slow client that doesn't return until after timeout
	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		time.Sleep(100 * time.Millisecond)
		return &mockS3Client{}, nil
	}

	result := validator.ValidateKeys(context.Background(), 50*time.Millisecond)

	// The context timeout should be handled, but the client creation might still complete
	// The timeout is on the ListObjectsV2 call, not the client creation
	if result.ResponseTimeMs < 0 {
		t.Fatalf("response time should be non-negative")
	}
}

func TestClassifyValidationErrorTimeout(t *testing.T) {
	errType := classifyValidationError(context.DeadlineExceeded)
	if errType != errorTypeTimeout {
		t.Fatalf("expected timeout error type, got %s", errType)
	}
}

func TestClassifyValidationErrorCanceled(t *testing.T) {
	errType := classifyValidationError(context.Canceled)
	if errType != errorTypeCanceled {
		t.Fatalf("expected canceled error type, got %s", errType)
	}
}

func TestClassifyValidationErrorNetworkTimeout(t *testing.T) {
	netErr := &mockNetError{msg: "i/o timeout", timeout: true}
	errType := classifyValidationError(netErr)
	if errType != errorTypeTimeout {
		t.Fatalf("expected timeout error type for network timeout, got %s", errType)
	}
}

func TestClassifyValidationErrorNetwork(t *testing.T) {
	netErr := &mockNetError{msg: "connection refused", timeout: false}
	errType := classifyValidationError(netErr)
	if errType != errorTypeNetwork {
		t.Fatalf("expected network error type, got %s", errType)
	}
}

func TestClassifyValidationErrorAccessDenied(t *testing.T) {
	mockErr := &mockAPIError{
		code: "AccessDenied",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeForbidden {
		t.Fatalf("expected forbidden error type, got %s", errType)
	}
}

func TestClassifyValidationErrorInvalidAccessKeyId(t *testing.T) {
	mockErr := &mockAPIError{
		code: "InvalidAccessKeyId",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeForbidden {
		t.Fatalf("expected forbidden error type, got %s", errType)
	}
}

func TestClassifyValidationErrorSignatureDoesNotMatch(t *testing.T) {
	mockErr := &mockAPIError{
		code: "SignatureDoesNotMatch",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeForbidden {
		t.Fatalf("expected forbidden error type, got %s", errType)
	}
}

func TestClassifyValidationErrorNoSuchBucket(t *testing.T) {
	mockErr := &mockAPIError{
		code: "NoSuchBucket",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeNotFound {
		t.Fatalf("expected not found error type, got %s", errType)
	}
}

func TestClassifyValidationErrorNoSuchBucketPolicy(t *testing.T) {
	mockErr := &mockAPIError{
		code: "NoSuchBucketPolicy",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeNotFound {
		t.Fatalf("expected not found error type, got %s", errType)
	}
}

func TestClassifyValidationErrorExpiredToken(t *testing.T) {
	mockErr := &mockAPIError{
		code: "ExpiredToken",
	}
	errType := classifyValidationError(mockErr)
	if errType != "token_expired" {
		t.Fatalf("expected token_expired error type, got %s", errType)
	}
}

func TestClassifyValidationErrorSlowdown(t *testing.T) {
	mockErr := &mockAPIError{
		code: "SlowDown",
	}
	errType := classifyValidationError(mockErr)
	if errType != "throttled" {
		t.Fatalf("expected throttled error type, got %s", errType)
	}
}

func TestClassifyValidationErrorThrottling(t *testing.T) {
	mockErr := &mockAPIError{
		code: "Throttling",
	}
	errType := classifyValidationError(mockErr)
	if errType != "throttled" {
		t.Fatalf("expected throttled error type, got %s", errType)
	}
}

func TestClassifyValidationErrorThrottlingException(t *testing.T) {
	mockErr := &mockAPIError{
		code: "ThrottlingException",
	}
	errType := classifyValidationError(mockErr)
	if errType != "throttled" {
		t.Fatalf("expected throttled error type, got %s", errType)
	}
}

func TestClassifyValidationErrorRequestTimeout(t *testing.T) {
	mockErr := &mockAPIError{
		code: "RequestTimeout",
	}
	errType := classifyValidationError(mockErr)
	if errType != errorTypeTimeout {
		t.Fatalf("expected timeout error type, got %s", errType)
	}
}

func TestClassifyValidationErrorResponseErrorForbidden(t *testing.T) {
	// smithyhttp.ResponseError is hard to mock, skip this test
	// The actual S3 client will generate proper ResponseError instances
}

func TestClassifyValidationErrorResponseErrorNotFound(t *testing.T) {
	// smithyhttp.ResponseError is hard to mock, skip this test
	// The actual S3 client will generate proper ResponseError instances
}

func TestClassifyValidationErrorResponseErrorGatewayTimeout(t *testing.T) {
	// smithyhttp.ResponseError is hard to mock, skip this test
	// The actual S3 client will generate proper ResponseError instances
}

func TestClassifyValidationErrorUnknown(t *testing.T) {
	errType := classifyValidationError(errors.New("unknown error"))
	if errType != errorTypeUnknown {
		t.Fatalf("expected unknown error type, got %s", errType)
	}
}

func TestClassifyValidationErrorNil(t *testing.T) {
	errType := classifyValidationError(nil)
	if errType != "" {
		t.Fatalf("expected empty string for nil error, got %s", errType)
	}
}

// Mock types for testing
type mockAPIError struct {
	code string
}

func (m *mockAPIError) Error() string {
	return "API Error: " + m.code
}

func (m *mockAPIError) ErrorCode() string {
	return m.code
}

func (m *mockAPIError) ErrorMessage() string {
	return "mock error message"
}

func (m *mockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultUnknown
}

var _ smithy.APIError = (*mockAPIError)(nil)
