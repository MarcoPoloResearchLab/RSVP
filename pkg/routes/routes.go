package routes

import (
	"net/http"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers/event"
	"github.com/temirov/RSVP/pkg/handlers/rsvp"
)

type Routes struct {
	ApplicationContext *config.ApplicationContext
	EnvConfig          *config.EnvConfig
}

func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) *Routes {
	return &Routes{
		applicationContext,
		&envConfig,
	}
}

func (routes Routes) RootRedirectHandler(responseWriter http.ResponseWriter, request *http.Request) {
	webSession, sessionError := session.Store().Get(request, gconstants.SessionName)
	if sessionError != nil {
		http.Error(responseWriter, "Internal server error", http.StatusInternalServerError)
		return
	}

	if userEmail, exists := webSession.Values[gconstants.SessionKeyUserEmail]; exists {
		if userEmailString, ok := userEmail.(string); ok && userEmailString != "" {
			http.Redirect(responseWriter, request, config.WebEvents, http.StatusFound)
			return
		}
	}

	// If not authenticated, redirect to the Google authentication route.
	http.Redirect(responseWriter, request, gconstants.GoogleAuthPath, http.StatusFound)
}

func (routes Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(routes.EnvConfig.SessionSecret))
	authenticationService, authServiceErr := gauss.NewService(
		routes.EnvConfig.GoogleClientID,
		routes.EnvConfig.GoogleClientSecret,
		routes.EnvConfig.GoogleOauth2Base,
		config.WebRoot,
	)
	if authServiceErr != nil {
		routes.ApplicationContext.Logger.Fatal("Failed to initialize auth service:", authServiceErr)
	}

	gaussHandlers, gaussHandlersErr := gauss.NewHandlers(authenticationService)
	if gaussHandlersErr != nil {
		routes.ApplicationContext.Logger.Fatal("Failed to initialize auth handlers:", gaussHandlersErr)
	}
	gaussHandlers.RegisterRoutes(mux)
}

func (routes Routes) RegisterRoutes(mux *http.ServeMux) {
	// Register the root route with the dedicated handler.
	mux.HandleFunc(config.WebRoot, routes.RootRedirectHandler)

	// Register event routes using the event router
	mux.Handle(config.WebEvents, gauss.AuthMiddleware(event.EventRouter(routes.ApplicationContext)))

	// Register RSVP routes with visualization support
	mux.HandleFunc(config.WebRSVPs, func(w http.ResponseWriter, r *http.Request) {
		// Check if the print parameter is present for visualization
		if r.URL.Query().Get("print") == "true" {
			// Use the visualization handler
			gauss.AuthMiddleware(rsvp.VisualizationHandler(routes.ApplicationContext)).ServeHTTP(w, r)
		} else {
			// Use the regular RSVP router
			gauss.AuthMiddleware(rsvp.RSVPRouter(routes.ApplicationContext)).ServeHTTP(w, r)
		}
	})

	// Register unprotected RSVP response route
	mux.HandleFunc("/rsvp", rsvp.ResponseHandler(routes.ApplicationContext))
}
