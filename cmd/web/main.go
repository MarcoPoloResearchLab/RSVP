package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/routes"
	"github.com/temirov/RSVP/pkg/services"
	"github.com/temirov/RSVP/pkg/templates"
	"github.com/temirov/RSVP/pkg/utils"
)

const (
	serverHttpPort                      = 8080
	serverHttpIpAddress                 = "0.0.0.0"
	serverGracefulShutdownTimeoutPeriod = 10 * time.Second
)

func main() {
	applicationLogger := utils.NewLogger()
	environmentConfiguration := config.NewEnvConfig(applicationLogger)

	// Initialize session using the secret key.
	session.NewSession([]byte(environmentConfiguration.SessionSecret))

	// Initialize the SQLite database and run auto-migrations.
	databaseConnection := services.InitDatabase(environmentConfiguration.Database.Name, applicationLogger)

	// Pre-parse all template sets exactly once at startup.
	templates.LoadAllPrecompiledTemplates("templates")

	// Build the application context; handlers will reference the precompiled templates via the templates package.
	applicationContext := &config.ApplicationContext{
		Database: databaseConnection,
		Logger:   applicationLogger,
		// Note: Template rendering in handlers now retrieves from templates.PrecompiledTemplatesMap.
	}

	// Set up the HTTP router and register middleware and routes.
	httpServeMuxRouter := http.NewServeMux()
	routesInstance := routes.New(applicationContext, *environmentConfiguration)
	routesInstance.RegisterMiddleware(httpServeMuxRouter)
	routesInstance.RegisterRoutes(httpServeMuxRouter)

	// Configure and start the HTTP server.
	serverAddress := fmt.Sprintf("%s:%d", serverHttpIpAddress, serverHttpPort)
	httpServerInstance := &http.Server{
		Addr:    serverAddress,
		Handler: httpServeMuxRouter,
	}

	if environmentConfiguration.CertificateFilePath == "" || environmentConfiguration.KeyFilePath == "" {
		applicationLogger.Printf("Starting HTTP server on http://%s", serverAddress)
		go func() {
			listenAndServeError := httpServerInstance.ListenAndServe()
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
			if listenAndServeTLSError != nil && !errors.Is(listenAndServeTLSError, http.ErrServerClosed) {
				applicationLogger.Printf("HTTPS ListenAndServeTLS error: %v", listenAndServeTLSError)
			}
		}()
	}

	// Wait for termination signal for graceful shutdown.
	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignalChannel
	applicationLogger.Println("Shutdown signal received; commencing graceful shutdown...")

	shutdownContext, shutdownCancelFunction := context.WithTimeout(context.Background(), serverGracefulShutdownTimeoutPeriod)
	defer shutdownCancelFunction()

	shutdownError := httpServerInstance.Shutdown(shutdownContext)
	if shutdownError != nil {
		applicationLogger.Printf("Error during server shutdown: %v", shutdownError)
	} else {
		applicationLogger.Println("Server shutdown completed successfully.")
	}
}
