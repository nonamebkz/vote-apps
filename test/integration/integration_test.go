package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	gorillaws "github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type TestServer struct {
	server     *httptest.Server
	db         *gorm.DB
	router     *mux.Router
	wsHub      *websocket.Hub
	adminToken string
	voterToken string
}

func setupTestServer(t *testing.T) *TestServer {
	// Setup test database
	cfg := &config.Config{
		DatabaseURL: "postgres://test:test@localhost:5432/polling_test?sslmode=disable",
		JWTSecret:   "test-secret-key-for-testing-only",
		BaseURL:     "http://localhost:8080",
	}

	// Initialize test database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		t.Skipf("Skipping integration tests - database not available: %v", err)
	}

	// Clean database before each test
	cleanDatabase(t, db)

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

	// Initialize poll and vote services with WebSocket integration
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
	wsHandler := handlers.NewWebSocketHandler(wsHub)

	// Setup router
	router := mux.NewRouter()
	setupTestRoutes(router, authHandler, pollHandler, voteHandler, qrHandler, adminHandler, wsHandler)

	// Create test server
	server := httptest.NewServer(router)

	testServer := &TestServer{
		server: server,
		db:     db,
		router: router,
		wsHub:  wsHub,
	}

	// Create test users and get tokens
	testServer.setupTestUsers(t)

	return testServer
}

func (ts *TestServer) Close() {
	ts.server.Close()
	ts.wsHub.Stop()
}

func (ts *TestServer) setupTestUsers(t *testing.T) {
	// Create admin user
	adminData := map[string]interface{}{
		"username": "admin",
		"password": "admin123",
		"role":     "admin",
	}
	adminResp := ts.makeRequest(t, "POST", "/api/auth/register", adminData, "")
	if adminResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create admin user: %d", adminResp.StatusCode)
	}

	// Login admin
	loginData := map[string]interface{}{
		"username": "admin",
		"password": "admin123",
	}
	loginResp := ts.makeRequest(t, "POST", "/api/auth/login", loginData, "")
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login admin: %d", loginResp.StatusCode)
	}

	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	ts.adminToken = loginResult["token"].(string)

	// Create voter user
	voterData := map[string]interface{}{
		"username": "voter1",
		"password": "voter123",
		"role":     "voter",
	}
	voterResp := ts.makeRequest(t, "POST", "/api/auth/register", voterData, "")
	if voterResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create voter user: %d", voterResp.StatusCode)
	}

	// Login voter
	voterLoginData := map[string]interface{}{
		"username": "voter1",
		"password": "voter123",
	}
	voterLoginResp := ts.makeRequest(t, "POST", "/api/auth/login", voterLoginData, "")
	if voterLoginResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login voter: %d", voterLoginResp.StatusCode)
	}

	var voterLoginResult map[string]interface{}
	json.NewDecoder(voterLoginResp.Body).Decode(&voterLoginResult)
	ts.voterToken = voterLoginResult["token"].(string)
}

func (ts *TestServer) makeRequest(t *testing.T, method, path string, data interface{}, token string) *http.Response {
	var body bytes.Buffer
	if data != nil {
		json.NewEncoder(&body).Encode(data)
	}

	req, err := http.NewRequest(method, ts.server.URL+path, &body)
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

func cleanDatabase(t *testing.T, db *gorm.DB) {
	// Clean tables in correct order (respecting foreign keys)
	tables := []string{"votes", "options", "polls", "users"}
	for _, table := range tables {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Logf("Warning: Failed to clean table %s: %v", table, err)
		}
	}
}

func setupTestRoutes(
	router *mux.Router,
	authHandler *handlers.AuthHandler,
	pollHandler *handlers.PollHandler,
	voteHandler *handlers.VoteHandler,
	qrHandler *handlers.QRHandler,
	adminHandler *handlers.AdminHandler,
	wsHandler *handlers.WebSocketHandler,
) {
	// Minimal middleware for testing
	router.Use(middleware.ErrorHandling)
	router.Use(middleware.CORS)

	// Auth routes
	auth := router.PathPrefix("/api/auth").Subrouter()
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
	polls.HandleFunc("/{id}/pause", pollHandler.PausePoll).Methods("POST")
	polls.HandleFunc("/{id}/stop", pollHandler.StopPoll).Methods("POST")
	polls.HandleFunc("/{id}/vote", voteHandler.SubmitVote).Methods("POST")
	polls.HandleFunc("/{id}/vote", voteHandler.GetVotePage).Methods("GET")

	// User routes
	users := api.PathPrefix("/users").Subrouter()
	users.HandleFunc("/{id}/votes", voteHandler.GetUserVotes).Methods("GET")

	// Admin routes
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.AdminOnly)
	admin.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	admin.HandleFunc("/polls/{id}/participation", adminHandler.GetParticipation).Methods("GET")
	admin.HandleFunc("/polls/{id}/export", adminHandler.ExportResults).Methods("GET")

	// WebSocket route
	router.HandleFunc("/ws/{poll_id}", wsHandler.HandleConnection)
}

