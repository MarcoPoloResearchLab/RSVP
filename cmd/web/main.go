// Package main is the entry point for the RSVP web application.
// It initializes configurations, database connections, template parsing,
// session management, routing, and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
	"github.com/tyemirov/RSVP/pkg/server"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/templates"
	"github.com/tyemirov/RSVP/pkg/utils"
)

// main is the primary function that sets up and runs the web server.
func main() {
	applicationLogger := utils.NewLogger()
	environmentConfiguration, configurationError := config.Load("config.yml")
	if configurationError != nil {
		applicationLogger.Fatalf("Initialize backend: %v", configurationError)
	}

	// Open the canonical SQLite database or initialize its fresh schema.
	databaseConnection := services.InitDatabase(environmentConfiguration.Database.Name, applicationLogger)

	// Pre-parse all application template sets (layout, partials, views) exactly once at startup.
	templates.LoadAllPrecompiledTemplates(config.TemplatesDir)

	// Build the application context containing shared resources like database connection and logger.
	// Handlers will access this context. Template rendering retrieves from templates.PrecompiledTemplatesMap.
	applicationContext := &config.ApplicationContext{
		Database:   databaseConnection,
		Logger:     applicationLogger,
		AppBaseURL: environmentConfiguration.AppBaseURL, WebsiteURL: environmentConfiguration.WebsiteURL, CalendarReturnURL: environmentConfiguration.WebsiteURL + "#horizon/settings/integrations", // Pass base URL to context
	}
	attentionService, attentionServiceError := services.NewAttentionService(databaseConnection, time.Now)
	if attentionServiceError != nil {
		applicationLogger.Fatalf("Initialize attention clock: %v", attentionServiceError)
	}
	applicationRuntimeContext, cancelApplicationRuntime := context.WithCancel(context.Background())
	defer cancelApplicationRuntime()
	go runAttentionClock(applicationRuntimeContext, attentionService, applicationLogger)

	// Set up the HTTP request multiplexer (router).
	httpServeMuxRouter := http.NewServeMux()

	// Create the routes instance and register middleware (like authentication) and application routes.
	routesInstance, routesError := routes.New(applicationContext, *environmentConfiguration)
	if routesError != nil {
		applicationLogger.Fatalf("Initialize routes: %v", routesError)
	}
	routesInstance.RegisterMiddleware(httpServeMuxRouter)
	routesInstance.RegisterRoutes(httpServeMuxRouter)
	go func() {
		if taskError := routesInstance.RunCalendarConnectionTasks(applicationRuntimeContext); taskError != nil {
			applicationLogger.Printf("Calendar connection task worker stopped: %v", taskError)
		}
	}()
	go func() {
		if synchronizationError := routesInstance.RunCalendarSyncClock(applicationRuntimeContext, config.CalendarSyncInterval); synchronizationError != nil {
			applicationLogger.Printf("Calendar synchronization clock stopped: %v", synchronizationError)
		}
	}()

	// Configure the HTTP server details.
	serverAddress := environmentConfiguration.Server.Address
	httpServerInstance := server.NewHTTPServer(serverAddress, httpServeMuxRouter)

	applicationLogger.Printf("Starting HTTP server on %s", serverAddress)
	go func() {
		if err := httpServerInstance.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			applicationLogger.Fatalf("Serve HTTP: %v", err)
		}
	}()

	// Set up a channel to listen for OS signals (Interrupt, SIGTERM) for graceful shutdown.
	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, os.Interrupt, syscall.SIGTERM)

	// Block until a shutdown signal is received.
	<-shutdownSignalChannel
	cancelApplicationRuntime()
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

func runAttentionClock(ctx context.Context, service *services.AttentionService, applicationLogger interface{ Printf(string, ...any) }) {
	if _, transitionError := service.MarkAllMissedProbes(ctx); transitionError != nil && !errors.Is(transitionError, context.Canceled) {
		applicationLogger.Printf("Attention clock failed: %v", transitionError)
	}
	ticker := time.NewTicker(config.AttentionClockInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, transitionError := service.MarkAllMissedProbes(ctx); transitionError != nil && !errors.Is(transitionError, context.Canceled) {
				applicationLogger.Printf("Attention clock failed: %v", transitionError)
			}
		}
	}
}
