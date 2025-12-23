# Makefile for Polling System

# Variables
BINARY_NAME=polling-system
MAIN_PATH=cmd/server/main.go

# Build the application
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Run the application
run:
	go run $(MAIN_PATH)

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Run database migrations (when implemented)
migrate:
	@echo "Database migrations will be implemented in later tasks"

# Setup development environment
setup: deps
	cp .env.example .env
	@echo "Please edit .env file with your configuration"

# Docker build (for future use)
docker-build:
	docker build -t polling-system .

# Help
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  setup         - Setup development environment"
	@echo "  help          - Show this help message"

.PHONY: build run test test-coverage clean deps fmt lint migrate setup docker-build help