// TestCompleteUserWorkflow tests the complete user workflow from poll creation to voting
func TestCompleteUserWorkflow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Step 1: Admin creates a poll
	pollData := map[string]interface{}{
		"title":       "Test Poll",
		"description": "Integration test poll",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Option 1"},
			{"option_text": "Option 2"},
			{"option_text": "Option 3"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Step 2: Admin starts the poll
	startResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to start poll: %d", startResp.StatusCode)
	}

	// Step 3: Voter accesses the poll
	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.voterToken)
	if getPollResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get poll: %d", getPollResp.StatusCode)
	}

	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)

	// Verify poll is active
	if poll["is_active"] != true {
		t.Errorf("Expected poll to be active, got %v", poll["is_active"])
	}

	// Step 4: Voter submits a vote
	options := poll["options"].([]interface{})
	firstOption := options[0].(map[string]interface{})
	optionID := fmt.Sprintf("%.0f", firstOption["id"].(float64))

	voteData := map[string]interface{}{
		"option_id": optionID,
	}

	voteResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)
	if voteResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to submit vote: %d", voteResp.StatusCode)
	}

	// Step 5: Verify vote was recorded
	updatedPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.adminToken)
	if updatedPollResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get updated poll: %d", updatedPollResp.StatusCode)
	}

	var updatedPoll map[string]interface{}
	json.NewDecoder(updatedPollResp.Body).Decode(&updatedPoll)

	updatedOptions := updatedPoll["options"].([]interface{})
	updatedFirstOption := updatedOptions[0].(map[string]interface{})

	if updatedFirstOption["vote_count"].(float64) != 1 {
		t.Errorf("Expected vote count to be 1, got %v", updatedFirstOption["vote_count"])
	}

	// Step 6: Test anti-double vote mechanism
	duplicateVoteResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)
	if duplicateVoteResp.StatusCode != http.StatusConflict {
		t.Errorf("Expected duplicate vote to be rejected with 409, got %d", duplicateVoteResp.StatusCode)
	}

	// Step 7: Admin stops the poll
	stopResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/stop", nil, ts.adminToken)
	if stopResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to stop poll: %d", stopResp.StatusCode)
	}

	// Step 8: Verify voting is no longer allowed
	newVoteResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)
	if newVoteResp.StatusCode == http.StatusCreated {
		t.Error("Expected voting to be disabled after poll stop")
	}
}

// TestQRCodeWorkflow tests the QR code generation and access workflow
func TestQRCodeWorkflow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create and start a poll
	pollData := map[string]interface{}{
		"title":       "QR Test Poll",
		"description": "Testing QR code functionality",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Yes"},
			{"option_text": "No"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Start the poll
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	// Test QR code generation
	qrResp := ts.makeRequest(t, "GET", "/api/qr/"+pollID, nil, "")
	if qrResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get QR code: %d", qrResp.StatusCode)
	}

	var qrResult map[string]interface{}
	json.NewDecoder(qrResp.Body).Decode(&qrResult)

	if qrResult["qr_code"] == nil || qrResult["qr_code"] == "" {
		t.Error("Expected QR code to be generated")
	}

	if qrResult["voting_url"] == nil || qrResult["voting_url"] == "" {
		t.Error("Expected voting URL to be generated")
	}

	// Test voting page access via QR code URL
	votingURL := "/vote/" + pollID
	votePageResp := ts.makeRequest(t, "GET", votingURL, nil, "")

	// Should redirect to login or show voting page
	if votePageResp.StatusCode != http.StatusOK && votePageResp.StatusCode != http.StatusFound {
		t.Errorf("Expected voting page to be accessible, got status %d", votePageResp.StatusCode)
	}
}

