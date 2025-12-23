package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTTokenManipulation tests various JWT token manipulation attacks
func TestJWTTokenManipulation(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Algorithm confusion attack (changing alg to none)
	// Create a token with "none" algorithm
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id": 1,
		"role":    "admin",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	
	noneTokenString, _ := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	
	resp := sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, noneTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token with 'none' algorithm should be rejected")
	}

	// Test 2: Token with modified payload (role escalation)
	// Parse the valid voter token and modify it
	voterTokenParts := strings.Split(sts.voterToken, ".")
	if len(voterTokenParts) == 3 {
		// Decode the payload (this is just for testing - in real attack, attacker would modify)
		// We'll create a new token with escalated privileges
		escalatedClaims := jwt.MapClaims{
			"user_id": 2,
			"role":    "admin", // Escalated from voter to admin
			"exp":     time.Now().Add(time.Hour).Unix(),
		}
		
		escalatedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, escalatedClaims)
		// Sign with wrong secret (simulating attack)
		escalatedTokenString, _ := escalatedToken.SignedString([]byte("wrong-secret"))
		
		resp := sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, escalatedTokenString)
		if resp.StatusCode == http.StatusOK {
			t.Error("Token with modified payload should be rejected")
		}
	}

	// Test 3: Expired token
	expiredClaims := jwt.MapClaims{
		"user_id": 1,
		"role":    "admin",
		"exp":     time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
	}
	
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp = sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, expiredTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Expired token should be rejected")
	}

	// Test 4: Token without expiration
	noExpClaims := jwt.MapClaims{
		"user_id": 1,
		"role":    "admin",
		// No exp claim
	}
	
	noExpToken := jwt.NewWithClaims(jwt.SigningMethodHS256, noExpClaims)
	noExpTokenString, _ := noExpToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp = sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, noExpTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token without expiration should be rejected")
	}

	// Test 5: Token with invalid user_id
	invalidUserClaims := jwt.MapClaims{
		"user_id": "invalid", // Should be number
		"role":    "admin",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	
	invalidUserToken := jwt.NewWithClaims(jwt.SigningMethodHS256, invalidUserClaims)
	invalidUserTokenString, _ := invalidUserToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp = sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, invalidUserTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token with invalid user_id should be rejected")
	}
}

// TestJWTTokenLeakage tests for potential token leakage scenarios
func TestJWTTokenLeakage(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Token not returned in error responses
	resp := sts.makeSecurityRequest(t, "GET", "/api/nonexistent", nil, sts.adminToken)
	
	// Read response body to check for token leakage
	var errorResponse map[string]interface{}
	if resp.Body != nil {
		json.NewDecoder(resp.Body).Decode(&errorResponse)
		
		// Convert response to string and check for token patterns
		responseStr := fmt.Sprintf("%v", errorResponse)
		if strings.Contains(responseStr, sts.adminToken) {
			t.Error("JWT token leaked in error response")
		}
	}

	// Test 2: Token not logged in server responses (check headers)
	for name, values := range resp.Header {
		for _, value := range values {
			if strings.Contains(value, sts.adminToken) {
				t.Errorf("JWT token leaked in response header %s", name)
			}
		}
	}
}

// TestJWTRefreshTokenSecurity tests refresh token functionality if implemented
func TestJWTRefreshTokenSecurity(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test refresh token endpoint
	refreshData := map[string]interface{}{
		"refresh_token": "some-refresh-token",
	}

	resp := sts.makeSecurityRequest(t, "POST", "/api/auth/refresh", refreshData, "")
	
	// If refresh is not implemented, this test will be skipped
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Refresh token endpoint not implemented")
	}

	// Test 1: Invalid refresh token
	invalidRefreshData := map[string]interface{}{
		"refresh_token": "invalid-refresh-token",
	}

	resp = sts.makeSecurityRequest(t, "POST", "/api/auth/refresh", invalidRefreshData, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("Invalid refresh token should be rejected")
	}

	// Test 2: Empty refresh token
	emptyRefreshData := map[string]interface{}{
		"refresh_token": "",
	}

	resp = sts.makeSecurityRequest(t, "POST", "/api/auth/refresh", emptyRefreshData, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("Empty refresh token should be rejected")
	}

	// Test 3: Missing refresh token field
	missingRefreshData := map[string]interface{}{}

	resp = sts.makeSecurityRequest(t, "POST", "/api/auth/refresh", missingRefreshData, "")
	if resp.StatusCode == http.StatusOK {
		t.Error("Missing refresh token should be rejected")
	}
}

