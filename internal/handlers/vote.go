package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"polling-system/internal/middleware"
	"polling-system/internal/models"
	"polling-system/internal/services"

	"github.com/gorilla/mux"
)

// VoteHandler handles voting related requests
type VoteHandler struct {
	pollService *services.PollService
	voteService *services.VoteService
	authService *services.AuthService
	wsHub       *services.WebSocketService
	templates   *template.Template
}

// NewVoteHandler creates a new vote handler
func NewVoteHandler(pollService *services.PollService, voteService *services.VoteService, authService *services.AuthService, wsHub *services.WebSocketService) *VoteHandler {
	return &VoteHandler{
		pollService: pollService,
		voteService: voteService,
		authService: authService,
		wsHub:       wsHub,
	}
}

// SubmitVoteRequest represents a vote submission request
type SubmitVoteRequest struct {
	OptionID uint `json:"option_id" validate:"required"`
}

// SubmitVote handles vote submission
func (h *VoteHandler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Parse request body
	var req SubmitVoteRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.OptionID == 0 {
		WriteError(w, http.StatusBadRequest, "MISSING_OPTION", "Option ID is required", nil)
		return
	}

	// Convert to service request
	serviceReq := services.SubmitVoteRequest{
		PollID:   pollID,
		OptionID: req.OptionID,
	}

	// Submit vote
	vote, err := h.voteService.SubmitVote(claims.UserID, serviceReq)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "VOTE_FAILED", "Failed to submit vote", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, vote, "Vote submitted successfully")
}

// GetVotePage handles getting the vote page (API endpoint)
func (h *VoteHandler) GetVotePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Get poll details
	poll, err := h.pollService.GetPollWithOptions(pollID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "POLL_NOT_FOUND", "Poll not found", map[string]string{"error": err.Error()})
		return
	}

	// Check if user has already voted
	hasVoted, err := h.voteService.HasUserVoted(claims.UserID, pollID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "CHECK_FAILED", "Failed to check vote status", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"poll":      poll,
		"has_voted": hasVoted,
	}, "Poll details for voting retrieved successfully")
}

// VotePage handles the public vote page (via QR code) - HTML response
func (h *VoteHandler) VotePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Check if user is authenticated
	userID, authenticated := h.getUserFromContext(r)
	if !authenticated {
		// Redirect to login with return URL
		returnURL := fmt.Sprintf("/vote/%d", pollID)
		loginURL := fmt.Sprintf("/login?return_url=%s", url.QueryEscape(returnURL))
		http.Redirect(w, r, loginURL, http.StatusFound)
		return
	}

	// Get poll details
	poll, err := h.pollService.GetPollWithOptions(uint(pollID))
	if err != nil {
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	// Check if poll is active and can accept votes
	if !poll.CanAcceptVotes() {
		h.renderVotePageClosed(w, poll)
		return
	}

	// Check if user can access this poll
	canAccess, err := h.pollService.CanUserAccessPoll(userID, uint(pollID))
	if err != nil {
		http.Error(w, "Error checking poll access", http.StatusInternalServerError)
		return
	}

	if !canAccess {
		http.Error(w, "You don't have permission to access this poll", http.StatusForbidden)
		return
	}

	// Check if user has already voted
	hasVoted, err := h.voteService.HasUserVoted(userID, uint(pollID))
	if err != nil {
		http.Error(w, "Error checking vote status", http.StatusInternalServerError)
		return
	}

	if hasVoted {
		h.renderVotePageAlreadyVoted(w, poll)
		return
	}

	// Render voting page
	h.renderVotePage(w, poll, userID)
}

// GetUserVotes handles getting user's vote history
func (h *VoteHandler) GetUserVotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Get query parameters for pagination
	page := GetQueryParamInt(r, "page", 1)
	limit := GetQueryParamInt(r, "limit", 10)

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	// Build filter
	filter := services.VoteHistoryFilter{
		UserID: claims.UserID,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}

	// Get user's vote history
	votes, total, err := h.voteService.GetVoteHistory(filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "HISTORY_FAILED", "Failed to get vote history", map[string]string{"error": err.Error()})
		return
	}

	// Calculate pagination info
	totalPages := (total + int64(limit) - 1) / int64(limit)

	WriteSuccess(w, map[string]interface{}{
		"votes": votes,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	}, "Vote history retrieved successfully")
}

