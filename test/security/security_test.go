package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"polling-system/internal/config"
	"polling-system/internal/database"
	"polling-system/internal/handlers"
	"polling-system/internal/middleware"
	"polling-system/internal/repositories"
	"polling-system/internal/services"
	"polling-system/pkg/export"
	"polling-system/pkg/qr"
	"polling-system/pkg/websocket"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type SecurityTestServer struct {
	server     *httptest.Server
	db         *gorm.DB
	router     *mux.Router
	adminToken string
	voterToken string
}

func setupSecurityTestServer(t *testing.T) *SecurityTestServer {
	// Setup test database
	cfg := &config.Config{
		DatabaseURL: "postgres://test:test@localhost:5432/polling_security_test?sslmode=disable",
		JWTSecret:   "test-secret-key-for-security-testing",
		BaseURL:     "http://localhost:8080",
	}

	// Initialize test database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		t.Skipf("Skipping security tests - database not available: %v", err)
	}

	// Clean database
	cleanSecurityDatabase(t, db)

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	pollRepo := repositories.NewPollRepository(db)
	voteRepo := repositories.NewVoteRepository(db)

	// Initialize services
	qrService := qr.NewService(cfg.BaseURL, t.TempDir())
	exportPkgService := export.NewService(t.TempDir())
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	userService := services.NewUserService(userRepo)
	
	// Initialize WebSocket hub and service
	wsHub := websocket.NewHub()
	wsService := services.NewWebSocketService(wsHub)
	go wsHub.Run()
	
	// Initialize poll and vote services
	pollService := services.NewPollService(pollRepo, userRepo, qrService)
	pollService.SetWebSocketService(wsService)
	
	voteService := services.NewVoteService(voteRepo, pollRepo, userRepo)
	voteService.SetWebSocketService(wsService)
	
	exportService := services.NewExportService(pollRepo, userRepo, voteRepo, exportPkgService)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)
	pollHandler := handlers.NewPollHandler(pollService, wsService)
	voteHandler := handlers.NewVoteHandler(pollService, voteService, authService, wsService)
	qrHandler := handlers.NewQRHandler(pollService)
	adminHandler := handlers.NewAdminHandler(userService, pollService, voteService, exportService)

	// Setup router with full security middleware
	router := mux.NewRouter()
	setupSecurityRoutes(router, authHandler, pollHandler, voteHandler, qrHandler, adminHandler)

	// Create test server
	server := httptest.NewServer(router)

	testServer := &SecurityTestServer{
		server: server,
		db:     db,
		router: router,
	}

	// Create test users and get tokens
	testServer.setupSecurityTestUsers(t)

	return testServer
}

func (sts *SecurityTestServer) Close() {
	sts.server.Close()
}

func (sts *SecurityTestServer) setupSecurityTestUsers(t *testing.T) {
	// Create admin user
	adminData := map[string]interface{}{
		"username": "securityadmin",
		"password": "SecureAdmin123!",
		"role":     "admin",
	}
	adminResp := sts.makeSecurityRequest(t, "POST", "/api/auth/register", adminData, "")
	if adminResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create admin user: %d", adminResp.StatusCode)
	}

	// Login admin
	loginData := map[string]interface{}{
		"username": "securityadmin",
		"password": "SecureAdmin123!",
	}
	loginResp := sts.makeSecurityRequest(t, "POST", "/api/auth/login", loginData, "")
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login admin: %d", loginResp.StatusCode)
	}

	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	sts.adminToken = loginResult["token"].(string)

	// Create voter user
	voterData := map[string]interface{}{
		"username": "securityvoter",
		"password": "SecureVoter123!",
		"role":     "voter",
	}
	voterResp := sts.makeSecurityRequest(t, "POST", "/api/auth/register", voterData, "")
	if voterResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create voter user: %d", voterResp.StatusCode)
	}

	// Login voter
	voterLoginData := map[string]interface{}{
		"username": "securityvoter",
		"password": "SecureVoter123!",
	}
	voterLoginResp := sts.makeSecurityRequest(t, "POST", "/api/auth/login", voterLoginData, "")
	if voterLoginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login voter: %d", voterLoginResp.StatusCode)
	}

	var voterLoginResult map[string]interface{}
	json.NewDecoder(voterLoginResp.Body).Decode(&voterLoginResult)
	sts.voterToken = voterLoginResult["token"].(string)
}

