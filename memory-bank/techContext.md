# Technical Context

## 1. Technology Stack

### 1.1 Backend
- **Language**: Go (Golang)
- **Web Framework**: Standard library `net/http`
- **Database**: SQLite with GORM ORM
- **Authentication**: Google OAuth via custom implementation
- **Template Engine**: Go's built-in `html/template`
- **QR Code Generation**: `github.com/skip2/go-qrcode`

### 1.2 Frontend
- **HTML/CSS Framework**: Bootstrap 4
- **JavaScript**: Minimal vanilla JS for interactive elements
- **Fonts**: Google Fonts (Montserrat)

### 1.3 Development Tools
- **Version Control**: Git
- **Containerization**: Docker
- **Deployment**: Shell scripts

## 2. Development Setup

### 2.1 Prerequisites
- Go 1.16+ installed
- Git for version control
- Docker (optional, for containerized deployment)

### 2.2 Environment Variables
The application requires the following environment variables:
- `GOOGLE_CLIENT_ID`: OAuth client ID
- `GOOGLE_CLIENT_SECRET`: OAuth client secret
- `SESSION_SECRET`: Secret for session encryption
- `REDIRECT_URL`: OAuth redirect URL

### 2.3 Local Development
1. Clone the repository
2. Set required environment variables
3. Run `go run cmd/web/main.go`
4. Access the application at `http://localhost:8080`

### 2.4 Building for Production
1. Run `go build -o rsvp-app cmd/web/main.go`
2. Set environment variables in production environment
3. Run the compiled binary

### 2.5 Docker Deployment
1. Build the Docker image: `docker build -t rsvp-app .`
2. Run the container: `docker run -p 8080:8080 --env-file .env rsvp-app`

## 3. Project Structure

```
RSVP/
├── cmd/
│   └── web/
│       └── main.go           # Application entry point
├── models/
│   ├── event.go              # Event model
│   ├── rsvp.go               # RSVP model
│   └── user.go               # User model
├── pkg/
│   ├── config/               # Configuration structures
│   ├── routes/               # HTTP route handlers
│   ├── services/             # Service implementations
│   └── utils/                # Utility functions
├── templates/
│   ├── event/                # Event-related templates
│   ├── response/             # Response-related templates
│   └── rsvp/                 # RSVP-related templates
├── .gitignore
├── Dockerfile                # Docker configuration
├── go.mod                    # Go module definition
├── go.sum                    # Go module checksums
└── README.md
```

## 4. Database Schema

### 4.0 Database Management
The application currently uses a development-oriented approach to database management:
- The database file is named `rsvps.db` and stored in the application root
- On every application start, the `InitDatabase` function in `pkg/services/database.go`:
  - Drops all existing tables
  - Recreates them with the updated schema
  - This means all data is lost on restart

**Important Note**: This approach is suitable for early development but should be changed for production:
- Database resets should be performed manually as needed, not automatically
- Schema changes should use migrations rather than dropping tables
- Data persistence should be maintained between application restarts

The application uses GORM to manage the database schema, which consists of the following tables:

### 4.1 Users Table
- `id`: Primary key
- `created_at`: Creation timestamp
- `updated_at`: Last update timestamp
- `deleted_at`: Soft delete timestamp (nullable)
- `email`: User's email (unique)
- `name`: User's name
- `picture`: URL to user's profile picture

### 4.2 Events Table
- `id`: Primary key
- `created_at`: Creation timestamp
- `updated_at`: Last update timestamp
- `deleted_at`: Soft delete timestamp (nullable)
- `title`: Event title
- `description`: Event description
- `start_time`: Event start time
- `end_time`: Event end time
- `user_id`: Foreign key to users table

### 4.3 RSVPs Table
- `id`: Primary key
- `created_at`: Creation timestamp
- `updated_at`: Last update timestamp
- `deleted_at`: Soft delete timestamp (nullable)
- `name`: Invitee name
- `code`: Unique RSVP code
- `response`: RSVP response (Yes/No)
- `extra_guests`: Number of additional guests
- `event_id`: Foreign key to events table

## 5. External Dependencies

### 5.1 Third-Party Go Packages
- `gorm.io/gorm`: ORM for database operations
- `gorm.io/driver/sqlite`: SQLite driver for GORM
- `github.com/skip2/go-qrcode`: QR code generation
- `github.com/temirov/GAuss`: Custom authentication package

### 5.2 Frontend Dependencies
- Bootstrap 4 (CDN)
- jQuery (CDN, minimal usage)
- Google Fonts (Montserrat)

## 6. Technical Constraints

### 6.1 Performance Considerations
- SQLite is suitable for moderate load but may become a bottleneck with high concurrent usage
- QR code generation is performed on-demand and may impact performance with large numbers of RSVPs

### 6.2 Security Considerations
- Session-based authentication
- HTTPS support for production deployment
- No sensitive data stored (no passwords, payment info, etc.)

### 6.3 Scalability Considerations
- Current architecture is suitable for single-server deployment
- Database may need migration to a more robust solution for high-scale usage

---

**Last Updated:** 03/16/2025
