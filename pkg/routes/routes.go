// Package routes holds application-wide shared resources and environment configuration.
package routes

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/attentionpolicy"
	"github.com/tyemirov/RSVP/pkg/handlers/calendar"
	"github.com/tyemirov/RSVP/pkg/handlers/calendarconnection"
	"github.com/tyemirov/RSVP/pkg/handlers/derivedmarker"
	"github.com/tyemirov/RSVP/pkg/handlers/event"
	"github.com/tyemirov/RSVP/pkg/handlers/horizon"
	"github.com/tyemirov/RSVP/pkg/handlers/ingestiondraft"
	"github.com/tyemirov/RSVP/pkg/handlers/lane"
	"github.com/tyemirov/RSVP/pkg/handlers/organizer"
	"github.com/tyemirov/RSVP/pkg/handlers/probe"
	"github.com/tyemirov/RSVP/pkg/handlers/response"
	"github.com/tyemirov/RSVP/pkg/handlers/rsvp"
	"github.com/tyemirov/RSVP/pkg/handlers/venue"
	"github.com/tyemirov/RSVP/pkg/middleware"
	naturallanguageprovider "github.com/tyemirov/RSVP/pkg/providers/naturallanguage"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/utils"
	staticassets "github.com/tyemirov/RSVP/static"
)

// Routes holds shared resources and environment configuration.
type Routes struct {
	ApplicationContext       *config.ApplicationContext
	EnvConfig                *config.EnvConfig
	calendarConnectionRoutes *calendarconnection.Resources
	authorizer               func(http.Handler) http.Handler
	parser                   services.NaturalLanguageParser
	derivedMarkers           *derivedmarker.Resources
	ingestionDrafts          *ingestiondraft.Resources
}

// New creates and returns a new Routes instance.
func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) (*Routes, error) {
	validator, err := sessionvalidator.New(sessionvalidator.Config{SigningKey: []byte(envConfig.Auth.SigningKey), Issuer: envConfig.Auth.Issuer, CookieName: envConfig.Auth.CookieName})
	if err != nil {
		return nil, fmt.Errorf("construct RSVP resource authorization: %w", err)
	}
	parser, err := naturallanguageprovider.New(envConfig.LLMProxy, http.DefaultClient)
	if err != nil {
		return nil, err
	}
	calendarResources, err := calendarconnection.New(applicationContext, envConfig, time.Now)
	if err != nil {
		return nil, fmt.Errorf("construct calendar connections: %w", err)
	}
	markerResources, err := derivedmarker.New(applicationContext)
	if err != nil {
		return nil, fmt.Errorf("construct derived markers: %w", err)
	}
	draftResources, err := ingestiondraft.New(applicationContext, time.Now, parser)
	if err != nil {
		return nil, fmt.Errorf("construct ingestion drafts: %w", err)
	}
	return &Routes{ApplicationContext: applicationContext, EnvConfig: &envConfig, authorizer: middleware.ResourceAuthorization(applicationContext, validator, envConfig.Auth.TenantID), parser: parser, calendarConnectionRoutes: calendarResources, derivedMarkers: markerResources, ingestionDrafts: draftResources}, nil
}

// LandingPageHandler serves the landing page.
func (appRoutes *Routes) LandingPageHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != config.WebRoot {
		http.NotFound(responseWriter, request)
		return
	}
	staticassets.Handler().ServeHTTP(responseWriter, request)
}

