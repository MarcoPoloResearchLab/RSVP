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

	"github.com/temirov/RSVP/pkg/routes"

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
	envConfig := config.NewEnvConfig(applicationLogger)

	session.NewSession([]byte(envConfig.SessionSecret))
	authenticationService, authServiceErr := gauss.NewService(
		envConfig.GoogleClientID,
		envConfig.GoogleClientSecret,
		envConfig.GoogleOauth2Base,
		config.WebEvents,
	)
	if authServiceErr != nil {
		applicationLogger.Fatal("Failed to initialize auth service:", authServiceErr)
	}

	databaseConnection := services.InitDatabase(envConfig.Database.Name, applicationLogger)
	parsedTemplates := template.Must(template.ParseGlob(TemplatesGlob))

	applicationContext := &config.ApplicationContext{
		Database:    databaseConnection,
		Templates:   parsedTemplates,
		Logger:      applicationLogger,
		AuthService: authenticationService,
	}

	httpRouter := http.NewServeMux()

	route := routes.New(applicationContext, *envConfig)
	route.RegisterMiddleware(httpRouter)
	route.RegisterRoutes(httpRouter)

	serverAddress := fmt.Sprintf("%s:%d", HttpIP, HttpPort)
	httpServer := &http.Server{
		Addr:    serverAddress,
		Handler: httpRouter,
	}

	if envConfig.CertificateFilePath == "" || envConfig.KeyFilePath == "" {
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
			secureServerError := httpServer.ListenAndServeTLS(envConfig.CertificateFilePath, envConfig.KeyFilePath)
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
