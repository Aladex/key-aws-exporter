package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ValidationAttempts tracks the total number of validation attempts
	ValidationAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_validation_attempts_total",
			Help: "Total number of S3 key validation attempts",
		},
		[]string{"bucket", "status"},
	)

	// ValidationSuccess tracks the number of successful validations
	ValidationSuccess = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_validation_success_total",
			Help: "Total number of successful S3 validations",
		},
		[]string{"bucket"},
	)

	// ValidationFailures tracks the number of failed validations
	ValidationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_validation_failures_total",
			Help: "Total number of failed S3 validations",
		},
		[]string{"bucket", "error_type"},
	)

	// ValidationDuration tracks the duration of validation operations
	ValidationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "s3_validation_duration_seconds",
			Help:    "Duration of S3 validation operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"bucket"},
	)

	// KeysValid indicates whether the current keys are valid (1 = valid, 0 = invalid)
	KeysValid = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "s3_keys_valid",
			Help: "Whether the S3 keys are currently valid (1 = valid, 0 = invalid)",
		},
		[]string{"bucket"},
	)

	// LastValidationTimestamp tracks when the last validation occurred
	LastValidationTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "s3_last_validation_timestamp_seconds",
			Help: "Unix timestamp of the last validation attempt",
		},
		[]string{"bucket"},
	)

	// ResponseTime tracks the response time of S3 operations
	ResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "s3_response_time_milliseconds",
			Help:    "Response time of S3 operations in milliseconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 8), // 10ms to 1280ms
		},
		[]string{"bucket", "operation"},
	)

	// EndpointConfigured marks configured endpoints so users can discover them via metrics
	EndpointConfigured = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "s3_endpoint_configured",
			Help: "Configured S3 endpoints (always 1 for configured endpoints)",
		},
		[]string{"bucket"},
	)
)

// RecordValidationAttempt records a validation attempt in metrics
func RecordValidationAttempt(bucket string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	ValidationAttempts.WithLabelValues(bucket, status).Inc()
}

// RecordValidationSuccess records a successful validation
func RecordValidationSuccess(bucket string) {
	ValidationSuccess.WithLabelValues(bucket).Inc()
	KeysValid.WithLabelValues(bucket).Set(1)
}

// RecordValidationFailure records a failed validation
func RecordValidationFailure(bucket, errorType string) {
	ValidationFailures.WithLabelValues(bucket, errorType).Inc()
	KeysValid.WithLabelValues(bucket).Set(0)
}

// SetLastValidationTime sets the last validation timestamp
func SetLastValidationTime(bucket string, timestamp float64) {
	LastValidationTimestamp.WithLabelValues(bucket).Set(timestamp)
}

// RecordResponseTime records the response time of an operation
func RecordResponseTime(bucket, operation string, milliseconds float64) {
	ResponseTime.WithLabelValues(bucket, operation).Observe(milliseconds)
}

// RegisterEndpoint seeds metrics for a bucket so they are visible before validation occurs
func RegisterEndpoint(bucket string) {
	EndpointConfigured.WithLabelValues(bucket).Set(1)
	KeysValid.WithLabelValues(bucket).Set(0)
	LastValidationTimestamp.WithLabelValues(bucket).Set(0)
	ValidationAttempts.WithLabelValues(bucket, "success").Add(0)
	ValidationAttempts.WithLabelValues(bucket, "failure").Add(0)
	ValidationSuccess.WithLabelValues(bucket).Add(0)
	ValidationFailures.WithLabelValues(bucket, "validation_failed").Add(0)
}
