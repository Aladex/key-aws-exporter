.PHONY: help build run test clean docker-build docker-run docker-stop lint fmt

BINARY_NAME=exporter
GO_FILES=$(shell find . -name "*.go" -type f)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the exporter binary
	go build -o $(BINARY_NAME) ./cmd/exporter

run: build ## Build and run the exporter
	./$(BINARY_NAME)

run-with-env: build ## Run with environment variables for testing
	S3_BUCKET=test-bucket \
	S3_ACCESS_KEY=test-key \
	S3_SECRET_KEY=test-secret \
	S3_REGION=us-east-1 \
	EXPORTER_PORT=8080 \
	./$(BINARY_NAME)

test: ## Run tests
	go test -v -cover ./...

test-short: ## Run short tests
	go test -short -v ./...

coverage: ## Generate test coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	go clean

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

check: vet fmt ## Run all checks

docker-build: ## Build Docker image
	docker build -t aws-s3-exporter:latest .

docker-run: ## Run Docker container
	docker run -e S3_BUCKET=test-bucket \
	           -e S3_ACCESS_KEY=test-key \
	           -e S3_SECRET_KEY=test-secret \
	           -p 8080:8080 \
	           aws-s3-exporter:latest

docker-stop: ## Stop Docker container
	docker stop $$(docker ps -q --filter ancestor=aws-s3-exporter:latest)

docker-compose-up: ## Start services with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop services with docker-compose
	docker-compose down

docker-compose-logs: ## View docker-compose logs
	docker-compose logs -f

deps: ## Download dependencies
	go mod download
	go mod tidy

docs: ## Generate documentation
	@echo "Documentation is in README.md"

all: clean test build ## Build everything