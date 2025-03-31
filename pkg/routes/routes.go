package routes

import (
	"html/template"
	"net/http"
	"path/filepath"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers/event"
	"github.com/temirov/RSVP/pkg/handlers/response"
	"github.com/temirov/RSVP/pkg/handlers/rsvp"
	"github.com/temirov/RSVP/pkg/middleware" // Import middleware package
	"github.com/temirov/RSVP/pkg/utils"
)

// Routes holds shared application context and configuration necessary for defining routes and middleware.
type Routes struct {
	// ApplicationContext provides access to shared resources like database, logger, config.
	ApplicationContext *config.ApplicationContext
	// EnvConfig holds environment-specific configuration values.
	EnvConfig *config.EnvConfig
}

// New creates and returns a new Routes structure populated with application context and environment configuration.
func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) *Routes {
	return &Routes{ApplicationContext: applicationContext, EnvConfig: &envConfig}
}

// LandingPageHandler handles requests to the root path ("/").
// If the user is logged in (based on session data), it redirects them to the events dashboard.
// If the user is logged out, it parses and executes the standalone landing page template directly.
func (routes *Routes) LandingPageHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != config.WebRoot {
		http.NotFound(responseWriter, request)
		return
	}

	webSession, sessionError := session.Store().Get(request, gconstants.SessionName)
	if sessionError != nil {
		routes.ApplicationContext.Logger.Printf("ERROR: Session error on root access for path %s: %v", request.URL.Path, sessionError)
	}

	userEmail, _ := webSession.Values[gconstants.SessionKeyUserEmail].(string)

	if userEmail != "" {
		http.Redirect(responseWriter, request, config.WebEvents, http.StatusFound)
		return
	}

	landingTemplatePath := filepath.Join(config.TemplatesDir, config.TemplateLanding+config.TemplateExtension)

	landingTemplate, parseError := template.ParseFiles(landingTemplatePath)
	if parseError != nil {
		routes.ApplicationContext.Logger.Printf("CRITICAL: Failed to parse standalone landing template '%s': %v", landingTemplatePath, parseError)
		utils.HandleError(responseWriter, parseError, utils.ServerError, routes.ApplicationContext.Logger, "Could not display the page.")
		return
	}

	templateData := map[string]interface{}{
		config.ErrorQueryParam: request.URL.Query().Get(config.ErrorQueryParam),
	}

	executeError := landingTemplate.Execute(responseWriter, templateData)
	if executeError != nil {
		routes.ApplicationContext.Logger.Printf("ERROR: Failed to execute standalone landing template '%s': %v", landingTemplatePath, executeError)
	}
}

// ApplyOverrides is middleware that checks POST requests for a `_method` form field.
// If found and the value is PUT, PATCH, or DELETE, it overrides the request.Method for downstream handlers.
func (routes *Routes) ApplyOverrides(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			if request.Form == nil {
				request.Body = http.MaxBytesReader(responseWriter, request.Body, 10<<20)
				if parseError := request.ParseForm(); parseError != nil {
					routes.ApplicationContext.Logger.Printf("WARN: Failed to parse form data in ApplyOverrides for %s: %v", request.URL.Path, parseError)
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

// RegisterMiddleware configures and registers middleware, primarily the GAuss authentication service.
func (routes *Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(routes.EnvConfig.SessionSecret))

	landingTemplatePath := filepath.Join(config.TemplatesDir, config.TemplateLanding+config.TemplateExtension)

	// Initialize GAuss without DB/Upsert logic here
	authenticationService, authServiceError := gauss.NewService(
		routes.EnvConfig.GoogleClientID,
		routes.EnvConfig.GoogleClientSecret,
		routes.EnvConfig.GoogleOauth2Base,
		config.WebEvents,
		landingTemplatePath,
	)
	if authServiceError != nil {
		routes.ApplicationContext.Logger.Fatalf("FATAL: Failed to initialize auth service: %v", authServiceError)
	}

	gaussHandlers, gaussHandlersError := gauss.NewHandlers(authenticationService)
	if gaussHandlersError != nil {
		routes.ApplicationContext.Logger.Fatalf("FATAL: Failed to initialize auth handlers: %v", gaussHandlersError)
	}

	gaussHandlers.RegisterRoutes(mux)

	routes.ApplicationContext.Logger.Println("GAuss authentication middleware and routes registered.")
}

// RegisterRoutes defines the application-specific routes for Events, RSVPs, and Responses.
// It applies authentication, user context, and method override middleware where needed.
func (routes *Routes) RegisterRoutes(mux *http.ServeMux) {
	authRequired := gauss.AuthMiddleware
	addUserMiddleware := middleware.AddUserToContext(routes.ApplicationContext)
	applyOverrides := routes.ApplyOverrides
	// Chain for protected routes: Auth -> Add User -> Apply Overrides -> Handler
	protectedChain := func(handler http.Handler) http.Handler {
		return authRequired(addUserMiddleware(applyOverrides(handler)))
	}
	// Chain for public routes needing override: Apply Overrides -> Handler
	publicChainWithOverride := func(handler http.Handler) http.Handler {
		return applyOverrides(handler)
	}

	mux.HandleFunc(config.WebRoot, routes.LandingPageHandler)

	responseBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: Path %s, Effective Method: %s", request.URL.Path, request.Method)
		response.Handler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
	})
	mux.Handle(config.WebResponse, publicChainWithOverride(responseBaseDispatcher))

	mux.HandleFunc(config.WebResponseThankYou, response.ThankYouHandler(routes.ApplicationContext))

	eventBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: Path %s, Effective Method: %s", request.URL.Path, request.Method)
		switch request.Method {
		case http.MethodGet:
			event.ListEventsHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			event.CreateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut, http.MethodPatch:
			event.UpdateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			event.DeleteHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, routes.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		}
	})
	mux.Handle(config.WebEvents, protectedChain(eventBaseDispatcher))

	mux.Handle(config.WebRSVPQR, authRequired(addUserMiddleware(http.HandlerFunc(rsvp.ShowHandler(routes.ApplicationContext))))) // QR needs user context but no override

	rsvpBaseDispatcher := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routes.ApplicationContext.Logger.Printf("Router: Path %s, Effective Method: %s", request.URL.Path, request.Method)
		switch request.Method {
		case http.MethodGet:
			rsvp.ListHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPost:
			rsvp.CreateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodPut, http.MethodPatch:
			rsvp.UpdateHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		case http.MethodDelete:
			rsvp.DeleteHandler(routes.ApplicationContext).ServeHTTP(responseWriter, request)
		default:
			utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, routes.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		}
	})
	mux.Handle(config.WebRSVPs, protectedChain(rsvpBaseDispatcher))

	routes.ApplicationContext.Logger.Println("Application-specific routes registered successfully.")
}
