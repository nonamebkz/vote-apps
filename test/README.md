# Polling System Test Suite

This directory contains comprehensive tests for the polling system, including integration tests and security validation.

## Test Structure

```
test/
├── integration/           # End-to-end integration tests
│   ├── integration_test.go    # Complete user workflow tests
│   └── websocket_test.go      # WebSocket functionality tests
├── security/             # Security validation tests
│   ├── security_test.go       # General security tests
│   └── jwt_security_test.go   # JWT-specific security tests
├── run_tests.sh          # Test runner script
└── README.md            # This file
```

## Running Tests

### Prerequisites

1. **Go 1.24+** installed
2. **PostgreSQL** running on localhost:5432
3. **Test database** created:
   ```bash
   createuser -s test
   createdb -U test polling_test
   createdb -U test polling_security_test
   ```

### Running All Tests

```bash
# Make the script executable
chmod +x test/run_tests.sh

# Run all test suites
./test/run_tests.sh
```

### Running Individual Test Suites

```bash
# Unit tests only
go test ./pkg/... -v

# Integration tests
go test ./test/integration/... -v

# Security tests
go test ./test/security/... -v

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Test Categories

### 1. Integration Tests (`test/integration/`)

#### Complete User Workflow Tests
- **TestCompleteUserWorkflow**: Tests the full user journey from poll creation to voting
- **TestQRCodeWorkflow**: Tests QR code generation and voting access
- **TestExportFunctionality**: Tests poll result export features
- **TestSecurityValidation**: Basic security validation in integration context

#### WebSocket Tests
- **TestMultipleWebSocketClients**: Tests concurrent WebSocket connections
- **TestWebSocketConnectionManagement**: Tests connection lifecycle
- **TestWebSocketPollStatusUpdates**: Tests real-time status notifications
- **TestWebSocketErrorHandling**: Tests error scenarios
- **TestWebSocketConcurrentVoting**: Tests concurrent voting with real-time updates

### 2. Security Tests (`test/security/`)

#### General Security Tests
- **TestJWTSecurityValidation**: JWT token validation and security
- **TestAuthorizationControls**: Role-based access control
- **TestInputValidationAndSanitization**: SQL injection and XSS prevention
- **TestPasswordSecurity**: Password handling and validation
- **TestSessionSecurity**: Session management security
- **TestDataAccessSecurity**: Data access controls
- **TestSecurityHeaders**: HTTP security headers validation
- **TestRateLimiting**: Rate limiting functionality
- **TestContentTypeValidation**: Content-Type validation

#### JWT-Specific Security Tests
- **TestJWTTokenManipulation**: Token manipulation attack prevention
- **TestJWTTokenLeakage**: Token leakage prevention
- **TestJWTRefreshTokenSecurity**: Refresh token security
- **TestJWTClaimsValidation**: JWT claims validation
- **TestJWTTimingAttacks**: Timing attack vulnerability detection

## Security Test Coverage

The security tests validate the following requirements from the specification:

### JWT Authentication (Requirement 10)
- ✅ Password hashing with bcrypt (10.1)
- ✅ JWT token generation and validation (10.2, 10.3)
- ✅ Authentication failure handling (10.4)
- ✅ Token refresh mechanism (10.5)

### Input Validation and Sanitization
- ✅ SQL injection prevention
- ✅ XSS attack prevention
- ✅ Request size limiting
- ✅ Content-Type validation

### Authorization Controls
- ✅ Role-based access control
- ✅ Admin-only endpoint protection
- ✅ User data access restrictions

### Security Headers
- ✅ X-Content-Type-Options: nosniff
- ✅ X-Frame-Options: DENY
- ✅ X-XSS-Protection: 1; mode=block
- ✅ CORS headers configuration

## Test Environment

### Database Configuration
Tests use separate test databases to avoid affecting development data:
- Integration tests: `polling_test`
- Security tests: `polling_security_test`

### Environment Variables
```bash
ENVIRONMENT=test
DATABASE_URL=postgres://test:test@localhost:5432/polling_test?sslmode=disable
```

### Test Data Cleanup
All tests automatically clean up their data before and after execution to ensure test isolation.

## Continuous Integration

The test suite is designed to work in CI/CD environments:

1. **Database Dependency**: Tests gracefully skip when database is unavailable
2. **Timeout Handling**: All tests have appropriate timeouts
3. **Race Detection**: Tests run with `-race` flag to detect race conditions
4. **Coverage Reporting**: Generates coverage reports in multiple formats

## Test Results Interpretation

### Success Criteria
- All unit tests pass
- Integration tests pass (when database available)
- Security tests pass (when database available)
- No race conditions detected
- Coverage above acceptable threshold

### Common Issues
1. **Database Connection**: Ensure PostgreSQL is running and test databases exist
2. **Port Conflicts**: Ensure test ports are available
3. **Timing Issues**: WebSocket tests may occasionally fail due to timing - retry if needed

## Adding New Tests

### Integration Tests
1. Add test functions to appropriate files in `test/integration/`
2. Use the existing test server setup pattern
3. Clean up test data appropriately
4. Add timeout handling for long-running operations

### Security Tests
1. Add test functions to appropriate files in `test/security/`
2. Use the security test server setup
3. Test both positive and negative security scenarios
4. Document which requirements the test validates

### Test Naming Convention
- Use descriptive test names: `TestFeatureName`
- Group related tests in the same file
- Use subtests for variations: `t.Run("subtest", func(t *testing.T) {...})`

## Performance Considerations

- Integration tests may take longer due to database operations
- WebSocket tests include timing-sensitive operations
- Security tests include intentional attack simulations
- Use `-timeout` flag for long-running test suites

## Troubleshooting

### Database Issues
```bash
# Check if PostgreSQL is running
pg_isready -h localhost -p 5432

# Create test databases
createdb -U test polling_test
createdb -U test polling_security_test
```

### Permission Issues
```bash
# Make test script executable
chmod +x test/run_tests.sh
```

### Import Issues
Ensure all Go modules are properly downloaded:
```bash
go mod download
go mod tidy
```