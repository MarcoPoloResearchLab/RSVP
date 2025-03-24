// Package main is the entry point for the RSVP application.
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

	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/routes"
	"github.com/temirov/RSVP/pkg/services"
	"github.com/temirov/RSVP/pkg/utils"
)

const (
	// HTTPPort is the default port for the web server.
	HTTPPort = 8080

	// HTTPIP is the default IP address for the web server.
	HTTPIP = "0.0.0.0"

	// TemplatesGlob is the pattern used to load all HTML templates.
	TemplatesGlob = "./templates/*.html"

	// ShutdownTimeout is the duration for graceful shutdown.
	ShutdownTimeout = 10 * time.Second
)

// main is the application entry point.
func main() {
	applicationLogger := utils.NewLogger()
	environmentConfiguration := config.NewEnvConfig(applicationLogger)

	session.NewSession([]byte(environmentConfiguration.SessionSecret))

	// Initialize the database and parse templates.
	databaseConnection := services.InitDatabase(environmentConfiguration.Database.Name, applicationLogger)
	parsedTemplates := template.Must(template.ParseGlob(TemplatesGlob))

	// Build the application context.
	applicationContext := &config.ApplicationContext{
		Database:  databaseConnection,
		Templates: parsedTemplates,
		Logger:    applicationLogger,
	}

	// Setup the HTTP router and register all routes and middleware.
	httpRouter := http.NewServeMux()
	route := routes.New(applicationContext, *environmentConfiguration)
	route.RegisterMiddleware(httpRouter)
	route.RegisterRoutes(httpRouter)

	// Start the HTTP/HTTPS server.
	serverAddress := fmt.Sprintf("%s:%d", HTTPIP, HTTPPort)
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
