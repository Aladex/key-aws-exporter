package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"key-aws-exporter/internal/exporter"
	"key-aws-exporter/pkg/s3"

	"github.com/sirupsen/logrus"
)

// Validator abstracts the exporter manager for easier testing
type Validator interface {
	GetEndpointCount() int
	ValidateAll(ctx context.Context) *exporter.ValidationResults
	ValidateEndpoint(ctx context.Context, endpointName string) *s3.ValidationResult
}

type ValidationResponse struct {
	IsValid        bool   `json:"is_valid"`
	Message        string `json:"message"`
	CheckedAt      string `json:"checked_at"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	ErrorType      string `json:"error_type,omitempty"`
}

type MultiValidationResponse struct {
	Timestamp time.Time                     `json:"timestamp"`
	Results   map[string]ValidationResponse `json:"results"`
	Summary   ValidationSummary             `json:"summary"`
}

type ValidationSummary struct {
	TotalEndpoints int `json:"total_endpoints"`
	Successful     int `json:"successful"`
	Failed         int `json:"failed"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Time      string `json:"time"`
	Endpoints int    `json:"endpoints"`
}

// NewHealthCheckHandler returns a handler for health checks
func NewHealthCheckHandler(manager Validator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := HealthResponse{
			Status:    "healthy",
			Time:      time.Now().UTC().Format(time.RFC3339),
			Endpoints: manager.GetEndpointCount(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logrus.Errorf("Failed to encode health response: %v", err)
		}
	}
}

// NewValidateAllHandler returns a handler for validating all endpoints
func NewValidateAllHandler(manager Validator, log *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		results := manager.ValidateAll(ctx)

		// Build response
		response := MultiValidationResponse{
			Timestamp: results.Timestamp,
			Results:   make(map[string]ValidationResponse),
			Summary: ValidationSummary{
				TotalEndpoints: len(results.Results),
			},
		}

		// Process results
		for endpointName, result := range results.Results {
			response.Results[endpointName] = ValidationResponse{
				IsValid:        result.IsValid,
				Message:        result.Message,
				CheckedAt:      result.CheckedAt.UTC().Format(time.RFC3339),
				ResponseTimeMs: result.ResponseTimeMs,
				ErrorType:      result.ErrorType,
			}

			exporter.RecordResult(log, endpointName, result)

			if result.IsValid {
				response.Summary.Successful++
			} else {
				response.Summary.Failed++
			}
		}

		// Determine status code (200 if all successful, 207 if mixed, 401 if all failed)
		statusCode := http.StatusOK
		if response.Summary.Failed > 0 && response.Summary.Successful > 0 {
			statusCode = http.StatusMultiStatus // 207
		} else if response.Summary.Failed > 0 {
			statusCode = http.StatusUnauthorized
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Errorf("Failed to encode validate all response: %v", err)
		}
	}
}

// NewValidateEndpointHandler returns a handler for validating a specific endpoint
func NewValidateEndpointHandler(manager Validator, log *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract endpoint name from URL path
		// Expected format: /validate/{endpoint}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "endpoint name is required", http.StatusBadRequest)
			return
		}

		endpointName := parts[len(parts)-1]
		if endpointName == "" {
			http.Error(w, "endpoint name cannot be empty", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		result := manager.ValidateEndpoint(ctx, endpointName)

		exporter.RecordResult(log, endpointName, result)

		response := ValidationResponse{
			IsValid:        result.IsValid,
			Message:        result.Message,
			CheckedAt:      result.CheckedAt.UTC().Format(time.RFC3339),
			ResponseTimeMs: result.ResponseTimeMs,
			ErrorType:      result.ErrorType,
		}

		w.Header().Set("Content-Type", "application/json")
		statusCode := http.StatusOK
		if !result.IsValid {
			statusCode = http.StatusUnauthorized
		}
		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Errorf("Failed to encode validate endpoint response: %v", err)
		}
	}
}