// TestJWTClaimsValidation tests JWT claims validation
func TestJWTClaimsValidation(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Token with invalid role
	invalidRoleClaims := jwt.MapClaims{
		"user_id": 1,
		"role":    "superadmin", // Invalid role
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	
	invalidRoleToken := jwt.NewWithClaims(jwt.SigningMethodHS256, invalidRoleClaims)
	invalidRoleTokenString, _ := invalidRoleToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp := sts.makeSecurityRequest(t, "GET", "/api/polls", nil, invalidRoleTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token with invalid role should be rejected")
	}

	// Test 2: Token with missing role
	missingRoleClaims := jwt.MapClaims{
		"user_id": 1,
		"exp":     time.Now().Add(time.Hour).Unix(),
		// Missing role claim
	}
	
	missingRoleToken := jwt.NewWithClaims(jwt.SigningMethodHS256, missingRoleClaims)
	missingRoleTokenString, _ := missingRoleToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, missingRoleTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token with missing role should be rejected")
	}

	// Test 3: Token with missing user_id
	missingUserIDClaims := jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(time.Hour).Unix(),
		// Missing user_id claim
	}
	
	missingUserIDToken := jwt.NewWithClaims(jwt.SigningMethodHS256, missingUserIDClaims)
	missingUserIDTokenString, _ := missingUserIDToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, missingUserIDTokenString)
	if resp.StatusCode == http.StatusOK {
		t.Error("Token with missing user_id should be rejected")
	}

	// Test 4: Token with extra malicious claims
	maliciousClaims := jwt.MapClaims{
		"user_id":    1,
		"role":       "admin",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"is_hacker":  true,
		"admin_override": true,
	}
	
	maliciousToken := jwt.NewWithClaims(jwt.SigningMethodHS256, maliciousClaims)
	maliciousTokenString, _ := maliciousToken.SignedString([]byte("test-secret-key-for-security-testing"))
	
	// This should still work as long as required claims are valid
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, maliciousTokenString)
	if resp.StatusCode != http.StatusOK {
		t.Log("Token with extra claims was rejected (this is acceptable)")
	}
}

// TestJWTTimingAttacks tests for timing attack vulnerabilities
func TestJWTTimingAttacks(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Measure response times for different token validation scenarios
	validToken := sts.adminToken
	invalidToken := "invalid.jwt.token"
	
	// Test multiple times to get average
	iterations := 10
	
	var validTokenTimes []time.Duration
	var invalidTokenTimes []time.Duration
	
	for i := 0; i < iterations; i++ {
		// Time valid token validation
		start := time.Now()
		resp := sts.makeSecurityRequest(t, "GET", "/api/polls", nil, validToken)
		validTokenTimes = append(validTokenTimes, time.Since(start))
		resp.Body.Close()
		
		// Time invalid token validation
		start = time.Now()
		resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, invalidToken)
		invalidTokenTimes = append(invalidTokenTimes, time.Since(start))
		resp.Body.Close()
	}
	
	// Calculate averages
	var validAvg, invalidAvg time.Duration
	for i := 0; i < iterations; i++ {
		validAvg += validTokenTimes[i]
		invalidAvg += invalidTokenTimes[i]
	}
	validAvg /= time.Duration(iterations)
	invalidAvg /= time.Duration(iterations)
	
	// Check if timing difference is significant (more than 100ms difference might indicate timing attack vulnerability)
	timingDiff := validAvg - invalidAvg
	if timingDiff < 0 {
		timingDiff = -timingDiff
	}
	
	if timingDiff > 100*time.Millisecond {
		t.Logf("Warning: Significant timing difference detected: valid=%v, invalid=%v, diff=%v", 
			validAvg, invalidAvg, timingDiff)
		t.Log("This might indicate a timing attack vulnerability")
	} else {
		t.Logf("Timing difference acceptable: valid=%v, invalid=%v, diff=%v", 
			validAvg, invalidAvg, timingDiff)
	}
}