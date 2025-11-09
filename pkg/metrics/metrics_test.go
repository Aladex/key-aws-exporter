package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func resetAll() {
	ValidationAttempts.Reset()
	ValidationSuccess.Reset()
	ValidationFailures.Reset()
	ValidationDuration.Reset()
	KeysValid.Reset()
	LastValidationTimestamp.Reset()
	ResponseTime.Reset()
	EndpointConfigured.Reset()
}

func TestRecordValidationAttempt(t *testing.T) {
	resetAll()

	RecordValidationAttempt("bucket-a", true)
	RecordValidationAttempt("bucket-a", false)

	success := testutil.ToFloat64(ValidationAttempts.WithLabelValues("bucket-a", "success"))
	failure := testutil.ToFloat64(ValidationAttempts.WithLabelValues("bucket-a", "failure"))

	if success != 1 {
		t.Fatalf("expected 1 success attempt, got %v", success)
	}
	if failure != 1 {
		t.Fatalf("expected 1 failure attempt, got %v", failure)
	}
}

func TestRecordValidationSuccessAndFailure(t *testing.T) {
	resetAll()

	RecordValidationSuccess("bucket-a")
	RecordValidationFailure("bucket-a", "timeout")

	successes := testutil.ToFloat64(ValidationSuccess.WithLabelValues("bucket-a"))
	failures := testutil.ToFloat64(ValidationFailures.WithLabelValues("bucket-a", "timeout"))
	gauge := testutil.ToFloat64(KeysValid.WithLabelValues("bucket-a"))

	if successes != 1 {
		t.Fatalf("expected 1 success recorded, got %v", successes)
	}
	if failures != 1 {
		t.Fatalf("expected 1 failure recorded, got %v", failures)
	}
	if gauge != 0 {
		t.Fatalf("expected latest keys valid gauge to be 0 after failure, got %v", gauge)
	}
}

func TestSetLastValidationTimeAndResponse(t *testing.T) {
	resetAll()

	SetLastValidationTime("bucket-a", 12345)
	RecordResponseTime("bucket-a", "ListObjectsV2", 42)

	last := testutil.ToFloat64(LastValidationTimestamp.WithLabelValues("bucket-a"))
	if last != 12345 {
		t.Fatalf("expected timestamp 12345, got %v", last)
	}

	metricsCount := testutil.CollectAndCount(ResponseTime)
	if metricsCount != 1 {
		t.Fatalf("expected histogram to have 1 metric sample, got %d", metricsCount)
	}
}

func TestRegisterEndpointSeedsMetrics(t *testing.T) {
	resetAll()

	RegisterEndpoint("bucket-a")

	configGauge := testutil.ToFloat64(EndpointConfigured.WithLabelValues("bucket-a"))
	if configGauge != 1 {
		t.Fatalf("expected configured gauge 1, got %v", configGauge)
	}

	keys := testutil.ToFloat64(KeysValid.WithLabelValues("bucket-a"))
	if keys != 0 {
		t.Fatalf("expected keys gauge 0, got %v", keys)
	}

	lastValidation := testutil.ToFloat64(LastValidationTimestamp.WithLabelValues("bucket-a"))
	if lastValidation != 0 {
		t.Fatalf("expected last validation timestamp 0, got %v", lastValidation)
	}

	if testutil.ToFloat64(ValidationAttempts.WithLabelValues("bucket-a", "success")) != 0 {
		t.Fatalf("expected success counter to remain 0")
	}
	if testutil.ToFloat64(ValidationAttempts.WithLabelValues("bucket-a", "failure")) != 0 {
		t.Fatalf("expected failure counter to remain 0")
	}
	if testutil.ToFloat64(ValidationFailures.WithLabelValues("bucket-a", "validation_failed")) != 0 {
		t.Fatalf("expected failure detail counter 0")
	}
}
