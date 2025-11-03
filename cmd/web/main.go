// Package main is the entry point for the RSVP web application.
// It initializes configurations, database connections, template parsing,
// session management, routing, and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/templates"
	"github.com/tyemirov/RSVP/pkg/utils"
)

// main is the primary function that sets up and runs the web server.
func main() {
	applicationLogger := utils.NewLogger()
	environmentConfiguration := config.NewEnvConfig(applicationLogger)

	// Initialize session management using the secret key from environment configuration.
	session.NewSession([]byte(environmentConfiguration.SessionSecret))

	// Initialize the SQLite database connection and run auto-migrations for models.
	databaseConnection := services.InitDatabase(environmentConfiguration.Database.Name, applicationLogger)

	// Pre-parse all application template sets (layout, partials, views) exactly once at startup.
	templates.LoadAllPrecompiledTemplates(config.TemplatesDir)

	// Build the application context containing shared resources like database connection and logger.
	// Handlers will access this context. Template rendering retrieves from templates.PrecompiledTemplatesMap.
	applicationContext := &config.ApplicationContext{
		Database:   databaseConnection,
		Logger:     applicationLogger,
		AppBaseURL: environmentConfiguration.AppBaseURL, // Pass base URL to context
	}

	// Set up the HTTP request multiplexer (router).
	httpServeMuxRouter := http.NewServeMux()

	// Create the routes instance and register middleware (like authentication) and application routes.
	routesInstance := routes.New(applicationContext, *environmentConfiguration)
	routesInstance.RegisterMiddleware(httpServeMuxRouter) // Order matters: GAuss/Auth middleware first
	routesInstance.RegisterRoutes(httpServeMuxRouter)     // Then application routes

	// Configure the HTTP server details.
	serverAddress := fmt.Sprintf("%s:%d", config.ServerHTTPAddress, config.ServerHTTPPort)
	httpServerInstance := &http.Server{
		Addr:    serverAddress,
		Handler: httpServeMuxRouter, // Use the configured mux as the handler
	}

	// Start the server in a goroutine. Choose between HTTP and HTTPS based on certificate configuration.
	if environmentConfiguration.CertificateFilePath == "" || environmentConfiguration.KeyFilePath == "" {
		applicationLogger.Printf("Starting HTTP server on http://%s", serverAddress)
		go func() {
			listenAndServeError := httpServerInstance.ListenAndServe()
			// Log errors unless it's the expected server closed error during shutdown.
			if listenAndServeError != nil && !errors.Is(listenAndServeError, http.ErrServerClosed) {
				applicationLogger.Printf("HTTP ListenAndServe error: %v", listenAndServeError)
			}
		}()
	} else {
		applicationLogger.Printf("Starting HTTPS server on https://%s", serverAddress)
		go func() {
			listenAndServeTLSError := httpServerInstance.ListenAndServeTLS(
				environmentConfiguration.CertificateFilePath,
				environmentConfiguration.KeyFilePath,
			)
			// Log errors unless it's the expected server closed error during shutdown.
			if listenAndServeTLSError != nil && !errors.Is(listenAndServeTLSError, http.ErrServerClosed) {
				applicationLogger.Printf("HTTPS ListenAndServeTLS error: %v", listenAndServeTLSError)
			}
		}()
	}

	// Set up a channel to listen for OS signals (Interrupt, SIGTERM) for graceful shutdown.
	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, os.Interrupt, syscall.SIGTERM)

	// Block until a shutdown signal is received.
	<-shutdownSignalChannel
	applicationLogger.Println("Shutdown signal received; commencing graceful shutdown...")

	// Create a context with a timeout for the shutdown process.
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), config.ServerGracefulShutdownTimeout)
	defer cancelShutdown()

	// Attempt to gracefully shut down the server.
	shutdownError := httpServerInstance.Shutdown(shutdownContext)
	if shutdownError != nil {
		applicationLogger.Printf("Error during server shutdown: %v", shutdownError)
	} else {
		applicationLogger.Println("Server shutdown completed successfully.")
	}
}