// ApplyOverrides applies HTTP method override.
func (appRoutes *Routes) ApplyOverrides(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			if request.Form == nil {
				request.Body = http.MaxBytesReader(responseWriter, request.Body, 10<<20)
				if parseError := request.ParseForm(); parseError != nil {
					appRoutes.ApplicationContext.Logger.Printf("WARN: Parsing form data for %s failed: %v", request.URL.Path, parseError)
				}
			}
			methodOverride := request.FormValue(config.MethodOverrideParam)
			if methodOverride != "" {
				switch methodOverride {
				case http.MethodPut, http.MethodPatch, http.MethodDelete:
					originalMethod := request.Method
					request.Method = methodOverride
					appRoutes.ApplicationContext.Logger.Printf("DEBUG: Method overridden from %s to %s for %s", originalMethod, request.Method, request.URL.Path)
				default:
					appRoutes.ApplicationContext.Logger.Printf("WARN: Invalid _method override '%s' for %s", methodOverride, request.URL.Path)
				}
			}
		}
		next.ServeHTTP(responseWriter, request)
	})
}

// publicChainWithOverride wraps a handler with HTTP override middleware.
func (appRoutes *Routes) publicChainWithOverride(handler http.Handler) http.Handler {
	return appRoutes.ApplyOverrides(handler)
}

// RegisterMiddleware exposes TAuth through the configured local front door.
func (appRoutes *Routes) RegisterMiddleware(mux *http.ServeMux) {
	target, _ := url.Parse(appRoutes.EnvConfig.Auth.UpstreamURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(writer, "Authentication service is unavailable.", http.StatusBadGateway)
	}
	mux.Handle("/auth/", proxy)
}

// UIConfig returns only the public browser configuration.
func (appRoutes *Routes) UIConfig() ([]byte, error) {
	auth := appRoutes.EnvConfig.Auth
	browserAuth := map[string]any{
		"tauthUrl": auth.URL, "tenantId": auth.TenantID, "logoutPath": auth.LogoutPath, "sessionPath": auth.SessionPath,
		"providers": map[string]any{"google": map[string]any{"enabled": true, "clientId": appRoutes.EnvConfig.GoogleClientID, "loginPath": auth.LoginPath, "noncePath": auth.NoncePath}, "apple": map[string]bool{"enabled": false}, "password": map[string]bool{"enabled": false}},
	}
	return yaml.Marshal(map[string]any{"environments": []any{map[string]any{"description": "RSVP", "origins": []string{strings.TrimSuffix(appRoutes.EnvConfig.AppBaseURL, "/")}, "auth": browserAuth}}})
}

