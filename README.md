# AWS S3 Key Exporter

A lightweight Prometheus exporter for validating **multiple AWS S3 credentials** and exposing metrics about key validity and S3 operations.

## Features

- ✅ **Multiple S3 endpoints** validation in a single exporter
- ✅ AWS S3 credentials validation
- ✅ Prometheus metrics export
- ✅ REST API for on-demand validation (all endpoints or specific)
- ✅ Health check endpoint
- ✅ Configurable via environment variables (JSON config for multiple endpoints)
- ✅ Support for custom S3 endpoints (MinIO, etc.)
- ✅ Structured logging with JSON output
- ✅ Parallel validation of multiple endpoints

## Project Structure

```
.
├── cmd/exporter/           # Main application entry point
├── internal/
│   ├── config/            # Configuration management (supports multiple endpoints)
│   ├── exporter/          # Validator manager for multiple endpoints
│   └── handlers/          # HTTP request handlers
├── pkg/
│   ├── s3/                # S3 validation logic
│   └── metrics/           # Prometheus metrics definitions
├── go.mod                 # Go module definition
└── README.md              # This file
```

## Quick Start

### Prerequisites

- Go 1.25 or higher
- AWS S3 buckets with appropriate permissions (or MinIO for local testing)

### Installation

```bash
go mod download
go build -o exporter ./cmd/exporter
```

## Configuration

The exporter supports two configuration modes:

### 1. Single Endpoint (Legacy Mode)

Set environment variables:

```bash
export S3_BUCKET=my-bucket
export S3_ACCESS_KEY=your-access-key
export S3_SECRET_KEY=your-secret-key
export S3_REGION=us-east-1
./exporter
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `S3_BUCKET` | Yes | - | S3 bucket name |
| `S3_ACCESS_KEY` | Yes | - | AWS Access Key ID |
| `S3_SECRET_KEY` | Yes | - | AWS Secret Access Key |
| `S3_REGION` | No | us-east-1 | AWS region |
| `S3_ENDPOINT` | No | - | Custom S3 endpoint |
| `EXPORTER_PORT` | No | 8080 | HTTP server port |
| `VALIDATION_TIMEOUT` | No | 10s | Timeout for validation |
| `AUTO_VALIDATE_INTERVAL` | No | 0s (disabled) | How often to run background validations automatically |

### 2. Multiple Endpoints (JSON Config)

Pass configuration as JSON:

```bash
export S3_ENDPOINTS_JSON='[
  {
    "name": "prod-bucket",
    "endpoint": "",
    "region": "us-east-1",
    "bucket": "prod-bucket-name",
    "access_key": "AKIA...",
    "secret_key": "..."
  },
  {
    "name": "staging-bucket",
    "endpoint": "",
    "region": "eu-west-1",
    "bucket": "staging-bucket-name",
    "access_key": "AKIA...",
    "secret_key": "..."
  },
  {
    "name": "minio-local",
    "endpoint": "http://minio:9000",
    "region": "us-east-1",
    "bucket": "test-bucket",
    "access_key": "minioadmin",
    "secret_key": "minioadmin"
  }
]'

export EXPORTER_PORT=8080
export AUTO_VALIDATE_INTERVAL=30s
./exporter
```

**Configuration Fields:**
- `name` - Unique endpoint identifier (used in URLs and metrics)
- `bucket` - S3 bucket name (required)
- `access_key` - AWS Access Key ID (required)
- `secret_key` - AWS Secret Access Key (required)
- `region` - AWS region (optional, defaults to us-east-1)
- `endpoint` - Custom endpoint URL (optional, for MinIO etc.)

## API Endpoints

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "time": "2024-11-09T10:30:45Z",
  "endpoints": 3
}
```

### Validate All Endpoints

```bash
curl -X POST http://localhost:8080/validate
```

Response (Mixed Success/Failure):
```json
{
  "timestamp": "2024-11-09T10:30:45Z",
  "results": {
    "prod-bucket": {
      "is_valid": true,
      "message": "AWS credentials are valid",
      "checked_at": "2024-11-09T10:30:45Z",
      "response_time_ms": 234
    },
    "staging-bucket": {
      "is_valid": false,
      "message": "S3 validation failed: InvalidAccessKeyId",
      "checked_at": "2024-11-09T10:30:45Z",
      "response_time_ms": 145
    }
  },
  "summary": {
    "total_endpoints": 2,
    "successful": 1,
    "failed": 1
  }
}
```

**Status Codes:**
- `200` - All endpoints valid
- `207` - Mixed (some valid, some failed)
- `401` - All endpoints failed

### Validate Specific Endpoint

```bash
curl -X POST http://localhost:8080/validate/prod-bucket
curl -X GET http://localhost:8080/validate/prod-bucket
```