// SubmitVoteForm handles form-based vote submission (for HTML forms)
func (h *VoteHandler) SubmitVoteForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Get user from context
	userID, authenticated := h.getUserFromContext(r)
	if !authenticated {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	optionIDStr := r.FormValue("option_id")
	if optionIDStr == "" {
		http.Error(w, "Option ID is required", http.StatusBadRequest)
		return
	}

	optionID, err := strconv.ParseUint(optionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid option ID", http.StatusBadRequest)
		return
	}

	// Submit vote
	serviceReq := services.SubmitVoteRequest{
		PollID:   pollID,
		OptionID: uint(optionID),
	}

	_, err = h.voteService.SubmitVote(userID, serviceReq)
	if err != nil {
		// Log error and redirect to an error page or show error in same page
		// For simplicity, we'll redirect back to the poll page with an error
		errURL := fmt.Sprintf("/vote/%d?error=%s", pollID, url.QueryEscape(err.Error()))
		http.Redirect(w, r, errURL, http.StatusFound)
		return
	}

	// Redirect to success page
	successURL := fmt.Sprintf("/polls/%d/results", pollID)
	http.Redirect(w, r, successURL, http.StatusFound)
}

// VoteSuccessPage renders the vote success page
func (h *VoteHandler) VoteSuccessPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Get poll details
	poll, err := h.pollService.GetPollWithOptions(uint(pollID))
	if err != nil {
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	h.renderVoteSuccessPage(w, poll)
}

// getUserFromContext extracts user ID from request context (JWT middleware should set this)
func (h *VoteHandler) getUserFromContext(r *http.Request) (uint, bool) {
	// Try to get from JWT middleware context first
	claims, ok := middleware.GetUserFromContext(r)
	if ok {
		return claims.UserID, true
	}

	// Fallback: check for user ID in context (for compatibility)
	userID := r.Context().Value("user_id")
	if userID == nil {
		return 0, false
	}

	if id, ok := userID.(uint); ok {
		return id, true
	}

	return 0, false
}

