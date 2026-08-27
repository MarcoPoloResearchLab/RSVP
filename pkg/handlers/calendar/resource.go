// Package calendar provides authenticated calendar resource handlers.
package calendar

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

const (
	calendarCacheControl = "private, no-store"
	calendarBodyBytes    = 4096
)

type createRequest struct {
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	ColorToken string `json:"color_token"`
	Timezone   string `json:"timezone"`
}

type patchRequest struct {
	Name         *string `json:"name"`
	Symbol       *string `json:"symbol"`
	ColorToken   *string `json:"color_token"`
	DisplayOrder *int    `json:"display_order"`
	Visible      *bool   `json:"visible"`
}

type representation struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Symbol       string `json:"symbol"`
	ColorToken   string `json:"color_token"`
	DisplayOrder int    `json:"display_order"`
	Visible      bool   `json:"visible"`
}

// Handler returns the canonical calendar collection and item handler.
func Handler(applicationContext *config.ApplicationContext) http.Handler {
	service, serviceError := services.NewTemporalManagementService(applicationContext.Database, func() time.Time { return time.Now().UTC() })
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", calendarCacheControl)
		if serviceError != nil {
			writeError(applicationContext, responseWriter, http.StatusInternalServerError, "calendar_service_failed", "Calendar management is unavailable.", serviceError)
			return
		}
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path == config.WebCalendars {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
				return
			}
			handleCreate(applicationContext, service, currentUser, responseWriter, request)
			return
		}
		calendarID, pathValid := handlers.ResourceIDFromPath(config.WebCalendars, request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		switch request.Method {
		case http.MethodGet:
			handleRead(applicationContext, service, currentUser.ID, calendarID, responseWriter, request)
		case http.MethodPatch:
			handleUpdate(applicationContext, service, currentUser.ID, calendarID, responseWriter, request)
		case http.MethodDelete:
			handleDelete(applicationContext, service, currentUser.ID, calendarID, responseWriter, request)
		default:
			responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch+", "+http.MethodDelete)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
		}
	})
}

func handleCreate(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, currentUser *models.User, responseWriter http.ResponseWriter, request *http.Request) {
	var body createRequest
	if decodeError := handlers.DecodeJSON(responseWriter, request, calendarBodyBytes, &body); decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Symbol) == "" || strings.TrimSpace(body.ColorToken) == "" {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_calendar", "Calendar fields are invalid.", nil)
		return
	}
	timezone, timezoneError := models.NewTimezone(body.Timezone)
	if timezoneError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_timezone", "Timezone is invalid.", timezoneError)
		return
	}
	calendar, createError := service.CreateCalendar(request.Context(), currentUser, timezone, body.Name, body.Symbol, body.ColorToken)
	if createError != nil {
		writeServiceError(applicationContext, responseWriter, createError)
		return
	}
	responseWriter.Header().Set("Location", config.WebCalendars+calendar.ID)
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusCreated, calendarRepresentation(calendar)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar creation response: %v", encodeError)
	}
}

func handleRead(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, calendarID string, responseWriter http.ResponseWriter, request *http.Request) {
	calendar, readError := service.ReadCalendar(request.Context(), organizerID, calendarID)
	if readError != nil {
		writeServiceError(applicationContext, responseWriter, readError)
		return
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, calendarRepresentation(calendar)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar response: %v", encodeError)
	}
}

func handleUpdate(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, calendarID string, responseWriter http.ResponseWriter, request *http.Request) {
	var body patchRequest
	if decodeError := handlers.DecodeJSON(responseWriter, request, calendarBodyBytes, &body); decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	if body.Name == nil && body.Symbol == nil && body.ColorToken == nil && body.DisplayOrder == nil && body.Visible == nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_calendar_patch", "Calendar patch is empty.", nil)
		return
	}
	if (body.Name != nil && strings.TrimSpace(*body.Name) == "") || (body.Symbol != nil && strings.TrimSpace(*body.Symbol) == "") ||
		(body.ColorToken != nil && strings.TrimSpace(*body.ColorToken) == "") || (body.DisplayOrder != nil && *body.DisplayOrder < 0) {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_calendar_patch", "Calendar patch is invalid.", nil)
		return
	}
	calendar, updateError := service.UpdateCalendar(request.Context(), organizerID, calendarID, services.CalendarPatch{
		Name: body.Name, Symbol: body.Symbol, ColorToken: body.ColorToken, DisplayOrder: body.DisplayOrder, Visible: body.Visible,
	})
	if updateError != nil {
		writeServiceError(applicationContext, responseWriter, updateError)
		return
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, calendarRepresentation(calendar)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar update response: %v", encodeError)
	}
}

func handleDelete(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, calendarID string, responseWriter http.ResponseWriter, request *http.Request) {
	if deleteError := service.DeleteCalendar(request.Context(), organizerID, calendarID); deleteError != nil {
		writeServiceError(applicationContext, responseWriter, deleteError)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func calendarRepresentation(calendar *models.Calendar) representation {
	return representation{ID: calendar.ID, Name: calendar.Name, Symbol: calendar.Symbol, ColorToken: calendar.ColorToken, DisplayOrder: calendar.DisplayOrder, Visible: calendar.Visible}
}

func writeDecodeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, decodeError error) {
	if errors.Is(decodeError, handlers.ErrUnsupportedJSONMediaType) {
		writeError(applicationContext, responseWriter, http.StatusUnsupportedMediaType, "unsupported_media_type", http.StatusText(http.StatusUnsupportedMediaType), decodeError)
		return
	}
	writeError(applicationContext, responseWriter, http.StatusBadRequest, "malformed_json", "Request JSON is malformed.", decodeError)
}

func writeServiceError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, serviceError error) {
	switch {
	case errors.Is(serviceError, services.ErrResourceForbidden):
		writeError(applicationContext, responseWriter, http.StatusForbidden, "calendar_forbidden", "Calendar access is forbidden.", serviceError)
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(applicationContext, responseWriter, http.StatusNotFound, "calendar_not_found", "Calendar was not found.", serviceError)
	case errors.Is(serviceError, services.ErrCalendarNotEmpty):
		writeError(applicationContext, responseWriter, http.StatusConflict, "calendar_not_empty", "Delete each calendar lane first.", serviceError)
	case errors.Is(serviceError, services.ErrDisplayOrderOutOfRange),
		errors.Is(serviceError, models.ErrCalendarNameRequired), errors.Is(serviceError, models.ErrCalendarPresentationRequired),
		errors.Is(serviceError, models.ErrDisplayOrderInvalid), errors.Is(serviceError, models.ErrTimezoneInvalid),
		errors.Is(serviceError, models.ErrTimezoneRequired):
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_calendar", "Calendar data is invalid.", serviceError)
	default:
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "calendar_operation_failed", "Calendar operation failed.", serviceError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar error response: %v", responseError)
	}
}
