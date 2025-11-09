package exporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"key-aws-exporter/internal/config"
	"key-aws-exporter/pkg/s3"

	"github.com/sirupsen/logrus"
)

// ValidatorManager manages multiple S3 validators
type bucketValidator interface {
	ValidateKeys(ctx context.Context, timeout time.Duration) *s3.ValidationResult
}

// ValidatorManager manages multiple S3 validators
type ValidatorManager struct {
	validators map[string]bucketValidator
	mu         sync.RWMutex
	log        *logrus.Logger
	timeout    time.Duration
}

// ValidationResults contains results for all endpoints
type ValidationResults struct {
	Timestamp time.Time
	Results   map[string]*s3.ValidationResult // key: endpoint name
}

// NewValidatorManager creates a new validator manager
func NewValidatorManager(cfg *config.Config, log *logrus.Logger) *ValidatorManager {
	vm := &ValidatorManager{
		validators: make(map[string]bucketValidator),
		log:        log,
		timeout:    cfg.ValidationTimeout,
	}

	// Initialize validators for each endpoint
	for _, endpointCfg := range cfg.Endpoints {
		validator := s3.NewS3Validator(
			endpointCfg.Endpoint,
			endpointCfg.Region,
			endpointCfg.Bucket,
			endpointCfg.AccessKey,
			endpointCfg.SecretKey,
		)
		vm.validators[endpointCfg.Name] = validator

		log.WithFields(logrus.Fields{
			"endpoint_name": endpointCfg.Name,
			"bucket":        endpointCfg.Bucket,
			"region":        endpointCfg.Region,
		}).Debug("Registered S3 validator")
	}

	return vm
}

// ValidateAll validates all endpoints and returns results
func (vm *ValidatorManager) ValidateAll(ctx context.Context) *ValidationResults {
	results := &ValidationResults{
		Timestamp: time.Now(),
		Results:   make(map[string]*s3.ValidationResult),
	}

	// Create channel for results
	resultsChan := make(chan struct {
		name   string
		result *s3.ValidationResult
	}, len(vm.validators))

	var wg sync.WaitGroup

	vm.mu.RLock()
	for name, validator := range vm.validators {
		wg.Add(1)
		go func(endpointName string, v bucketValidator) {
			defer wg.Done()
			result := v.ValidateKeys(ctx, vm.timeout)
			resultsChan <- struct {
				name   string
				result *s3.ValidationResult
			}{endpointName, result}
		}(name, validator)
	}
	vm.mu.RUnlock()

	wg.Wait()
	close(resultsChan)

	for item := range resultsChan {
		results.Results[item.name] = item.result
	}

	return results
}

// ValidateEndpoint validates a specific endpoint
func (vm *ValidatorManager) ValidateEndpoint(ctx context.Context, endpointName string) *s3.ValidationResult {
	vm.mu.RLock()
	validator, exists := vm.validators[endpointName]
	vm.mu.RUnlock()

	if !exists {
		return &s3.ValidationResult{
			IsValid:   false,
			Message:   fmt.Sprintf("endpoint '%s' not found", endpointName),
			CheckedAt: time.Now(),
		}
	}

	return validator.ValidateKeys(ctx, vm.timeout)
}

// GetEndpoints returns list of configured endpoint names
func (vm *ValidatorManager) GetEndpoints() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	endpoints := make([]string, 0, len(vm.validators))
	for name := range vm.validators {
		endpoints = append(endpoints, name)
	}
	return endpoints
}

// GetEndpointCount returns the number of configured endpoints
func (vm *ValidatorManager) GetEndpointCount() int {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return len(vm.validators)
}
