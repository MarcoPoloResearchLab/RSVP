package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/services"
	"github.com/temirov/RSVP/pkg/utils"
)

const (
	HTTPPort      = 8080
	HTTPIP        = "0.0.0.0"
	DBName        = "rsvps.db"
	TemplatesPath = "./templates/*.html"
)

func main() {
	// Initialize shared logger.
	applicationLogger := utils.NewLogger()

	// Load required environment variables.
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		applicationLogger.Fatal("SESSION_SECRET is not set")
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		applicationLogger.Fatal("GOOGLE_CLIENT_ID is not set")
	}
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		applicationLogger.Fatal("GOOGLE_CLIENT_SECRET is not set")
	}
	googleOauth2Base := os.Getenv("GOOGLE_OAUTH2_BASE")
	if googleOauth2Base == "" {
		applicationLogger.Fatal("GOOGLE_OAUTH2_BASE is not set")
	}
	certFilePath := os.Getenv("TLS_CERT_PATH")
	keyFilePath := os.Getenv("TLS_KEY_PATH")

	// Initialize session and auth service.
	session.NewSession([]byte(sessionSecret))
	authenticationService, errorValue := gauss.NewService(googleClientID, googleClientSecret, googleOauth2Base, config.WebRoot)
	if errorValue != nil {
		applicationLogger.Fatal("Failed to initialize auth service:", errorValue)
	}

	// Initialize the database.
	databaseConnection := services.InitDatabase(DBName, applicationLogger)

	// Parse HTML templates.
	parsedTemplates := template.Must(template.ParseGlob(TemplatesPath))

	// Create the shared application context.
	applicationContext := &config.App{
		Database:    databaseConnection,
		Templates:   parsedTemplates,
		Logger:      applicationLogger,
		AuthService: authenticationService,
	}

	// Set up HTTP router.
	httpRouter := http.NewServeMux()
	authHandlers, errorValue := gauss.NewHandlers(authenticationService)
	if errorValue != nil {
		applicationLogger.Fatal("Failed to initialize auth handlers:", errorValue)
	}
	authHandlers.RegisterRoutes(httpRouter)

	// Register protected endpoints using your auth middleware.
	httpRouter.Handle(config.WebRoot, gauss.AuthMiddleware(handlers.IndexHandler(applicationContext)))
	httpRouter.Handle(config.WebGenerate, gauss.AuthMiddleware(handlers.GenerateHandler(applicationContext)))
	httpRouter.Handle(config.WebResponses, gauss.AuthMiddleware(handlers.ResponsesHandler(applicationContext)))

	// Register unprotected endpoints.
	httpRouter.HandleFunc(config.WebRSVP, handlers.RsvpHandler(applicationContext))
	httpRouter.HandleFunc(config.WebSubmit, handlers.SubmitHandler(applicationContext))
	httpRouter.HandleFunc(config.WebThankYou, handlers.ThankYouHandler(applicationContext))

	// Determine server address.
	serverAddress := fmt.Sprintf("%s:%d", HTTPIP, HTTPPort)
	var httpServer *http.Server

	if certFilePath == "" || keyFilePath == "" {
		applicationLogger.Printf("No SSL certificates found, starting HTTP server on http://%s", serverAddress)
		httpServer = &http.Server{
			Addr:    serverAddress,
			Handler: httpRouter,
		}
		go func() {
			if errorValue := httpServer.ListenAndServe(); errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
				applicationLogger.Printf("HTTP server ListenAndServe error: %v", errorValue)
			}
		}()
	} else {
		applicationLogger.Printf("Starting HTTPS server on https://%s", serverAddress)
		httpServer = &http.Server{
			Addr:    serverAddress,
			Handler: httpRouter,
		}
		go func() {
			if errorValue := httpServer.ListenAndServeTLS(certFilePath, keyFilePath); errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
				applicationLogger.Printf("HTTPS server ListenAndServeTLS error: %v", errorValue)
			}
		}()
	}

	// Graceful shutdown handling.
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignal
	applicationLogger.Println("Shutdown signal received, shutting down gracefully...")
	shutdownContext, cancelFunction := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunction()
	if errorValue := httpServer.Shutdown(shutdownContext); errorValue != nil {
		applicationLogger.Printf("Server Shutdown error: %v", errorValue)
	} else {
		applicationLogger.Println("Server shutdown completed")
	}
}
