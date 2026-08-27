// Package horizon provides the authenticated horizon projection handler.
package horizon

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	gaussConstants "github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/utils"
)

const (
	horizonCacheControl               = "private, no-store"
	horizonAuthenticationErrorCode    = "authentication_required"
	horizonAuthenticationErrorMessage = "Authentication is required."
	horizonErrorCode                  = "invalid_time_window"
	horizonErrorMessage               = "The time window is invalid."
	horizonJSONMediaType              = handlers.JSONMediaType
	horizonHTMLMediaType              = "text/html"
)

var errMalformedHorizonWindow = errors.New("malformed horizon window")

type handler struct {
	baseHandler *handlers.BaseHttpHandler
	projector   *services.HorizonProjectionService
	now         func() time.Time
}

// Handler returns the authenticated horizon HTTP handler.
func Handler(applicationContext *config.ApplicationContext, now func() time.Time) http.Handler {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameHorizon, config.WebHorizon)
	projector, projectionServiceError := services.NewHorizonProjectionService(applicationContext.Database)
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if projectionServiceError != nil || now == nil {
			baseHandler.HandleError(responseWriter, projectionServiceError, utils.ServerError, utils.ErrMsgInternalServer)
			return
		}
		(&handler{baseHandler: &baseHandler, projector: projector, now: now}).serveHTTP(responseWriter, request)
	})
}

// AuthenticationMiddleware enforces the horizon authentication response contract.
func AuthenticationMiddleware(applicationContext *config.ApplicationContext, next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		webSession, sessionError := session.Store().Get(request, gaussConstants.SessionName)
		if sessionError == nil {
			userEmail, emailFound := webSession.Values[gaussConstants.SessionKeyUserEmail].(string)
			if emailFound && userEmail != "" {
				next.ServeHTTP(responseWriter, request)
				return
			}
		}

		setProjectionHeaders(responseWriter.Header())
		representation, representationAccepted := selectRepresentation(request.Header.Get("Accept"))
		if representationAccepted && representation == horizonHTMLMediaType {
			http.Redirect(responseWriter, request, gaussConstants.LoginPath, http.StatusFound)
			return
		}
		if responseError := handlers.WriteTypedError(
			responseWriter,
			http.StatusUnauthorized,
			horizonAuthenticationErrorCode,
			horizonAuthenticationErrorMessage,
		); responseError != nil {
			applicationContext.Logger.Printf("ERROR: Write horizon authentication response: %v", responseError)
		}
	})
}

func (horizonHandler *handler) serveHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	setProjectionHeaders(responseWriter.Header())
	if request.URL.Path != config.WebHorizon {
		http.NotFound(responseWriter, request)
		return
	}
	if request.Method != http.MethodGet {
		responseWriter.Header().Set("Allow", http.MethodGet)
		utils.HandleError(responseWriter, nil, utils.MethodNotAllowedError, horizonHandler.baseHandler.ApplicationContext.Logger, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	representation, representationAccepted := selectRepresentation(request.Header.Get("Accept"))
	if !representationAccepted {
		http.Error(responseWriter, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
		return
	}

	currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
	if !userFound || currentUser == nil || currentUser.ID == "" {
		utils.HandleError(responseWriter, nil, utils.AuthenticationError, horizonHandler.baseHandler.ApplicationContext.Logger, utils.ErrMsgUnauthorized)
		return
	}
	window, windowError := horizonHandler.windowFromRequest(request, currentUser)
	if windowError != nil {
		statusCode := http.StatusUnprocessableEntity
		if errors.Is(windowError, errMalformedHorizonWindow) {
			statusCode = http.StatusBadRequest
		}
		horizonHandler.writeInvalidWindow(responseWriter, statusCode, windowError)
		return
	}
	projection, projectionError := horizonHandler.projector.Project(request.Context(), currentUser.ID, window)
	if projectionError != nil {
		horizonHandler.baseHandler.HandleError(responseWriter, projectionError, utils.DatabaseError, utils.ErrMsgInternalServer)
		return
	}

	if representation == horizonJSONMediaType {
		responseWriter.Header().Set("Content-Type", horizonJSONMediaType)
		if encodeError := json.NewEncoder(responseWriter).Encode(projection); encodeError != nil {
			horizonHandler.baseHandler.ApplicationContext.Logger.Printf("ERROR: Encode horizon projection for organizer %s: %v", currentUser.ID, encodeError)
		}
		return
	}
	viewData, viewDataError := newHorizonViewData(projection, horizonHandler.now())
	if viewDataError != nil {
		horizonHandler.baseHandler.HandleError(responseWriter, viewDataError, utils.ServerError, utils.ErrMsgInternalServer)
		return
	}
	responseWriter.Header().Set("Content-Type", horizonHTMLMediaType+"; charset=utf-8")
	horizonHandler.baseHandler.RenderView(responseWriter, request, config.TemplateHorizon, viewData)
}

func (horizonHandler *handler) windowFromRequest(request *http.Request, currentUser *models.User) (services.HorizonWindow, error) {
	if currentUser.Timezone == nil {
		return services.HorizonWindow{}, models.ErrOrganizerTimezoneRequired
	}
	timezone, timezoneError := models.NewTimezone(*currentUser.Timezone)
	if timezoneError != nil {
		return services.HorizonWindow{}, timezoneError
	}
	query := request.URL.Query()
	startValues, startPresent := query[config.HorizonStartParam]
	endValues, endPresent := query[config.HorizonEndParam]
	if !startPresent && !endPresent {
		return services.NewDefaultHorizonWindow(horizonHandler.now(), timezone)
	}
	if !startPresent || !endPresent || len(startValues) != 1 || len(endValues) != 1 || startValues[0] == "" || endValues[0] == "" {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	start, startError := time.Parse(time.RFC3339, startValues[0])
	if startError != nil {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	end, endError := time.Parse(time.RFC3339, endValues[0])
	if endError != nil {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	return services.NewHorizonWindow(start, end, timezone)
}

func (horizonHandler *handler) writeInvalidWindow(responseWriter http.ResponseWriter, statusCode int, windowError error) {
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, horizonErrorCode, horizonErrorMessage); responseError != nil {
		horizonHandler.baseHandler.ApplicationContext.Logger.Printf("ERROR: Write invalid horizon window response: %v", errors.Join(windowError, responseError))
	}
}

func setProjectionHeaders(responseHeaders http.Header) {
	responseHeaders.Set("Cache-Control", horizonCacheControl)
	responseHeaders.Set("Vary", "Accept")
}

func selectRepresentation(acceptHeader string) (string, bool) {
	if strings.TrimSpace(acceptHeader) == "" {
		return horizonHTMLMediaType, true
	}
	selectedType := ""
	selectedQuality := -1.0
	for _, acceptValue := range strings.Split(acceptHeader, ",") {
		mediaType, parameters, parseError := mime.ParseMediaType(strings.TrimSpace(acceptValue))
		if parseError != nil {
			continue
		}
		quality := 1.0
		if qualityValue, qualityFound := parameters["q"]; qualityFound {
			parsedQuality, qualityError := strconv.ParseFloat(qualityValue, 64)
			if qualityError != nil || parsedQuality < 0 || parsedQuality > 1 {
				continue
			}
			quality = parsedQuality
		}
		candidate := ""
		switch mediaType {
		case horizonJSONMediaType:
			candidate = horizonJSONMediaType
		case horizonHTMLMediaType, "*/*":
			candidate = horizonHTMLMediaType
		}
		if candidate != "" && quality > 0 && quality > selectedQuality {
			selectedType = candidate
			selectedQuality = quality
		}
	}
	return selectedType, selectedType != ""
}
