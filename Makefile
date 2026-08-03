# Customer Dashboard Makefile
# Provides convenient commands for development and testing

.PHONY: help test test-unit test-integration test-coverage test-db-up test-db-down clean lint fmt client-dev

# Default target
help:
	@echo "Customer Dashboard - Available Commands"
	@echo ""
	@echo "Testing:"
	@echo "  make test              - Run all tests (unit + integration if DB available)"
	@echo "  make test-unit         - Run unit tests only (fast, no dependencies)"
	@echo "  make test-integration  - Run integration tests (requires test DB)"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo ""
	@echo "Test Infrastructure:"
	@echo "  make test-db-up        - Start test PostgreSQL container"
	@echo "  make test-db-down      - Stop test PostgreSQL container"
	@echo "  make test-db-reset     - Reset test database (stop, remove, start)"
	@echo ""
	@echo "Development:"
	@echo "  make fmt               - Format Go code"
	@echo "  make lint              - Run linters"
	@echo "  make client-dev        - Run Bun React client on :5173"
	@echo "  make clean             - Clean build artifacts"
	@echo ""

# Test database URL for integration tests
TEST_DATABASE_URL ?= postgres://test:test@localhost:5433/customer_dashboard_test?sslmode=disable

# Run all tests
test: test-unit
	@echo "Running all tests..."
	@if [ -n "$$(docker ps -q -f name=customer-dashboard-test-db)" ]; then \
		echo "Test database running, including integration tests..."; \
		TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -v ./...; \
	else \
		echo "Test database not running, running unit tests only..."; \
		go test -v -short ./...; \
	fi

# Run unit tests only (fast, no external dependencies)
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./...

# Run integration tests (requires test database)
test-integration: test-db-up
	@echo "Running integration tests..."
	@echo "Waiting for database to be ready..."
	@sleep 2
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -v -run Integration ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	go test -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated at coverage/coverage.html"
	go tool cover -func=coverage/coverage.out | tail -1

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Start test database
test-db-up:
	@echo "Starting test database..."
	docker-compose -f docker-compose.test.yml up -d postgres-test
	@echo "Waiting for PostgreSQL to be ready..."
	@timeout 30 sh -c 'until docker exec customer-dashboard-test-db pg_isready -U test; do sleep 1; done'
	@echo "Test database is ready!"
	@echo "Connection URL: $(TEST_DATABASE_URL)"

# Stop test database
test-db-down:
	@echo "Stopping test database..."
	docker-compose -f docker-compose.test.yml down

# Reset test database
test-db-reset: test-db-down
	@echo "Removing test database volume..."
	docker volume rm customer-dashboard_test-postgres-data 2>/dev/null || true
	$(MAKE) test-db-up

# Format Go code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Running go vet instead..."; \
		go vet ./...; \
	fi

# Run React client dev server (Bun + Vite)
client-dev:
	@echo "Starting React client dev server on http://127.0.0.1:5173 ..."
	./scripts/run-client-dev.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf coverage/
	go clean -testcache

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Generate test mocks (if using mockgen)
generate-mocks:
	@echo "Generating mocks..."
	@if command -v mockgen &> /dev/null; then \
		go generate ./...; \
	else \
		echo "mockgen not installed. Install with: go install github.com/golang/mock/mockgen@latest"; \
	fi

# Quick check - format, lint, and unit tests
check: fmt lint test-unit
	@echo "All checks passed!"