Response (Single Endpoint):
```json
{
  "is_valid": true,
  "message": "AWS credentials are valid",
  "checked_at": "2024-11-09T10:30:45Z",
  "response_time_ms": 234
}
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

**Available Metrics (per endpoint):**
- `s3_validation_attempts_total{endpoint="..."}` - Total validation attempts
- `s3_validation_success_total{endpoint="..."}` - Successful validations
- `s3_validation_failures_total{endpoint="...", error_type="..."}` - Failed validations
- `s3_validation_duration_seconds{endpoint="..."}` - Validation duration histogram
- `s3_keys_valid{endpoint="..."}` - Current key validity (1=valid, 0=invalid)
- `s3_last_validation_timestamp_seconds{endpoint="..."}` - Last validation timestamp
- `s3_response_time_milliseconds{endpoint="...", operation="..."}` - Response time histogram

## Usage Examples

### Example 1: Single Production Bucket

```bash
export S3_BUCKET=prod-data
export S3_ACCESS_KEY=AKIA...
export S3_SECRET_KEY=...
./exporter
```

### Example 2: Multiple Buckets in Single Exporter

```bash
export S3_ENDPOINTS_JSON='[
  {"name": "prod", "bucket": "prod-data", "access_key": "...", "secret_key": "..."},
  {"name": "dev", "bucket": "dev-data", "access_key": "...", "secret_key": "..."}
]'
./exporter
```

### Example 3: Multiple S3-Compatible Services

```bash
export S3_ENDPOINTS_JSON='[
  {
    "name": "aws-prod",
    "bucket": "prod-bucket",
    "access_key": "AKIA...",
    "secret_key": "...",
    "region": "us-east-1"
  },
  {
    "name": "minio-staging",
    "endpoint": "http://minio.staging:9000",
    "bucket": "staging-bucket",
    "access_key": "minioadmin",
    "secret_key": "minioadmin"
  },
  {
    "name": "digitalocean-spaces",
    "endpoint": "https://nyc3.digitaloceanspaces.com",
    "bucket": "my-space",
    "access_key": "...",
    "secret_key": "...",
    "region": "nyc3"
  }
]'
./exporter
```

## Docker

### Build Docker Image

```bash
docker build -t aws-s3-exporter .
```

### Run Single Endpoint

```bash
docker run -e S3_BUCKET=my-bucket \
           -e S3_ACCESS_KEY=key \
           -e S3_SECRET_KEY=secret \
           -p 8080:8080 \
           aws-s3-exporter
```

### Run Multiple Endpoints

```bash
docker run -e 'S3_ENDPOINTS_JSON=[{"name":"prod","bucket":"prod-bucket","access_key":"...","secret_key":"..."}]' \
           -p 8080:8080 \
           aws-s3-exporter
```

### Docker Compose

```bash
docker-compose up -d
```

## Prometheus Integration

### prometheus.yml Configuration

```yaml
global:
  scrape_interval: 30s

scrape_configs:
  - job_name: 's3-exporter'
    static_configs:
      - targets: ['localhost:8080']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: 's3_.*'
        action: keep
```

### Grafana Dashboard Example

Monitor multiple S3 endpoints:

```
Query: s3_keys_valid{endpoint=~"prod|staging|dev"}
Legend: {{endpoint}} - Validity Status
```

## Development

### Build

```bash
make build
```

### Run with Environment Variables

```bash
make run-with-env
```

### Run Tests

```bash
go test ./...
```

### Run with Docker Compose (includes MinIO)

```bash
docker-compose up -d
```

## Makefile Commands

```bash
make help              # Show all commands
make build             # Build the binary
make run               # Run exporter
make test              # Run tests
make clean             # Clean artifacts
make docker-build      # Build Docker image
make docker-compose-up # Start with docker-compose
```

## Logging

The exporter uses structured JSON logging:

```bash
./exporter  # Info level (default)
```

Each endpoint will log its validation status:

```json
{"endpoint":"prod-bucket","response_time":234,"level":"info","msg":"S3 key validation successful"}
{"endpoint":"staging-bucket","message":"InvalidAccessKeyId","level":"warn","msg":"S3 key validation failed"}
```

## Performance Considerations

- **Parallel Validation**: Multiple endpoints are validated in parallel
- **Timeout**: Configurable per-exporter (not per-endpoint) - default 10s
- **Minimal S3 Operations**: Uses `ListObjectsV2` with MaxKeys=1
- **Concurrent Requests**: Safe for use with multiple concurrent HTTP requests

## Security Best Practices

1. **Don't expose credentials in logs** - Use secure credential management
2. **Use environment variables** - Avoid command-line arguments
3. **Secure communication** - Use HTTPS in production
4. **Network isolation** - Restrict access to exporter port
5. **Credential rotation** - Regularly rotate AWS keys

## Troubleshooting

### Error: "S3_BUCKET environment variable is required"

Provide either single bucket variables OR JSON configuration:

```bash
# Option 1: Single
export S3_BUCKET=my-bucket
export S3_ACCESS_KEY=...
export S3_SECRET_KEY=...

# Option 2: Multiple
export S3_ENDPOINTS_JSON='[...]'
```

### Error: "InvalidAccessKeyId"

Check credentials are correct and have S3 access.

### Error: "NoSuchBucket"

Verify bucket exists in the specified region.

## Notes

- Validation uses `ListObjectsV2` operation to verify credentials
- Only 1 object is fetched to minimize latency and cost
- All metrics include endpoint label for filtering
- Status code 207 indicates partial success for multi-endpoint validation
- Each endpoint is validated independently and in parallel

## License

MIT License

## Contributing

Contributions welcome! Please ensure:
- Code follows Go conventions
- Tests pass
- New features include tests
- Metrics are documented
