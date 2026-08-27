// Package routes holds application-wide shared resources and environment configuration.
package routes

import (
	"github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/gauss"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/handlers/attentionpolicy"
	"github.com/tyemirov/RSVP/pkg/handlers/calendar"
	"github.com/tyemirov/RSVP/pkg/handlers/calendarconnection"
	"github.com/tyemirov/RSVP/pkg/handlers/derivedmarker"
	"github.com/tyemirov/RSVP/pkg/handlers/event"
	"github.com/tyemirov/RSVP/pkg/handlers/horizon"
	"github.com/tyemirov/RSVP/pkg/handlers/lane"
	"github.com/tyemirov/RSVP/pkg/handlers/probe"
	"github.com/tyemirov/RSVP/pkg/handlers/response"
	"github.com/tyemirov/RSVP/pkg/handlers/rsvp"
	"github.com/tyemirov/RSVP/pkg/handlers/venue"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
	staticassets "github.com/tyemirov/RSVP/static"
	"html/template"
	"net/http"
	"path/filepath"
	"time"
)

// Routes holds shared resources and environment configuration.
type Routes struct {
	ApplicationContext *config.ApplicationContext
	EnvConfig          *config.EnvConfig
}

// New creates and returns a new Routes instance.
func New(applicationContext *config.ApplicationContext, envConfig config.EnvConfig) *Routes {
	return &Routes{
		ApplicationContext: applicationContext,
		EnvConfig:          &envConfig,
	}
}

// LandingPageHandler serves the landing page.
func (appRoutes *Routes) LandingPageHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != config.WebRoot {
		http.NotFound(responseWriter, request)
		return
	}
	webSession, sessionError := session.Store().Get(request, constants.SessionName)
	if sessionError != nil {
		appRoutes.ApplicationContext.Logger.Printf("ERROR: Session error on path %s: %v", request.URL.Path, sessionError)
	}
	userEmail := ""
	if webSession != nil && webSession.Values != nil {
		if emailValue, emailExists := webSession.Values[constants.SessionKeyUserEmail].(string); emailExists {
			userEmail = emailValue
		}
	}
	if userEmail != "" {
		http.Redirect(responseWriter, request, config.WebHorizon, http.StatusFound)
		return
	}
	landingTemplatePath := filepath.Join(config.TemplatesDir, config.TemplateLanding+config.TemplateExtension)
	landingTemplate, parseError := template.ParseFiles(landingTemplatePath)
	if parseError != nil {
		appRoutes.ApplicationContext.Logger.Printf("FATAL: Parsing landing template '%s' failed: %v", landingTemplatePath, parseError)
		utils.HandleError(responseWriter, parseError, utils.ServerError, appRoutes.ApplicationContext.Logger, "Could not display the page.")
		return
	}
	templateData := map[string]interface{}{
		config.ErrorQueryParam: request.URL.Query().Get(config.ErrorQueryParam),
	}
	executeError := landingTemplate.Execute(responseWriter, templateData)
	if executeError != nil {
		appRoutes.ApplicationContext.Logger.Printf("ERROR: Executing landing template '%s' failed: %v", landingTemplatePath, executeError)
	}
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

// RegisterMiddleware registers authentication and session middleware.
func (appRoutes *Routes) RegisterMiddleware(mux *http.ServeMux) {
	session.NewSession([]byte(appRoutes.EnvConfig.SessionSecret))
	landingTemplatePath := filepath.Join(config.TemplatesDir, config.TemplateLanding+config.TemplateExtension)
	authenticationService, authServiceError := gauss.NewService(
		appRoutes.EnvConfig.GoogleClientID,
		appRoutes.EnvConfig.GoogleClientSecret,
		appRoutes.EnvConfig.GoogleOauth2Base,
		config.WebHorizon,
		nil,
		landingTemplatePath,
	)
	if authServiceError != nil {
		appRoutes.ApplicationContext.Logger.Fatalf("FATAL: Initializing auth service failed: %v", authServiceError)
	}
	gaussHandlers, gaussHandlersError := gauss.NewHandlers(authenticationService)
	if gaussHandlersError != nil {
		appRoutes.ApplicationContext.Logger.Fatalf("FATAL: Initializing auth handlers failed: %v", gaussHandlersError)
	}
	gaussHandlers.RegisterRoutes(mux)
	appRoutes.ApplicationContext.Logger.Println("GAuss authentication middleware and routes registered.")
}

// RegisterRoutes registers all application routes.
func (appRoutes *Routes) RegisterRoutes(mux *http.ServeMux) {
	authRequired := gauss.AuthMiddleware
	addUserMiddleware := middleware.AddUserToContext(appRoutes.ApplicationContext)
	applyOverrides := appRoutes.ApplyOverrides
	protectedChain := func(handler http.Handler) http.Handler {
		return authRequired(addUserMiddleware(applyOverrides(handler)))
	}
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
	mux.Handle(config.WebCalendars, protectedChain(calendar.Handler(appRoutes.ApplicationContext)))
	derivedMarkerResources, derivedMarkerError := derivedmarker.New(appRoutes.ApplicationContext)
	if derivedMarkerError != nil {
		panic(derivedMarkerError)
	}
	mux.Handle(config.WebDerivedMarkerRules, protectedChain(derivedMarkerResources.Handler()))
	calendarConnectionResources, calendarConnectionError := calendarconnection.New(appRoutes.ApplicationContext, *appRoutes.EnvConfig, time.Now)
	if calendarConnectionError == nil {
		mux.Handle(config.WebCalendarAuthorizationRequests, protectedChain(calendarConnectionResources.AuthorizationRequests()))
		mux.Handle(config.WebCalendarConnectionCallbacksGoogle, protectedChain(calendarConnectionResources.Callback()))
		mux.Handle(config.WebCalendarConnections, protectedChain(calendarConnectionResources.Connections()))
		mux.Handle(config.WebCalendarSyncs, protectedChain(calendarConnectionResources.CalendarSyncs()))
	} else {
		unavailable := http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
			if responseError := handlers.WriteTypedError(responseWriter, http.StatusServiceUnavailable, "calendar_connection_unavailable", "Calendar connection is unavailable."); responseError != nil {
				appRoutes.ApplicationContext.Logger.Printf("ERROR: Write calendar connection unavailable response: %v", responseError)
			}
		})
		mux.Handle(config.WebCalendarAuthorizationRequests, protectedChain(unavailable))
		mux.Handle(config.WebCalendarConnectionCallbacksGoogle, protectedChain(unavailable))
		mux.Handle(config.WebCalendarConnections, protectedChain(unavailable))
		mux.Handle(config.WebCalendarSyncs, protectedChain(unavailable))
	}
	mux.Handle(config.WebAttentionPolicies, protectedChain(attentionpolicy.Handler(appRoutes.ApplicationContext, time.Now)))
	mux.Handle(config.WebLanes, protectedChain(lane.Handler(appRoutes.ApplicationContext, time.Now)))
	mux.Handle(config.WebProbes, protectedChain(probe.Handler(appRoutes.ApplicationContext, time.Now)))
	horizonHandler := horizon.Handler(appRoutes.ApplicationContext, time.Now)
	mux.Handle(config.WebHorizon, horizon.AuthenticationMiddleware(appRoutes.ApplicationContext, addUserMiddleware(applyOverrides(horizonHandler))))
	mux.Handle(config.WebRSVPQR, authRequired(addUserMiddleware(http.HandlerFunc(rsvp.ShowHandler(appRoutes.ApplicationContext)))))
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
