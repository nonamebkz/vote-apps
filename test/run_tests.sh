#!/bin/bash

# Test runner script for polling system
set -e

echo "=== Polling System Test Suite ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "INFO")
            echo -e "${YELLOW}[INFO]${NC} $message"
            ;;
        "SUCCESS")
            echo -e "${GREEN}[SUCCESS]${NC} $message"
            ;;
        "ERROR")
            echo -e "${RED}[ERROR]${NC} $message"
            ;;
    esac
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_status "ERROR" "Go is not installed or not in PATH"
    exit 1
fi

print_status "INFO" "Go version: $(go version)"

# Set test environment
export ENVIRONMENT=test
export DATABASE_URL="postgres://test:test@localhost:5432/polling_test?sslmode=disable"

# Check if database is available
print_status "INFO" "Checking database connectivity..."
if ! pg_isready -h localhost -p 5432 -U test &> /dev/null; then
    print_status "ERROR" "PostgreSQL database is not available"
    print_status "INFO" "Please ensure PostgreSQL is running and test database exists"
    print_status "INFO" "You can create it with: createdb -U test polling_test"
    exit 1
fi

print_status "SUCCESS" "Database is available"

# Run different test suites
run_test_suite() {
    local suite_name=$1
    local test_path=$2
    local test_flags=$3
    
    print_status "INFO" "Running $suite_name..."
    echo "----------------------------------------"
    
    if go test $test_flags $test_path; then
        print_status "SUCCESS" "$suite_name completed successfully"
    else
        print_status "ERROR" "$suite_name failed"
        return 1
    fi
    echo
}

# Initialize test results
FAILED_SUITES=()

# Run unit tests
if ! run_test_suite "Unit Tests" "./pkg/..." "-v -race"; then
    FAILED_SUITES+=("Unit Tests")
fi

# Run integration tests
if ! run_test_suite "Integration Tests" "./test/integration/..." "-v -timeout=30s"; then
    FAILED_SUITES+=("Integration Tests")
fi

# Run security tests
if ! run_test_suite "Security Tests" "./test/security/..." "-v -timeout=30s"; then
    FAILED_SUITES+=("Security Tests")
fi

# Run all tests with coverage
print_status "INFO" "Running full test suite with coverage..."
echo "----------------------------------------"
if go test -v -race -coverprofile=coverage.out -covermode=atomic ./...; then
    print_status "SUCCESS" "Full test suite completed"
    
    # Generate coverage report
    if command -v go &> /dev/null; then
        print_status "INFO" "Generating coverage report..."
        go tool cover -html=coverage.out -o coverage.html
        print_status "SUCCESS" "Coverage report generated: coverage.html"
        
        # Show coverage summary
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        print_status "INFO" "Total coverage: $COVERAGE"
    fi
else
    FAILED_SUITES+=("Full Test Suite")
fi

echo
echo "=== Test Summary ==="
if [ ${#FAILED_SUITES[@]} -eq 0 ]; then
    print_status "SUCCESS" "All test suites passed!"
    exit 0
else
    print_status "ERROR" "The following test suites failed:"
    for suite in "${FAILED_SUITES[@]}"; do
        echo "  - $suite"
    done
    exit 1
fi