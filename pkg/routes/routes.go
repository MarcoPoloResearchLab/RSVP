package routes

import (
	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers/event"
	"net/http"
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
	// Register event routes using the authentication middleware.
	//mux.Handle(config.WebEvents, gauss.AuthMiddleware(event.List(routes.ApplicationContext)))
	mux.Handle(config.WebEvents, gauss.AuthMiddleware(event.EventIndexHandler(routes.ApplicationContext)))

	//mux.HandleFunc("/events/new", handlers.EventNew)
	//mux.HandleFunc("/events/create", handlers.EventCreate)
	//mux.HandleFunc("/events/show", handlers.EventShow)
	//mux.HandleFunc("/events/edit", handlers.EventEdit)
	//mux.HandleFunc("/events/update", handlers.EventUpdate)
	//mux.HandleFunc("/events/delete", handlers.EventDelete)
	//
	//// RSVP routes (nested under events by convention)
	//mux.HandleFunc("/events/rsvps", handlers.RSVPList)
	//mux.HandleFunc("/events/rsvps/new", handlers.RSVPNew)
	//mux.HandleFunc("/events/rsvps/create", handlers.RSVPCreate)
	//mux.HandleFunc("/events/rsvps/edit", handlers.RSVPEdit)
	//mux.HandleFunc("/events/rsvps/update", handlers.RSVPUpdate)
	//mux.HandleFunc("/events/rsvps/delete", handlers.RSVPDelete)
}
