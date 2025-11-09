package exporter

import (
	"context"
	"testing"
	"time"

	"key-aws-exporter/internal/config"
	"key-aws-exporter/pkg/s3"

	"github.com/sirupsen/logrus"
)

type stubValidator struct {
	result *s3.ValidationResult
}

func (s *stubValidator) ValidateKeys(ctx context.Context, timeout time.Duration) *s3.ValidationResult {
	return s.result
}

func TestValidatorManagerValidateAll(t *testing.T) {
	cfg := &config.Config{
		ValidationTimeout: time.Second,
		Endpoints: []config.S3EndpointConfig{
			{Name: "one"},
			{Name: "two"},
		},
	}

	vm := NewValidatorManager(cfg, logrus.New())

	vm.mu.Lock()
	vm.validators["one"] = &stubValidator{result: &s3.ValidationResult{IsValid: true, Message: "ok", CheckedAt: time.Now()}}
	vm.validators["two"] = &stubValidator{result: &s3.ValidationResult{IsValid: false, Message: "bad", CheckedAt: time.Now()}}
	vm.mu.Unlock()

	results := vm.ValidateAll(context.Background())

	if len(results.Results) != 2 {
		t.Fatalf("expected results for 2 endpoints, got %d", len(results.Results))
	}
	if !results.Results["one"].IsValid {
		t.Fatalf("expected endpoint one to be valid")
	}
	if results.Results["two"].IsValid {
		t.Fatalf("expected endpoint two to be invalid")
	}
}

func TestValidatorManagerValidateEndpoint(t *testing.T) {
	cfg := &config.Config{ValidationTimeout: time.Second}
	vm := NewValidatorManager(cfg, logrus.New())

	now := time.Now()
	vm.mu.Lock()
	vm.validators = map[string]bucketValidator{
		"exists": &stubValidator{result: &s3.ValidationResult{IsValid: true, Message: "ok", CheckedAt: now}},
	}
	vm.mu.Unlock()

	res := vm.ValidateEndpoint(context.Background(), "exists")
	if !res.IsValid {
		t.Fatalf("expected valid result, got %v", res)
	}

	missing := vm.ValidateEndpoint(context.Background(), "missing")
	if missing.IsValid {
		t.Fatalf("expected invalid result for missing endpoint")
	}
	if missing.Message == "" {
		t.Fatalf("expected error message for missing endpoint")
	}
}

func TestValidatorManagerGetters(t *testing.T) {
	cfg := &config.Config{
		Endpoints: []config.S3EndpointConfig{{Name: "a"}, {Name: "b"}},
	}
	vm := NewValidatorManager(cfg, logrus.New())

	names := vm.GetEndpoints()
	if len(names) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(names))
	}

	if vm.GetEndpointCount() != 2 {
		t.Fatalf("expected endpoint count 2")
	}
}
