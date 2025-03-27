// Package routes defines the HTTP routing, middleware application, and
// connection between URL paths and their corresponding handlers for the RSVP application.
package routes

import (
	"net/http"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers/event"
	"github.com/temirov/RSVP/pkg/handlers/response"
	"github.com/temirov/RSVP/pkg/handlers/rsvp"
)

// Routes holds shared application context and configuration necessary for routing.
type Routes struct {
	ApplicationContext *config.ApplicationContext
	EnvConfig          *config.EnvConfig
}

// New creates and returns a new Routes structure populated with application context and environment configuration.
func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) *Routes {
	return &Routes{ApplicationContext: applicationContext, EnvConfig: &envConfig}
}

// RootRedirectHandler directs traffic from the root path ("/") based on user authentication status.
// Authenticated users are sent to the events dashboard, while unauthenticated users are redirected to the login flow.
func (routes *Routes) RootRedirectHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != config.WebRoot {
		http.NotFound(responseWriter, request)
		return
	}

	webSession, sessionError := session.Store().Get(request, gconstants.SessionName)
	if sessionError != nil {
		routes.ApplicationContext.Logger.Printf("ERROR: Session error on root access: %v", sessionError)
		http.Error(responseWriter, "Session error", http.StatusInternalServerError)
		return
	}

	userEmail, _ := webSession.Values[gconstants.SessionKeyUserEmail].(string)

	if userEmail != "" {
		http.Redirect(responseWriter, request, config.WebEvents, http.StatusFound)
	} else {
		http.Redirect(responseWriter, request, gconstants.GoogleAuthPath, http.StatusFound)
	}
}

// ApplyOverrides is middleware that inspects POST requests for a `_method` form value
// and modifies the request method accordingly (e.g., to PUT or DELETE) before passing it to the next handler.
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

// RegisterMiddleware configures and registers authentication-related middleware and routes provided by the GAuss library.
// It initializes the session store and sets up Google OAuth2 authentication endpoints.
func (routes *Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(routes.EnvConfig.SessionSecret))

	authenticationService, authServiceError := gauss.NewService(
		routes.EnvConfig.GoogleClientID,
		routes.EnvConfig.GoogleClientSecret,
		routes.EnvConfig.GoogleOauth2Base,
		routes.EnvConfig.AppBaseURL,
		"",
	)
	if authServiceError != nil {
		routes.ApplicationContext.Logger.Fatal("FATAL: Failed to initialize authentication service:", authServiceError)
	}

	gaussHandlers, gaussHandlersError := gauss.NewHandlers(authenticationService)
	if gaussHandlersError != nil {
		routes.ApplicationContext.Logger.Fatal("FATAL: Failed to initialize authentication handlers:", gaussHandlersError)
	}

	gaussHandlers.RegisterRoutes(mux)
	routes.ApplicationContext.Logger.Println("GAuss authentication middleware registered.")
}

// RegisterRoutes defines all application-specific URL path to handler mappings.
// It applies necessary middleware like authentication checks and method overrides to the appropriate routes.
func (routes *Routes) RegisterRoutes(mux *http.ServeMux) {
	authRequired := gauss.AuthMiddleware
	overrideHandler := routes.ApplyOverrides

	mux.HandleFunc(config.WebRoot, routes.RootRedirectHandler)

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
