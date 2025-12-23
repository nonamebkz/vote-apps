package main

import (
	"net/http"
	"os"
	"strings"

	"polling-system/internal/config"
	"polling-system/internal/database"
	"polling-system/internal/handlers"
	"polling-system/internal/logger"
	"polling-system/internal/middleware"
	"polling-system/internal/repositories"
	"polling-system/internal/services"
	"polling-system/pkg/export"
	"polling-system/pkg/qr"
	"polling-system/pkg/websocket"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to initialize database", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Database initialized successfully")

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	pollRepo := repositories.NewPollRepository(db)
	voteRepo := repositories.NewVoteRepository(db)

	// Initialize services
	qrService := qr.NewService(cfg.BaseURL, "./storage/qr")
	exportPkgService := export.NewService("./storage/exports")
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

	// Setup rate limiter (60 requests per minute, burst of 10)
	rateLimiter := middleware.NewRateLimiter(60, 10)

	// Setup middleware chain
	router.Use(middleware.ErrorHandling)          // Panic recovery and error handling
	router.Use(middleware.RequestID)              // Add request IDs
	router.Use(middleware.SecurityHeaders)        // Security headers
	router.Use(middleware.MaxBodySize(10 << 20))  // Limit request body to 10MB
	router.Use(middleware.NoScriptInjection)      // Prevent script injection
	router.Use(middleware.SQLInjectionPrevention) // Prevent SQL injection
	router.Use(middleware.ValidationMiddleware)   // Input validation and sanitization
	router.Use(middleware.CORS)                   // CORS handling
	router.Use(middleware.Logging)                // Structured logging
	router.Use(middleware.RateLimit(rateLimiter)) // Rate limiting

	// Setup routes
	setupRoutes(router, authHandler, pollHandler, voteHandler, qrHandler, adminHandler, wsHandler)

	// Setup static file serving for frontend
	setupStaticFiles(router)

	// Setup custom error handlers
	router.NotFoundHandler = middleware.NotFound()
	router.MethodNotAllowedHandler = middleware.MethodNotAllowed()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Server starting", map[string]interface{}{
		"port": port,
	})

	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.Fatal("Server failed to start", map[string]interface{}{
			"error": err.Error(),
			"port":  port,
		})
	}
}

func setupRoutes(
	router *mux.Router,
	authHandler *handlers.AuthHandler,
	pollHandler *handlers.PollHandler,
	voteHandler *handlers.VoteHandler,
	qrHandler *handlers.QRHandler,
	adminHandler *handlers.AdminHandler,
	wsHandler *handlers.WebSocketHandler,
) {
	// Auth routes
	auth := router.PathPrefix("/api/auth").Subrouter()
	auth.Use(middleware.ContentTypeValidation("application/json")) // Require JSON for auth endpoints
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	auth.HandleFunc("/refresh", authHandler.RefreshToken).Methods("POST")

	// Public routes (QR code access)
	router.HandleFunc("/vote/{poll_id}", voteHandler.VotePage).Methods("GET")
	router.HandleFunc("/login", voteHandler.HandleLoginRedirect).Methods("GET")

	// QR code routes
	router.HandleFunc("/api/qr/{poll_id}", qrHandler.GetQRCode).Methods("GET")
	router.HandleFunc("/api/qr/{poll_id}/file", qrHandler.GetQRCodeFile).Methods("GET")
	router.HandleFunc("/api/qr/{poll_id}/url", qrHandler.GetVotingURL).Methods("GET")

	// Protected routes
	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.JWTAuth)

	// QR code management (protected)
	api.HandleFunc("/qr/{poll_id}/regenerate", qrHandler.RegenerateQRCode).Methods("POST")

	// Poll routes
	polls := api.PathPrefix("/polls").Subrouter()
	polls.HandleFunc("", pollHandler.ListPolls).Methods("GET")
	polls.HandleFunc("", pollHandler.CreatePoll).Methods("POST").Headers("Content-Type", "application/json")
	polls.HandleFunc("/{id}", pollHandler.GetPoll).Methods("GET")
	polls.HandleFunc("/{id}", pollHandler.UpdatePoll).Methods("PUT").Headers("Content-Type", "application/json")
	polls.HandleFunc("/{id}", pollHandler.DeletePoll).Methods("DELETE")
	polls.HandleFunc("/{id}/start", pollHandler.StartPoll).Methods("POST")
	polls.HandleFunc("/{id}/pause", pollHandler.PausePoll).Methods("POST")
	polls.HandleFunc("/{id}/stop", pollHandler.StopPoll).Methods("POST")
	polls.HandleFunc("/{id}/vote", voteHandler.SubmitVote).Methods("POST").Headers("Content-Type", "application/json")
	polls.HandleFunc("/{id}/vote", voteHandler.GetVotePage).Methods("GET")

	// User routes
	users := api.PathPrefix("/users").Subrouter()
	users.HandleFunc("/{id}/votes", voteHandler.GetUserVotes).Methods("GET")

	// Admin routes
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.AdminOnly)
	admin.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	admin.HandleFunc("/users", adminHandler.CreateUser).Methods("POST").Headers("Content-Type", "application/json")
	admin.HandleFunc("/users/{id}/status", adminHandler.UpdateUserStatus).Methods("PUT").Headers("Content-Type", "application/json")
	admin.HandleFunc("/polls/{id}/participation", adminHandler.GetParticipation).Methods("GET")

	// Export routes
	admin.HandleFunc("/polls/{id}/export", adminHandler.ExportResults).Methods("GET")
	admin.HandleFunc("/polls/{id}/export/direct", adminHandler.ExportResultsDirect).Methods("GET")
	admin.HandleFunc("/polls/{id}/export/download/{filename}", adminHandler.DownloadExport).Methods("GET")
	admin.HandleFunc("/export/formats", adminHandler.GetExportFormats).Methods("GET")
	admin.HandleFunc("/export/cleanup", adminHandler.CleanupExports).Methods("POST")

	// WebSocket route
	router.HandleFunc("/ws/{poll_id}", wsHandler.HandleConnection)
}

// setupStaticFiles configures static file serving for the frontend
func setupStaticFiles(router *mux.Router) {
	// Serve static files from web/frontend/build directory
	staticDir := "./web/frontend/build"

	// Create a file server for static assets
	fs := http.FileServer(http.Dir(staticDir))

	// Handle static assets (CSS, JS, images, etc.)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// Handle favicon and other root-level assets
	router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/favicon.ico")
	})

	router.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/manifest.json")
	})

	// Serve the React app for all other routes (SPA routing)
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve index.html for API routes or WebSocket connections
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/ws/") ||
			strings.HasPrefix(r.URL.Path, "/vote/") {
			http.NotFound(w, r)
			return
		}

		// Serve index.html for all other routes (React Router will handle routing)
		http.ServeFile(w, r, staticDir+"/index.html")
	})
}