func (sts *SecurityTestServer) makeSecurityRequest(t *testing.T, method, path string, data interface{}, token string) *http.Response {
	var body bytes.Buffer
	if data != nil {
		json.NewEncoder(&body).Encode(data)
	}

	req, err := http.NewRequest(method, sts.server.URL+path, &body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

func cleanSecurityDatabase(t *testing.T, db *gorm.DB) {
	tables := []string{"votes", "options", "polls", "users"}
	for _, table := range tables {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}
}

func setupSecurityRoutes(
	router *mux.Router,
	authHandler *handlers.AuthHandler,
	pollHandler *handlers.PollHandler,
	voteHandler *handlers.VoteHandler,
	qrHandler *handlers.QRHandler,
	adminHandler *handlers.AdminHandler,
) {
	// Apply full security middleware stack
	router.Use(middleware.ErrorHandling)
	router.Use(middleware.RequestID)
	router.Use(middleware.SecurityHeaders)
	router.Use(middleware.MaxBodySize(10 << 20))
	router.Use(middleware.NoScriptInjection)
	router.Use(middleware.SQLInjectionPrevention)
	router.Use(middleware.ValidationMiddleware)
	router.Use(middleware.CORS)

	// Auth routes
	auth := router.PathPrefix("/api/auth").Subrouter()
	auth.Use(middleware.ContentTypeValidation("application/json"))
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	auth.HandleFunc("/refresh", authHandler.RefreshToken).Methods("POST")

	// Public routes
	router.HandleFunc("/vote/{poll_id}", voteHandler.VotePage).Methods("GET")
	router.HandleFunc("/api/qr/{poll_id}", qrHandler.GetQRCode).Methods("GET")

	// Protected routes
	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.JWTAuth)

	// Poll routes
	polls := api.PathPrefix("/polls").Subrouter()
	polls.HandleFunc("", pollHandler.ListPolls).Methods("GET")
	polls.HandleFunc("", pollHandler.CreatePoll).Methods("POST")
	polls.HandleFunc("/{id}", pollHandler.GetPoll).Methods("GET")
	polls.HandleFunc("/{id}", pollHandler.UpdatePoll).Methods("PUT")
	polls.HandleFunc("/{id}", pollHandler.DeletePoll).Methods("DELETE")
	polls.HandleFunc("/{id}/start", pollHandler.StartPoll).Methods("POST")
	polls.HandleFunc("/{id}/vote", voteHandler.SubmitVote).Methods("POST")

	// Admin routes
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.AdminOnly)
	admin.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	admin.HandleFunc("/polls/{id}/export", adminHandler.ExportResults).Methods("GET")
}

// TestJWTSecurityValidation tests JWT token security
func TestJWTSecurityValidation(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Access protected endpoint without token
	resp := sts.makeSecurityRequest(t, "GET", "/api/polls", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for missing token, got %d", resp.StatusCode)
	}

	// Test 2: Access with invalid token format
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, "invalid-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token format, got %d", resp.StatusCode)
	}

	// Test 3: Access with malformed JWT
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, "Bearer malformed.jwt.token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for malformed JWT, got %d", resp.StatusCode)
	}

	// Test 4: Access with expired token (simulate by using wrong secret)
	expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, "Bearer "+expiredToken)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for expired/invalid token, got %d", resp.StatusCode)
	}

	// Test 5: Valid token should work
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, sts.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for valid token, got %d", resp.StatusCode)
	}
}

// TestAuthorizationControls tests role-based access control
func TestAuthorizationControls(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Voter accessing admin endpoints
	resp := sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, sts.voterToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for voter accessing admin endpoint, got %d", resp.StatusCode)
	}

	// Test 2: Admin accessing admin endpoints (should work)
	resp = sts.makeSecurityRequest(t, "GET", "/api/admin/users", nil, sts.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for admin accessing admin endpoint, got %d", resp.StatusCode)
	}

	// Test 3: Both roles can access general poll endpoints
	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, sts.voterToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for voter accessing polls, got %d", resp.StatusCode)
	}

	resp = sts.makeSecurityRequest(t, "GET", "/api/polls", nil, sts.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for admin accessing polls, got %d", resp.StatusCode)
	}
}

