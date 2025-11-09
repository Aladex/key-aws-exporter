package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	DefaultPort           = 8080
	DefaultS3Region       = "us-east-1"
	ShutdownTimeout       = 30 * time.Second
	DefaultValidationTimeout = 10 * time.Second
)

// S3EndpointConfig represents configuration for a single S3 endpoint
type S3EndpointConfig struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

type Config struct {
	Port              int
	Endpoints         []S3EndpointConfig
	ValidationTimeout time.Duration
	MetricsPath       string
}

// LoadConfig loads configuration from environment variables
// Supports both single endpoint (legacy) and multiple endpoints (JSON config)
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:              getEnvInt("EXPORTER_PORT", DefaultPort),
		ValidationTimeout: getEnvDuration("VALIDATION_TIMEOUT", DefaultValidationTimeout),
		MetricsPath:       "/metrics",
	}

	// Try to load multiple endpoints from JSON config first
	if endpointsJSON := os.Getenv("S3_ENDPOINTS_JSON"); endpointsJSON != "" {
		var endpoints []S3EndpointConfig
		if err := json.Unmarshal([]byte(endpointsJSON), &endpoints); err != nil {
			return nil, fmt.Errorf("failed to parse S3_ENDPOINTS_JSON: %w", err)
		}

		if len(endpoints) == 0 {
			return nil, fmt.Errorf("S3_ENDPOINTS_JSON must contain at least one endpoint")
		}

		// Set defaults for endpoints
		for i := range endpoints {
			if endpoints[i].Name == "" {
				endpoints[i].Name = endpoints[i].Bucket
			}
			if endpoints[i].Region == "" {
				endpoints[i].Region = DefaultS3Region
			}
			// Validate required fields
			if endpoints[i].Bucket == "" || endpoints[i].AccessKey == "" || endpoints[i].SecretKey == "" {
				return nil, fmt.Errorf("endpoint %d: bucket, access_key, and secret_key are required", i)
			}
		}

		cfg.Endpoints = endpoints
		return cfg, nil
	}

	// Fall back to legacy single endpoint configuration
	singleEndpoint := S3EndpointConfig{
		Endpoint:  getEnv("S3_ENDPOINT", ""),
		Region:    getEnv("S3_REGION", DefaultS3Region),
		Bucket:    getEnv("S3_BUCKET", ""),
		AccessKey: getEnv("S3_ACCESS_KEY", ""),
		SecretKey: getEnv("S3_SECRET_KEY", ""),
	}

	// Validate required fields for legacy mode
	if singleEndpoint.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable is required (or use S3_ENDPOINTS_JSON for multiple endpoints)")
	}

	if singleEndpoint.AccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY environment variable is required")
	}

	if singleEndpoint.SecretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_KEY environment variable is required")
	}

	singleEndpoint.Name = singleEndpoint.Bucket
	cfg.Endpoints = []S3EndpointConfig{singleEndpoint}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return duration
	}
	return defaultValue
}