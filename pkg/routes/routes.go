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
		ApplicationContext: applicationContext,
		EnvConfig:          &envConfig,
	}
}

func (routes Routes) RootRedirectHandler(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
	webSession, sessionError := session.Store().Get(httpRequest, gconstants.SessionName)
	if sessionError != nil {
		http.Error(httpResponseWriter, "Internal server error", http.StatusInternalServerError)
		return
	}

	if userEmailValue, exists := webSession.Values[gconstants.SessionKeyUserEmail]; exists {
		if userEmailString, ok := userEmailValue.(string); ok && userEmailString != "" {
			http.Redirect(httpResponseWriter, httpRequest, config.WebEvents, http.StatusFound)
			return
		}
	}

	http.Redirect(httpResponseWriter, httpRequest, gconstants.GoogleAuthPath, http.StatusFound)
}

func (routes Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(routes.EnvConfig.SessionSecret))
	authenticationService, authServiceError := gauss.NewService(
		routes.EnvConfig.GoogleClientID,
		routes.EnvConfig.GoogleClientSecret,
		routes.EnvConfig.GoogleOauth2Base,
		config.WebRoot,
	)
	if authServiceError != nil {
		routes.ApplicationContext.Logger.Fatal("Failed to initialize auth service:", authServiceError)
	}

	gaussHandlers, gaussHandlersError := gauss.NewHandlers(authenticationService)
	if gaussHandlersError != nil {
		routes.ApplicationContext.Logger.Fatal("Failed to initialize auth handlers:", gaussHandlersError)
	}
	gaussHandlers.RegisterRoutes(mux)
}

func (routes Routes) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(config.WebRoot, routes.RootRedirectHandler)
	mux.Handle(config.WebEvents, gauss.AuthMiddleware(event.EventRouter(routes.ApplicationContext)))
	mux.HandleFunc(config.WebRSVPs, func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.URL.Query().Get("print") == "true" {
			gauss.AuthMiddleware(rsvp.VisualizationHandler(routes.ApplicationContext)).ServeHTTP(httpResponseWriter, httpRequest)
		} else {
			gauss.AuthMiddleware(rsvp.RSVPRouter(routes.ApplicationContext)).ServeHTTP(httpResponseWriter, httpRequest)
		}
	})
	mux.HandleFunc("/rsvp", rsvp.ResponseHandler(routes.ApplicationContext))
}