// renderVotePage renders the voting page with poll options
func (h *VoteHandler) renderVotePage(w http.ResponseWriter, poll *models.Poll, userID uint) {
	// For now, render a simple HTML response
	// In a full implementation, this would use proper templates
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Vote - %s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; }
        .poll-title { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        .poll-description { color: #666; margin: 20px 0; }
        .options { margin: 20px 0; }
        .option { margin: 10px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; cursor: pointer; }
        .option:hover { background-color: #f8f9fa; }
        .option input { margin-right: 10px; }
        .submit-btn { background-color: #007bff; color: white; padding: 12px 24px; border: none; border-radius: 5px; cursor: pointer; font-size: 16px; }
        .submit-btn:hover { background-color: #0056b3; }
        .poll-info { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1 class="poll-title">%s</h1>
    <div class="poll-description">%s</div>
    
    <div class="poll-info">
        <strong>Poll Status:</strong> %s<br>
        <strong>End Date:</strong> %s
    </div>
    
    <form method="POST" action="/api/polls/%d/vote">
        <div class="options">
            <h3>Select your choice:</h3>
`, poll.Title, poll.Title, poll.Description, poll.Status, poll.EndDate.Format("2006-01-02 15:04:05"), poll.ID)

	// Add options
	for _, option := range poll.Options {
		html += fmt.Sprintf(`
            <div class="option">
                <label>
                    <input type="radio" name="option_id" value="%d" required>
                    %s
                </label>
            </div>
`, option.ID, option.OptionText)
	}

	html += `
        </div>
        <button type="submit" class="submit-btn">Submit Vote</button>
    </form>
    
    <script>
        // Add some basic interactivity
        document.querySelectorAll('.option').forEach(option => {
            option.addEventListener('click', function() {
                const radio = this.querySelector('input[type="radio"]');
                radio.checked = true;
            });
        });
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// renderVotePageClosed renders the page when poll is closed
func (h *VoteHandler) renderVotePageClosed(w http.ResponseWriter, poll *models.Poll) {
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Poll Closed - %s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; text-align: center; }
        .closed-message { background-color: #f8d7da; color: #721c24; padding: 20px; border-radius: 5px; margin: 20px 0; }
        .poll-info { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <div class="closed-message">
        <h2>This poll is currently closed</h2>
        <p>Voting is not available at this time.</p>
    </div>
    
    <div class="poll-info">
        <strong>Poll Status:</strong> %s<br>
        <strong>Start Date:</strong> %s<br>
        <strong>End Date:</strong> %s
    </div>
    
    <p><a href="/">Return to Home</a></p>
</body>
</html>`, poll.Title, poll.Title, poll.Status, poll.StartDate.Format("2006-01-02 15:04:05"), poll.EndDate.Format("2006-01-02 15:04:05"))

	w.Write([]byte(html))
}

// HandleLoginRedirect handles the login page with return URL
func (h *VoteHandler) HandleLoginRedirect(w http.ResponseWriter, r *http.Request) {
	returnURL := r.URL.Query().Get("return_url")
	if returnURL == "" {
		returnURL = "/"
	}

	// Render login page with return URL
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Login Required</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 0 auto; padding: 20px; }
        .login-form { background-color: #f8f9fa; padding: 30px; border-radius: 10px; }
        .form-group { margin: 15px 0; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input { width: 100%%; padding: 10px; border: 1px solid #ddd; border-radius: 5px; box-sizing: border-box; }
        .submit-btn { background-color: #007bff; color: white; padding: 12px 24px; border: none; border-radius: 5px; cursor: pointer; font-size: 16px; width: 100%%; }
        .submit-btn:hover { background-color: #0056b3; }
        .message { background-color: #d1ecf1; color: #0c5460; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="login-form">
        <h2>Login Required</h2>
        <div class="message">
            Please log in to access the voting page.
        </div>
        
        <form method="POST" action="/api/auth/login">
            <input type="hidden" name="return_url" value="%s">
            
            <div class="form-group">
                <label for="username">Username:</label>
                <input type="text" id="username" name="username" required>
            </div>
            
            <div class="form-group">
                <label for="password">Password:</label>
                <input type="password" id="password" name="password" required>
            </div>
            
            <button type="submit" class="submit-btn">Login</button>
        </form>
        
        <p style="text-align: center; margin-top: 20px;">
            Don't have an account? <a href="/register?return_url=%s">Register here</a>
        </p>
    </div>
</body>
</html>`, returnURL, url.QueryEscape(returnURL))

	w.Write([]byte(html))
}

// renderVotePageAlreadyVoted renders the page when user has already voted
func (h *VoteHandler) renderVotePageAlreadyVoted(w http.ResponseWriter, poll *models.Poll) {
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Already Voted - %s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; text-align: center; }
        .voted-message { background-color: #d4edda; color: #155724; padding: 20px; border-radius: 5px; margin: 20px 0; }
        .poll-info { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .results-link { background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; display: inline-block; margin: 10px; }
        .results-link:hover { background-color: #0056b3; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <div class="voted-message">
        <h2>Thank you for voting!</h2>
        <p>You have already submitted your vote for this poll.</p>
    </div>
    
    <div class="poll-info">
        <strong>Poll Status:</strong> %s<br>
        <strong>End Date:</strong> %s
    </div>
    
    <a href="/polls/%d/results" class="results-link">View Results</a>
    <a href="/" class="results-link">Return to Home</a>
</body>
</html>`, poll.Title, poll.Title, poll.Status, poll.EndDate.Format("2006-01-02 15:04:05"), poll.ID)

	w.Write([]byte(html))
}

// renderVoteSuccessPage renders the vote success page
func (h *VoteHandler) renderVoteSuccessPage(w http.ResponseWriter, poll *models.Poll) {
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Vote Submitted - %s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; text-align: center; }
        .success-message { background-color: #d4edda; color: #155724; padding: 20px; border-radius: 5px; margin: 20px 0; }
        .poll-info { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .results-link { background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; display: inline-block; margin: 10px; }
        .results-link:hover { background-color: #0056b3; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <div class="success-message">
        <h2>Vote Submitted Successfully!</h2>
        <p>Thank you for participating in this poll. Your vote has been recorded.</p>
    </div>
    
    <div class="poll-info">
        <strong>Poll Status:</strong> %s<br>
        <strong>End Date:</strong> %s
    </div>
    
    <a href="/polls/%d/results" class="results-link">View Results</a>
    <a href="/" class="results-link">Return to Home</a>
</body>
</html>`, poll.Title, poll.Title, poll.Status, poll.EndDate.Format("2006-01-02 15:04:05"), poll.ID)

	w.Write([]byte(html))
}