// TestInputValidationAndSanitization tests input security measures
func TestInputValidationAndSanitization(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: SQL Injection attempts in poll creation
	sqlInjectionPayloads := []string{
		"'; DROP TABLE polls; --",
		"' OR '1'='1",
		"'; INSERT INTO users (username, password) VALUES ('hacker', 'password'); --",
		"' UNION SELECT * FROM users --",
	}

	for _, payload := range sqlInjectionPayloads {
		pollData := map[string]interface{}{
			"title":       payload,
			"description": "SQL injection test",
			"start_date":  time.Now().Format(time.RFC3339),
			"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
			"options": []map[string]interface{}{
				{"option_text": "Option 1"},
			},
		}

		resp := sts.makeSecurityRequest(t, "POST", "/api/polls", pollData, sts.adminToken)
		
		// Should either succeed (if properly sanitized) or fail with validation error
		// Should NOT cause internal server error (500)
		if resp.StatusCode == http.StatusInternalServerError {
			t.Errorf("SQL injection payload '%s' caused server error", payload)
		}
	}

	// Verify database integrity by listing polls
	resp := sts.makeSecurityRequest(t, "GET", "/api/polls", nil, sts.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Error("Database may have been compromised - cannot list polls")
	}

	// Test 2: XSS attempts in poll creation
	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"javascript:alert('xss')",
		"<img src=x onerror=alert('xss')>",
		"<svg onload=alert('xss')>",
	}

	for _, payload := range xssPayloads {
		pollData := map[string]interface{}{
			"title":       "XSS Test",
			"description": payload,
			"start_date":  time.Now().Format(time.RFC3339),
			"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
			"options": []map[string]interface{}{
				{"option_text": payload},
			},
		}

		resp := sts.makeSecurityRequest(t, "POST", "/api/polls", pollData, sts.adminToken)
		
		if resp.StatusCode == http.StatusCreated {
			// If poll was created, verify XSS payload was sanitized
			var createdPoll map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&createdPoll)
			
			description := createdPoll["description"].(string)
			if strings.Contains(description, "<script>") || strings.Contains(description, "javascript:") {
				t.Errorf("XSS payload was not sanitized: %s", description)
			}
		}
	}

	// Test 3: Oversized request body
	largeData := map[string]interface{}{
		"title":       strings.Repeat("A", 1000000), // 1MB title
		"description": "Large data test",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Option 1"},
		},
	}

	resp = sts.makeSecurityRequest(t, "POST", "/api/polls", largeData, sts.adminToken)
	// Should be rejected due to size limits
	if resp.StatusCode != http.StatusRequestEntityTooLarge && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected request to be rejected due to size, got %d", resp.StatusCode)
	}
}

// TestPasswordSecurity tests password handling security
func TestPasswordSecurity(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Weak password rejection
	weakPasswords := []string{
		"123",
		"password",
		"abc",
		"",
	}

	for _, weakPassword := range weakPasswords {
		userData := map[string]interface{}{
			"username": fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
			"password": weakPassword,
			"role":     "voter",
		}

		resp := sts.makeSecurityRequest(t, "POST", "/api/auth/register", userData, "")
		
		// Weak passwords should be rejected
		if resp.StatusCode == http.StatusCreated {
			t.Errorf("Weak password '%s' was accepted", weakPassword)
		}
	}

	// Test 2: Password not returned in responses
	userData := map[string]interface{}{
		"username": fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
		"password": "StrongPassword123!",
		"role":     "voter",
	}

	resp := sts.makeSecurityRequest(t, "POST", "/api/auth/register", userData, "")
	if resp.StatusCode == http.StatusCreated {
		var user map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&user)
		
		if user["password"] != nil {
			t.Error("Password was returned in registration response")
		}
	}

	// Test 3: Login with wrong password
	loginData := map[string]interface{}{
		"username": userData["username"],
		"password": "WrongPassword123!",
	}

	resp = sts.makeSecurityRequest(t, "POST", "/api/auth/login", loginData, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for wrong password, got %d", resp.StatusCode)
	}
}

// TestSessionSecurity tests session and token security
func TestSessionSecurity(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test 1: Token reuse after logout (if logout is implemented)
	// For now, just test that tokens have reasonable expiration

	// Test 2: Multiple login attempts with wrong password
	loginData := map[string]interface{}{
		"username": "securityadmin",
		"password": "WrongPassword123!",
	}

	// Attempt multiple failed logins
	for i := 0; i < 5; i++ {
		resp := sts.makeSecurityRequest(t, "POST", "/api/auth/login", loginData, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for wrong password attempt %d, got %d", i+1, resp.StatusCode)
		}
	}

	// Verify legitimate login still works after failed attempts
	correctLoginData := map[string]interface{}{
		"username": "securityadmin",
		"password": "SecureAdmin123!",
	}

	resp := sts.makeSecurityRequest(t, "POST", "/api/auth/login", correctLoginData, "")
	if resp.StatusCode != http.StatusOK {
		t.Error("Legitimate login failed after multiple wrong attempts")
	}
}

