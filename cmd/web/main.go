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
	rsvpHandler "github.com/temirov/RSVP/pkg/handlers/rsvp"
	"github.com/temirov/RSVP/pkg/services"
	"github.com/temirov/RSVP/pkg/utils"
)

const (
	HttpPort        = 8080
	HttpIP          = "0.0.0.0"
	DatabaseName    = "rsvps.db"
	TemplatesGlob   = "./templates/*.html"
	ShutdownTimeout = 10 * time.Second
)

func main() {
	applicationLogger := utils.NewLogger()

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
	certificateFilePath := os.Getenv("TLS_CERT_PATH")
	keyFilePath := os.Getenv("TLS_KEY_PATH")

	session.NewSession([]byte(sessionSecret))
	authenticationService, authServiceErr := gauss.NewService(
		googleClientID,
		googleClientSecret,
		googleOauth2Base,
		config.WebRoot,
	)
	if authServiceErr != nil {
		applicationLogger.Fatal("Failed to initialize auth service:", authServiceErr)
	}

	databaseConnection := services.InitDatabase(DatabaseName, applicationLogger)
	parsedTemplates := template.Must(template.ParseGlob(TemplatesGlob))
	applicationContext := &config.App{
		Database:    databaseConnection,
		Templates:   parsedTemplates,
		Logger:      applicationLogger,
		AuthService: authenticationService,
	}

	httpRouter := http.NewServeMux()

	gaussHandlers, gaussHandlersErr := gauss.NewHandlers(authenticationService)
	if gaussHandlersErr != nil {
		applicationLogger.Fatal("Failed to initialize auth handlers:", gaussHandlersErr)
	}
	gaussHandlers.RegisterRoutes(httpRouter)

	httpRouter.Handle(config.WebRoot, gauss.AuthMiddleware(
		handlers.IndexHandler(applicationContext),
	))
	httpRouter.Handle(config.WebRSVPs,
		gauss.AuthMiddleware(rsvpHandler.ListCreateHandler(applicationContext)),
	)
	httpRouter.HandleFunc(config.WebUnderRSVPs, rsvpHandler.Subrouter(applicationContext))

	serverAddress := fmt.Sprintf("%s:%d", HttpIP, HttpPort)
	httpServer := &http.Server{
		Addr:    serverAddress,
		Handler: httpRouter,
	}

	if certificateFilePath == "" || keyFilePath == "" {
		applicationLogger.Printf("Starting HTTP server on http://%s", serverAddress)
		go func() {
			serverError := httpServer.ListenAndServe()
			if serverError != nil && !errors.Is(serverError, http.ErrServerClosed) {
				applicationLogger.Printf("ListenAndServe error: %v", serverError)
			}
		}()
	} else {
		applicationLogger.Printf("Starting HTTPS server on https://%s", serverAddress)
		go func() {
			secureServerError := httpServer.ListenAndServeTLS(certificateFilePath, keyFilePath)
			if secureServerError != nil && !errors.Is(secureServerError, http.ErrServerClosed) {
				applicationLogger.Printf("ListenAndServeTLS error: %v", secureServerError)
			}
		}()
	}

	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignalChannel
	applicationLogger.Println("Shutdown signal received...")

	shutdownContext, shutdownCancelFunction := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer shutdownCancelFunction()

	shutdownError := httpServer.Shutdown(shutdownContext)
	if shutdownError != nil {
		applicationLogger.Printf("Server Shutdown error: %v", shutdownError)
	} else {
		applicationLogger.Println("Server shutdown completed successfully.")
	}
}
