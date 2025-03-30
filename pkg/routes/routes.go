// Package routes defines the HTTP routing, middleware application, and
// connection between URL paths and their corresponding handlers for the RSVP application.
package routes

import (
	"html/template" // Import html/template
	"net/http"
	"path/filepath"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"

	"github.com/temirov/RSVP/pkg/config"
	// base handlers no longer needed by LandingPageHandler
	"github.com/temirov/RSVP/pkg/handlers/event"
	"github.com/temirov/RSVP/pkg/handlers/response"
	"github.com/temirov/RSVP/pkg/handlers/rsvp"
	"github.com/temirov/RSVP/pkg/utils" // Keep for HandleError
)

// Routes holds shared application context and configuration necessary for defining routes.
type Routes struct {
	ApplicationContext *config.ApplicationContext
	EnvConfig          *config.EnvConfig
}

// New creates and returns a new Routes structure populated with application context and environment configuration.
func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) *Routes {
	return &Routes{ApplicationContext: applicationContext, EnvConfig: &envConfig}
}

// LandingPageHandler handles requests to the root path ("/").
// If the user is logged in (session exists), it redirects to the events dashboard (config.WebEvents).
// If the user is logged out, it parses and executes the standalone landing page template directly.
func (routes *Routes) LandingPageHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != config.WebRoot {
		http.NotFound(responseWriter, request)
		return
	}

	webSession, sessionError := session.Store().Get(request, gconstants.SessionName)
	if sessionError != nil {
		routes.ApplicationContext.Logger.Printf("ERROR: Session error on root access: %v", sessionError)
	}

	userEmail, _ := webSession.Values[gconstants.SessionKeyUserEmail].(string)

	if userEmail != "" {
		http.Redirect(responseWriter, request, config.WebEvents, http.StatusFound)
		return
	}

	// Logged out: Render the standalone landing page directly
	landingTemplatePath := filepath.Join(config.TemplatesDir, "landing"+config.TemplateExtension)

	tmpl, err := template.ParseFiles(landingTemplatePath)
	if err != nil {
		routes.ApplicationContext.Logger.Printf("CRITICAL: Failed to parse standalone landing template '%s': %v", landingTemplatePath, err)
		utils.HandleError(responseWriter, err, utils.ServerError, routes.ApplicationContext.Logger, "Could not display page.")
		return
	}

	// Pass potential error from query param (though less likely to hit '/' with error)
	templateData := map[string]interface{}{
		"error": request.URL.Query().Get("error"),
	}
	err = tmpl.Execute(responseWriter, templateData)
	if err != nil {
		routes.ApplicationContext.Logger.Printf("ERROR: Failed to execute standalone landing template '%s': %v", landingTemplatePath, err)
	}
}

// ApplyOverrides middleware... (remains the same)
func (routes *Routes) ApplyOverrides(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			if request.Form == nil {
				request.Body = http.MaxBytesReader(responseWriter, request.Body, 10<<20)
				if err := request.ParseForm(); err != nil {
					routes.ApplicationContext.Logger.Printf("WARN: Failed to parse form data in ApplyOverrides for %s: %v", request.URL.Path, err)
				}
			}
			methodOverride := request.FormValue(config.MethodOverrideParam)
			if methodOverride != "" {
				switch methodOverride {
				case http.MethodPut, http.MethodPatch, http.MethodDelete:
					originalMethod := request.Method
					request.Method = methodOverride
					routes.ApplicationContext.Logger.Printf("DEBUG: Method overridden from %s to %s for %s", originalMethod, request.Method, request.URL.Path)
				default:
					routes.ApplicationContext.Logger.Printf("WARN: Invalid _method override value '%s' received for %s", methodOverride, request.URL.Path)
				}
			}
		}
		next.ServeHTTP(responseWriter, request)
	})
}

// RegisterMiddleware configures GAuss service and registers its routes.
// GAuss is configured to use the standalone landing.tmpl for its /login route.
func (routes *Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(routes.EnvConfig.SessionSecret))

	landingTemplatePath := filepath.Join(config.TemplatesDir, "landing"+config.TemplateExtension)

	authenticationService, authServiceError := gauss.NewService(
		routes.EnvConfig.GoogleClientID,
		routes.EnvConfig.GoogleClientSecret,
		routes.EnvConfig.GoogleOauth2Base,
		config.WebEvents,    // Redirect here after successful Google login
		landingTemplatePath, // GAuss uses the standalone landing page for /login
	)
	if authServiceError != nil {
		routes.ApplicationContext.Logger.Fatal("FATAL: Failed to initialize auth service:", authServiceError)
	}

	gaussHandlers, gaussHandlersError := gauss.NewHandlers(authenticationService)
	if gaussHandlersError != nil {
		routes.ApplicationContext.Logger.Fatal("FATAL: Failed to initialize auth handlers:", gaussHandlersError)
	}

	gaussHandlers.RegisterRoutes(mux) // Let GAuss register /login, /logout, /auth/*

	routes.ApplicationContext.Logger.Println("GAuss authentication middleware registered.")
}

// RegisterRoutes defines RSVP's application-specific routes.
func (routes *Routes) RegisterRoutes(mux *http.ServeMux) {
	authRequired := gauss.AuthMiddleware
	overrideHandler := routes.ApplyOverrides

	mux.HandleFunc(config.WebRoot, routes.LandingPageHandler) // Handles '/'

	// Protected routes... (remain the same)
	eventBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: /events/ effective method: %s, path: %s", request.Method, request.URL.Path)
		switch request.Method {
		case http.MethodGet:
			event.ListEventsHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			event.CreateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut:
			event.UpdateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			event.DeleteHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			http.Error(responseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})
	mux.Handle(config.WebEvents, authRequired(overrideHandler(eventBaseDispatcher)))
	mux.Handle(config.WebRSVPQR, authRequired(http.HandlerFunc(rsvp.ShowHandler(routes.ApplicationContext))))
	rsvpBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: /rsvps/ effective method: %s, path: %s", request.Method, request.URL.Path)
		switch request.Method {
		case http.MethodGet:
			rsvp.ListHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			rsvp.CreateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut:
			rsvp.UpdateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			rsvp.DeleteHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			http.Error(responseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})
	mux.Handle(config.WebRSVPs, authRequired(overrideHandler(rsvpBaseDispatcher)))

	responseBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: /response/ effective method: %s, path: %s", request.Method, request.URL.Path)
		response.Handler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
	})
	mux.Handle(config.WebResponse, overrideHandler(responseBaseDispatcher))
	mux.HandleFunc(config.WebResponseThankYou, response.ThankYouHandler(routes.ApplicationContext))

	routes.ApplicationContext.Logger.Println("Application routes registered successfully.")
}
