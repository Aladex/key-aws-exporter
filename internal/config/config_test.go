package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_MultipleEndpointsJSON(t *testing.T) {
	t.Setenv("S3_ENDPOINTS_JSON", `[{"name":"primary","endpoint":"https://s3.example.com","region":"eu-west-1","bucket":"bucket-a","access_key":"AKIA","secret_key":"SECRET"},{"bucket":"bucket-b","access_key":"AKIA2","secret_key":"SECRET2"}]`)
	t.Setenv("EXPORTER_PORT", "9090")
	t.Setenv("VALIDATION_TIMEOUT", "5s")
	t.Setenv("AUTO_VALIDATE_INTERVAL", "2m")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}

	if cfg.ValidationTimeout != 5*time.Second {
		t.Fatalf("expected validation timeout 5s, got %v", cfg.ValidationTimeout)
	}

	if cfg.AutoValidateInterval != 2*time.Minute {
		t.Fatalf("expected auto interval 2m, got %v", cfg.AutoValidateInterval)
	}

	if len(cfg.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}

	if cfg.Endpoints[0].Name != "primary" {
		t.Fatalf("expected first endpoint name 'primary', got %s", cfg.Endpoints[0].Name)
	}

	if cfg.Endpoints[1].Name != "bucket-b" {
		t.Fatalf("expected second endpoint name to default to bucket name, got %s", cfg.Endpoints[1].Name)
	}

	if cfg.Endpoints[1].Region != DefaultS3Region {
		t.Fatalf("expected default region %s, got %s", DefaultS3Region, cfg.Endpoints[1].Region)
	}
}

func TestLoadConfig_LegacyConfig(t *testing.T) {
	t.Setenv("S3_ENDPOINTS_JSON", "")
	t.Setenv("S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("S3_REGION", "us-west-2")
	t.Setenv("S3_BUCKET", "legacy-bucket")
	t.Setenv("S3_ACCESS_KEY", "ACCESS")
	t.Setenv("S3_SECRET_KEY", "SECRET")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cfg.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(cfg.Endpoints))
	}

	endpoint := cfg.Endpoints[0]
	if endpoint.Name != "legacy-bucket" {
		t.Fatalf("expected endpoint name to default to bucket, got %s", endpoint.Name)
	}

	if endpoint.Endpoint != "https://s3.example.com" {
		t.Fatalf("unexpected endpoint: %s", endpoint.Endpoint)
	}

	if endpoint.Region != "us-west-2" {
		t.Fatalf("unexpected region: %s", endpoint.Region)
	}

	if cfg.AutoValidateInterval != 0 {
		t.Fatalf("expected default auto interval 0, got %v", cfg.AutoValidateInterval)
	}
}

func TestLoadConfig_LegacyMissingValues(t *testing.T) {
	tests := []struct {
		name   string
		bucket string
		access string
		secret string
	}{
		{name: "missing bucket", bucket: "", access: "AKIA", secret: "SECRET"},
		{name: "missing access", bucket: "bucket", access: "", secret: "SECRET"},
		{name: "missing secret", bucket: "bucket", access: "AKIA", secret: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("S3_ENDPOINTS_JSON", "")
			t.Setenv("S3_BUCKET", tt.bucket)
			t.Setenv("S3_ACCESS_KEY", tt.access)
			t.Setenv("S3_SECRET_KEY", tt.secret)
			t.Setenv("S3_REGION", "")
			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected error when %s", tt.name)
			}
		})
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	t.Setenv("S3_ENDPOINTS_JSON", "not-a-json")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error for invalid JSON config")
	}
}

func TestLoadConfig_LoadsDotEnv(t *testing.T) {
	// Use a temp dir to avoid touching the real project .env
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tempDir := t.TempDir()
	t.Cleanup(func() {
		os.Chdir(wd)
	})

	dotEnvContent := "S3_BUCKET=dotenv-bucket\nS3_ACCESS_KEY=KEY\nS3_SECRET_KEY=SECRET\nAUTO_VALIDATE_INTERVAL=15s\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(dotEnvContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .env: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Ensure JSON mode is disabled so legacy env path is used
	t.Setenv("S3_ENDPOINTS_JSON", "")
	t.Setenv("S3_REGION", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cfg.Endpoints) != 1 {
		t.Fatalf("expected a single endpoint from .env, got %d", len(cfg.Endpoints))
	}

	endpoint := cfg.Endpoints[0]
	if endpoint.Bucket != "dotenv-bucket" {
		t.Fatalf("expected bucket from .env, got %s", endpoint.Bucket)
	}
	if endpoint.AccessKey != "KEY" || endpoint.SecretKey != "SECRET" {
		t.Fatalf("expected credentials from .env")
	}
	if cfg.AutoValidateInterval != 15*time.Second {
		t.Fatalf("expected auto interval from .env, got %v", cfg.AutoValidateInterval)
	}
}
