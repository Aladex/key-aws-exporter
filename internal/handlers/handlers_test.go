package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"key-aws-exporter/internal/exporter"
	"key-aws-exporter/pkg/s3"

	"github.com/sirupsen/logrus"
)

type stubManager struct {
	endpointsCount       int
	validateAllFunc      func(context.Context) *exporter.ValidationResults
	validateEndpointFunc func(context.Context, string) *s3.ValidationResult
}

func (s *stubManager) ValidateAll(ctx context.Context) *exporter.ValidationResults {
	if s.validateAllFunc != nil {
		return s.validateAllFunc(ctx)
	}
	return &exporter.ValidationResults{Results: map[string]*s3.ValidationResult{}}
}

func (s *stubManager) ValidateEndpoint(ctx context.Context, name string) *s3.ValidationResult {
	if s.validateEndpointFunc != nil {
		return s.validateEndpointFunc(ctx, name)
	}
	return &s3.ValidationResult{IsValid: true, Message: "ok", CheckedAt: time.Now()}
}

func (s *stubManager) GetEndpointCount() int {
	return s.endpointsCount
}

func TestHealthCheckHandler(t *testing.T) {
	mgr := &stubManager{endpointsCount: 2}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler := NewHealthCheckHandler(mgr)
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Endpoints != 2 {
		t.Fatalf("expected 2 endpoints in response, got %d", resp.Endpoints)
	}

	reqInvalid := httptest.NewRequest(http.MethodPost, "/health", nil)
	rrInvalid := httptest.NewRecorder()
	handler(rrInvalid, reqInvalid)

	if rrInvalid.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405 for unsupported method, got %d", rrInvalid.Code)
	}
}

func TestValidateAllHandlerStatusCodes(t *testing.T) {
	baseTime := time.Unix(1730000000, 0)
	logger := logrus.New()

	cases := []struct {
		name       string
		results    map[string]bool
		wantStatus int
	}{
		{name: "all success", results: map[string]bool{"a": true, "b": true}, wantStatus: http.StatusOK},
		{name: "mixed", results: map[string]bool{"a": true, "b": false}, wantStatus: http.StatusMultiStatus},
		{name: "all failed", results: map[string]bool{"a": false}, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &stubManager{
				validateAllFunc: func(ctx context.Context) *exporter.ValidationResults {
					res := &exporter.ValidationResults{
						Timestamp: baseTime,
						Results:   make(map[string]*s3.ValidationResult),
					}
					for bucket, ok := range tt.results {
						res.Results[bucket] = &s3.ValidationResult{
							IsValid:        ok,
							Message:        "tested",
							CheckedAt:      baseTime,
							ResponseTimeMs: 5,
						}
					}
					return res
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/validate", nil)
			rr := httptest.NewRecorder()

			handler := NewValidateAllHandler(mgr, logger)
			handler(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("expected JSON content type")
			}
		})
	}

	t.Run("method not allowed", func(t *testing.T) {
		mgr := &stubManager{}
		req := httptest.NewRequest(http.MethodGet, "/validate", nil)
		rr := httptest.NewRecorder()
		handler := NewValidateAllHandler(mgr, logger)
		handler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rr.Code)
		}
	})
}

func TestValidateEndpointHandler(t *testing.T) {
	baseTime := time.Unix(1730000000, 0)
	logger := logrus.New()

	mgr := &stubManager{
		validateEndpointFunc: func(ctx context.Context, name string) *s3.ValidationResult {
			if name == "broken" {
				return &s3.ValidationResult{IsValid: false, Message: "broken", CheckedAt: baseTime}
			}
			if name == "missing" {
				return &s3.ValidationResult{IsValid: false, Message: "endpoint 'missing' not found", CheckedAt: baseTime}
			}
			return &s3.ValidationResult{IsValid: true, Message: "ok", CheckedAt: baseTime}
		},
	}

	handler := NewValidateEndpointHandler(mgr, logger)

	req := httptest.NewRequest(http.MethodGet, "/validate/bucket-a", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	reqPost := httptest.NewRequest(http.MethodPost, "/validate/broken", nil)
	rrPost := httptest.NewRecorder()
	handler(rrPost, reqPost)
	if rrPost.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when validation fails, got %d", rrPost.Code)
	}

	reqMissing := httptest.NewRequest(http.MethodPost, "/validate/", nil)
	rrMissing := httptest.NewRecorder()
	handler(rrMissing, reqMissing)
	if rrMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when endpoint missing, got %d", rrMissing.Code)
	}

	reqInvalidMethod := httptest.NewRequest(http.MethodDelete, "/validate/bucket-a", nil)
	rrInvalidMethod := httptest.NewRecorder()
	handler(rrInvalidMethod, reqInvalidMethod)
	if rrInvalidMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for invalid method, got %d", rrInvalidMethod.Code)
	}
}
