package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockS3Client struct {
	err    error
	called bool
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &s3.ListObjectsV2Output{}, nil
}

func TestValidateKeysSuccess(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk")
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
}

func TestValidateKeysListError(t *testing.T) {
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk")
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
	validator := NewS3Validator("endpoint", "region", "bucket", "ak", "sk")
	validator.newClient = func(ctx context.Context) (s3ListObjectsClient, error) {
		return nil, errors.New("config failed")
	}

	result := validator.ValidateKeys(context.Background(), time.Second)

	if result.IsValid {
		t.Fatalf("expected validation failure")
	}
	if !strings.Contains(result.Message, "Failed to create AWS config") {
		t.Fatalf("unexpected error message: %s", result.Message)
	}
}