// TestDataAccessSecurity tests data access controls
func TestDataAccessSecurity(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Create a poll as admin
	pollData := map[string]interface{}{
		"title":       "Security Test Poll",
		"description": "Testing data access security",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Secure Option 1"},
			{"option_text": "Secure Option 2"},
		},
	}

	createResp := sts.makeSecurityRequest(t, "POST", "/api/polls", pollData, sts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Test 1: Voter can read poll but not modify it
	resp := sts.makeSecurityRequest(t, "GET", "/api/polls/"+pollID, nil, sts.voterToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Voter should be able to read poll, got %d", resp.StatusCode)
	}

	updateData := map[string]interface{}{
		"title": "Modified by voter",
	}
	resp = sts.makeSecurityRequest(t, "PUT", "/api/polls/"+pollID, updateData, sts.voterToken)
	if resp.StatusCode == http.StatusOK {
		t.Error("Voter should not be able to modify poll")
	}

	// Test 2: Voter cannot delete poll
	resp = sts.makeSecurityRequest(t, "DELETE", "/api/polls/"+pollID, nil, sts.voterToken)
	if resp.StatusCode == http.StatusOK {
		t.Error("Voter should not be able to delete poll")
	}

	// Test 3: Admin can modify and delete
	resp = sts.makeSecurityRequest(t, "PUT", "/api/polls/"+pollID, updateData, sts.adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Admin should be able to modify poll, got %d", resp.StatusCode)
	}
}

// TestSecurityHeaders tests HTTP security headers
func TestSecurityHeaders(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	resp := sts.makeSecurityRequest(t, "GET", "/api/polls", nil, sts.adminToken)

	// Check for security headers
	securityHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}

	for header, expectedValue := range securityHeaders {
		actualValue := resp.Header.Get(header)
		if actualValue == "" {
			t.Errorf("Missing security header: %s", header)
		} else if actualValue != expectedValue {
			t.Errorf("Security header %s: expected '%s', got '%s'", header, expectedValue, actualValue)
		}
	}

	// Check for CORS headers
	corsHeaders := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
	}

	for _, header := range corsHeaders {
		if resp.Header.Get(header) == "" {
			t.Errorf("Missing CORS header: %s", header)
		}
	}
}

// TestRateLimiting tests rate limiting functionality (if implemented)
func TestRateLimiting(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Make rapid requests to test rate limiting
	loginData := map[string]interface{}{
		"username": "nonexistent",
		"password": "wrongpassword",
	}

	successCount := 0
	rateLimitedCount := 0

	// Make 100 rapid requests
	for i := 0; i < 100; i++ {
		resp := sts.makeSecurityRequest(t, "POST", "/api/auth/login", loginData, "")
		
		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		} else if resp.StatusCode == http.StatusUnauthorized {
			successCount++
		}
		
		resp.Body.Close()
	}

	// If rate limiting is implemented, we should see some 429 responses
	if rateLimitedCount == 0 {
		t.Log("No rate limiting detected (may not be implemented)")
	} else {
		t.Logf("Rate limiting working: %d requests rate limited out of 100", rateLimitedCount)
	}
}

// TestContentTypeValidation tests content type validation
func TestContentTypeValidation(t *testing.T) {
	sts := setupSecurityTestServer(t)
	defer sts.Close()

	// Test sending JSON data without proper Content-Type header
	pollData := `{"title":"Test","description":"Test","start_date":"2024-01-01T00:00:00Z","end_date":"2024-01-02T00:00:00Z","options":[{"option_text":"Option 1"}]}`
	
	req, _ := http.NewRequest("POST", sts.server.URL+"/api/polls", strings.NewReader(pollData))
	req.Header.Set("Authorization", "Bearer "+sts.adminToken)
	// Intentionally not setting Content-Type

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Should be rejected due to missing/wrong content type
	if resp.StatusCode == http.StatusCreated {
		t.Error("Request without proper Content-Type should be rejected")
	}

	// Test with correct Content-Type
	req2, _ := http.NewRequest("POST", sts.server.URL+"/api/polls", strings.NewReader(pollData))
	req2.Header.Set("Authorization", "Bearer "+sts.adminToken)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("Request with proper Content-Type should succeed, got %d", resp2.StatusCode)
	}
}