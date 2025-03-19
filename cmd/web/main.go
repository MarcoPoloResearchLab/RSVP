package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/temirov/RSVP/pkg/routes"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/services"
	"github.com/temirov/RSVP/pkg/utils"
)

const (
	HttpPort        = 8080
	HttpIP          = "0.0.0.0"
	TemplatesGlob   = "./templates/*.html"
	ShutdownTimeout = 10 * time.Second
)

func main() {
	applicationLogger := utils.NewLogger()
	environmentConfiguration := config.NewEnvConfig(applicationLogger)

	session.NewSession([]byte(environmentConfiguration.SessionSecret))

	authenticationService, authenticationServiceError := gauss.NewService(
		environmentConfiguration.GoogleClientID,
		environmentConfiguration.GoogleClientSecret,
		environmentConfiguration.GoogleOauth2Base,
		config.WebEvents,
	)
	if authenticationServiceError != nil {
		applicationLogger.Fatal("Failed to initialize auth service:", authenticationServiceError)
	}

	databaseConnection := services.InitDatabase(environmentConfiguration.Database.Name, applicationLogger)
	parsedTemplates := template.Must(template.ParseGlob(TemplatesGlob))

	applicationContext := &config.ApplicationContext{
		Database:    databaseConnection,
		Templates:   parsedTemplates,
		Logger:      applicationLogger,
		AuthService: authenticationService,
	}

	httpRouter := http.NewServeMux()

	route := routes.New(applicationContext, *environmentConfiguration)
	route.RegisterMiddleware(httpRouter)
	route.RegisterRoutes(httpRouter)

	serverAddress := fmt.Sprintf("%s:%d", HttpIP, HttpPort)
	httpServer := &http.Server{
		Addr:    serverAddress,
		Handler: httpRouter,
	}

	if environmentConfiguration.CertificateFilePath == "" || environmentConfiguration.KeyFilePath == "" {
		applicationLogger.Printf("Starting HTTP server on http://%s", serverAddress)
		go func() {
			listenAndServeError := httpServer.ListenAndServe()
			if listenAndServeError != nil && !errors.Is(listenAndServeError, http.ErrServerClosed) {
				applicationLogger.Printf("ListenAndServe error: %v", listenAndServeError)
			}
		}()
	} else {
		applicationLogger.Printf("Starting HTTPS server on https://%s", serverAddress)
		go func() {
			listenAndServeTLSError := httpServer.ListenAndServeTLS(
				environmentConfiguration.CertificateFilePath,
				environmentConfiguration.KeyFilePath,
			)
			if listenAndServeTLSError != nil && !errors.Is(listenAndServeTLSError, http.ErrServerClosed) {
				applicationLogger.Printf("ListenAndServeTLS error: %v", listenAndServeTLSError)
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
