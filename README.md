# Polling System

A real-time polling system built with Go and React.js that supports admin management, QR code voting, and WebSocket-based real-time updates.

## Features

- **Admin Management**: Create, edit, and manage polls
- **Real-time Voting**: Vote through QR codes with live result updates
- **User Management**: JWT-based authentication with role-based access
- **Export Functionality**: Export results to PDF and Excel formats
- **WebSocket Integration**: Real-time updates for voting results
- **Anti-Double Vote**: Prevent duplicate voting per user per poll

## Architecture

The application follows clean architecture principles with the following layers:

- **Handlers**: HTTP request handlers and WebSocket management
- **Services**: Business logic layer
- **Repositories**: Data access layer
- **Models**: Data models and structures
- **Middleware**: Authentication, CORS, and logging

## Project Structure

```
polling-system/
├── cmd/server/           # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── database/        # Database connection and migrations
│   ├── handlers/        # HTTP handlers
│   ├── middleware/      # HTTP middleware
│   ├── models/          # Data models
│   ├── repositories/    # Data access layer
│   └── services/        # Business logic layer
├── pkg/
│   ├── auth/           # JWT authentication utilities
│   ├── export/         # PDF/Excel export utilities
│   ├── qr/             # QR code generation
│   └── websocket/      # WebSocket hub and client management
└── web/
    ├── static/         # Static files
    └── templates/      # HTML templates
```

## Dependencies

- **GORM**: ORM for database operations
- **Gorilla Mux**: HTTP router
- **Gorilla WebSocket**: WebSocket implementation
- **JWT**: JSON Web Token authentication
- **bcrypt**: Password hashing
- **QR Code**: QR code generation
- **PostgreSQL**: Database driver

## Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd polling-system
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Setup environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Setup PostgreSQL database**
   ```bash
   # Create database
   createdb polling_db
   ```

5. **Run the application**
   ```bash
   go run cmd/server/main.go
   ```

## Environment Variables

See `.env.example` for all available configuration options:

- `DATABASE_URL`: PostgreSQL connection string
- `JWT_SECRET`: Secret key for JWT token signing
- `PORT`: Server port (default: 8080)
- `ENVIRONMENT`: Application environment (development/production)

## API Endpoints

### Authentication
- `POST /api/auth/register` - User registration
- `POST /api/auth/login` - User login
- `POST /api/auth/refresh` - Token refresh

### Polls
- `GET /api/polls` - List polls
- `POST /api/polls` - Create poll
- `GET /api/polls/{id}` - Get poll details
- `PUT /api/polls/{id}` - Update poll
- `DELETE /api/polls/{id}` - Delete poll
- `POST /api/polls/{id}/start` - Start poll
- `POST /api/polls/{id}/pause` - Pause poll
- `POST /api/polls/{id}/stop` - Stop poll

### Voting
- `GET /vote/{poll_id}` - Public voting page (via QR code)
- `POST /api/polls/{id}/vote` - Submit vote
- `GET /api/users/{id}/votes` - Get user vote history

### Admin
- `GET /api/admin/users` - List users
- `POST /api/admin/users` - Create user
- `PUT /api/admin/users/{id}/status` - Update user status
- `GET /api/admin/polls/{id}/participation` - Get participation stats
- `GET /api/admin/polls/{id}/export` - Export results

### WebSocket
- `GET /ws/{poll_id}` - WebSocket connection for real-time updates

## Development

This project is structured to be implemented incrementally following the tasks defined in `.kiro/specs/polling-system/tasks.md`.

Each task builds upon the previous ones, ensuring a systematic development approach from basic setup to full functionality.

## Testing

The project includes both unit tests and property-based tests:

- Unit tests for specific functionality
- Property-based tests for universal correctness properties
- Integration tests for end-to-end workflows

Run tests with:
```bash
go test ./...
```

## License

[Add your license information here]