// TestWebSocketFunctionality tests real-time WebSocket updates
func TestWebSocketFunctionality(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create and start a poll
	pollData := map[string]interface{}{
		"title":       "WebSocket Test Poll",
		"description": "Testing real-time updates",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Option A"},
			{"option_text": "Option B"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Start the poll
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	// Connect to WebSocket
	wsURL := "ws" + ts.server.URL[4:] + "/ws/" + pollID // Replace http with ws
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Set up message reading in goroutine
	messages := make(chan map[string]interface{}, 10)
	go func() {
		for {
			var msg map[string]interface{}
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}
			messages <- msg
		}
	}()

	// Submit a vote to trigger WebSocket update
	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.voterToken)
	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)

	options := poll["options"].([]interface{})
	firstOption := options[0].(map[string]interface{})
	optionID := fmt.Sprintf("%.0f", firstOption["id"].(float64))

	voteData := map[string]interface{}{
		"option_id": optionID,
	}

	voteResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)
	if voteResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to submit vote: %d", voteResp.StatusCode)
	}

	// Wait for WebSocket message
	select {
	case msg := <-messages:
		if msg["type"] != "vote_update" {
			t.Errorf("Expected vote_update message, got %v", msg["type"])
		}

		if msg["poll_id"] != pollID {
			t.Errorf("Expected poll_id %s, got %v", pollID, msg["poll_id"])
		}

	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for WebSocket message")
	}
}

// TestSecurityValidation tests basic security measures
func TestSecurityValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Test unauthorized access to protected endpoints
	unauthorizedResp := ts.makeRequest(t, "GET", "/api/polls", nil, "")
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthorized access, got %d", unauthorizedResp.StatusCode)
	}

	// Test invalid JWT token
	invalidTokenResp := ts.makeRequest(t, "GET", "/api/polls", nil, "invalid-token")
	if invalidTokenResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d", invalidTokenResp.StatusCode)
	}

	// Test voter accessing admin endpoints
	voterAdminResp := ts.makeRequest(t, "GET", "/api/admin/users", nil, ts.voterToken)
	if voterAdminResp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for voter accessing admin endpoint, got %d", voterAdminResp.StatusCode)
	}

	// Test SQL injection prevention in poll creation
	maliciousData := map[string]interface{}{
		"title":       "'; DROP TABLE polls; --",
		"description": "SQL injection test",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Option 1"},
		},
	}

	maliciousResp := ts.makeRequest(t, "POST", "/api/polls", maliciousData, ts.adminToken)
	// Should either succeed (if properly sanitized) or fail with validation error
	if maliciousResp.StatusCode == http.StatusInternalServerError {
		t.Error("SQL injection may have caused server error")
	}

	// Verify polls table still exists by listing polls
	listResp := ts.makeRequest(t, "GET", "/api/polls", nil, ts.adminToken)
	if listResp.StatusCode != http.StatusOK {
		t.Error("Polls table may have been compromised by SQL injection")
	}
}

// TestExportFunctionality tests the export feature
func TestExportFunctionality(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create poll with votes
	pollData := map[string]interface{}{
		"title":       "Export Test Poll",
		"description": "Testing export functionality",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Export Option 1"},
			{"option_text": "Export Option 2"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Start poll and add vote
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.voterToken)
	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)

	options := poll["options"].([]interface{})
	firstOption := options[0].(map[string]interface{})
	optionID := fmt.Sprintf("%.0f", firstOption["id"].(float64))

	voteData := map[string]interface{}{
		"option_id": optionID,
	}
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)

	// Test export functionality
	exportResp := ts.makeRequest(t, "GET", "/api/admin/polls/"+pollID+"/export?format=pdf", nil, ts.adminToken)
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to export poll results: %d", exportResp.StatusCode)
	}

	var exportResult map[string]interface{}
	json.NewDecoder(exportResp.Body).Decode(&exportResult)

	if exportResult["download_url"] == nil {
		t.Error("Expected download URL in export response")
	}
}

func TestMain(m *testing.M) {
	// Setup test environment
	os.Setenv("ENVIRONMENT", "test")

	// Run tests
	code := m.Run()

	// Cleanup
	os.Exit(code)
}