// RegisterRoutes registers all application routes.
func (appRoutes *Routes) RegisterRoutes(mux *http.ServeMux) {
	protectedChain := func(handler http.Handler) http.Handler {
		return appRoutes.authorizer(appRoutes.ApplyOverrides(handler))
	}
	strictProtectedChain := appRoutes.authorizer
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if request.Method == http.MethodGet {
			_, _ = writer.Write([]byte("OK"))
		}
	})
	mux.HandleFunc("/config-ui.yaml", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		payload, err := appRoutes.UIConfig()
		if err != nil {
			http.Error(writer, "Browser configuration is unavailable", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/yaml")
		writer.Header().Set("Cache-Control", "no-store")
		if request.Method == http.MethodGet {
			_, _ = writer.Write(payload)
		}
	})
	mux.HandleFunc(config.WebRoot, appRoutes.LandingPageHandler)
	mux.Handle(config.WebStatic, http.StripPrefix(config.WebStatic, staticassets.Handler()))
	responseBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		appRoutes.ApplicationContext.Logger.Printf("Router: Public path %s, method %s", request.URL.Path, request.Method)
		response.Handler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
	})
	mux.Handle(config.WebResponse, appRoutes.publicChainWithOverride(responseBaseDispatcher))
	mux.HandleFunc(config.WebResponseThankYou, response.ThankYouHandler(appRoutes.ApplicationContext))
	eventBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		appRoutes.ApplicationContext.Logger.Printf("Router: Protected path %s, method %s", request.URL.Path, request.Method)
		switch request.Method {
		case http.MethodGet:
			event.ListEventsHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			event.CreateHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut, http.MethodPatch:
			event.UpdateEventHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			event.DeleteHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, appRoutes.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		}
	})
	mux.Handle(config.WebEvents, protectedChain(eventBaseDispatcher))
	mux.Handle(config.WebCalendars, strictProtectedChain(calendar.Handler(appRoutes.ApplicationContext)))
	mux.Handle(config.WebDerivedMarkerRules, strictProtectedChain(appRoutes.derivedMarkers.Handler()))
	mux.Handle(config.WebIngestionDrafts, strictProtectedChain(appRoutes.ingestionDrafts.Handler()))
	mux.Handle(config.WebCalendarAuthorizationRequests, strictProtectedChain(appRoutes.calendarConnectionRoutes.AuthorizationRequests()))
	mux.Handle(config.WebCalendarConnectionCallbacksGoogle, strictProtectedChain(appRoutes.calendarConnectionRoutes.Callback()))
	mux.Handle(config.WebCalendarConnections, strictProtectedChain(appRoutes.calendarConnectionRoutes.Connections()))
	mux.Handle(config.WebAttentionPolicies, strictProtectedChain(attentionpolicy.Handler(appRoutes.ApplicationContext, time.Now)))
	mux.Handle(config.WebLanes, strictProtectedChain(lane.Handler(appRoutes.ApplicationContext, time.Now)))
	mux.Handle(config.WebOrganizers, strictProtectedChain(organizer.Handler(appRoutes.ApplicationContext)))
	mux.Handle(config.WebProbes, strictProtectedChain(probe.Handler(appRoutes.ApplicationContext, time.Now)))
	horizonHandler := horizon.Handler(appRoutes.ApplicationContext, time.Now)
	mux.Handle(config.WebHorizon, strictProtectedChain(horizonHandler))
	mux.Handle(config.WebRSVPQR, strictProtectedChain(http.HandlerFunc(rsvp.ShowHandler(appRoutes.ApplicationContext))))
	rsvpBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		appRoutes.ApplicationContext.Logger.Printf("Router: Protected path %s, method %s", request.URL.Path, request.Method)
		switch request.Method {
		case http.MethodGet:
			rsvp.ListHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			rsvp.CreateHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut, http.MethodPatch:
			rsvp.UpdateHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			rsvp.DeleteHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, appRoutes.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		}
	})
	mux.Handle(config.WebRSVPs, protectedChain(rsvpBaseDispatcher))
	venueBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		appRoutes.ApplicationContext.Logger.Printf("Router: Protected path %s, method %s", request.URL.Path, request.Method)
		switch request.Method {
		case http.MethodGet:
			venue.ListVenuesHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			venue.CreateVenueHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			venue.DeleteVenueHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut, http.MethodPatch:
			venue.UpdateVenueHandler(appRoutes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, appRoutes.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		}
	})
	mux.Handle(config.WebVenues, protectedChain(venueBaseDispatcher))
	appRoutes.ApplicationContext.Logger.Println("Application-specific routes registered successfully.")
}

// RunCalendarSyncClock immediately reconciles calendars and repeats until shutdown.
func (appRoutes *Routes) RunCalendarSyncClock(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("calendar synchronization interval must be positive")
	}
	if appRoutes.calendarConnectionRoutes == nil {
		return errors.New("calendar synchronization is unavailable")
	}
	synchronize := func() {
		if synchronizationError := appRoutes.calendarConnectionRoutes.SynchronizeAll(ctx); synchronizationError != nil && !errors.Is(synchronizationError, context.Canceled) {
			appRoutes.ApplicationContext.Logger.Printf("ERROR: Automatic calendar synchronization failed: %v", synchronizationError)
		}
	}
	synchronize()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			synchronize()
		}
	}
}

// RunCalendarConnectionTasks executes durable calendar import tasks until shutdown.
func (appRoutes *Routes) RunCalendarConnectionTasks(ctx context.Context) error {
	if appRoutes.calendarConnectionRoutes == nil {
		return errors.New("calendar connection tasks are unavailable")
	}
	appRoutes.calendarConnectionRoutes.RunTaskWorker(ctx)
	return nil
}
