// Package organizer provides authenticated organizer resource handlers.
package organizer

import (
	"errors"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
)

const (
	organizerBodyBytes    = 4096
	organizerCacheControl = "private, no-store"
)

type patchRequest struct {
	Timezone *string `json:"timezone"`
}

type representation struct {
	ID       string  `json:"id"`
	Timezone *string `json:"timezone"`
}

// Handler returns the canonical organizer item handler.
func Handler(applicationContext *config.ApplicationContext) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", organizerCacheControl)
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		organizerID, pathValid := handlers.ResourceIDFromPath(config.WebOrganizers, request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		if organizerID != currentUser.ID {
			writeError(applicationContext, responseWriter, http.StatusForbidden, "organizer_forbidden", "Organizer access is forbidden.", nil)
			return
		}
		switch request.Method {
		case http.MethodGet:
			handleRead(applicationContext, currentUser, responseWriter, request)
		case http.MethodPatch:
			handleUpdate(applicationContext, currentUser, responseWriter, request)
		default:
			responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
		}
	})
}

func handleRead(applicationContext *config.ApplicationContext, currentUser *models.User, responseWriter http.ResponseWriter, request *http.Request) {
	var organizer models.User
	if findError := organizer.FindByID(applicationContext.Database.WithContext(request.Context()), currentUser.ID); findError != nil {
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "organizer_operation_failed", "Organizer operation failed.", findError)
		return
	}
	writeRepresentation(applicationContext, responseWriter, &organizer)
}

func handleUpdate(applicationContext *config.ApplicationContext, currentUser *models.User, responseWriter http.ResponseWriter, request *http.Request) {
	var body patchRequest
	if decodeError := handlers.DecodeJSON(responseWriter, request, organizerBodyBytes, &body); decodeError != nil {
		if errors.Is(decodeError, handlers.ErrUnsupportedJSONMediaType) {
			writeError(applicationContext, responseWriter, http.StatusUnsupportedMediaType, "unsupported_media_type", http.StatusText(http.StatusUnsupportedMediaType), decodeError)
			return
		}
		writeError(applicationContext, responseWriter, http.StatusBadRequest, "malformed_json", "Request JSON is malformed.", decodeError)
		return
	}
	if body.Timezone == nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_organizer_patch", "Organizer patch is empty.", nil)
		return
	}
	timezone, timezoneError := models.NewTimezone(*body.Timezone)
	if timezoneError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_timezone", "Timezone is invalid.", timezoneError)
		return
	}
	if updateError := currentUser.UpdateTimezone(applicationContext.Database.WithContext(request.Context()), timezone); updateError != nil {
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "organizer_operation_failed", "Organizer operation failed.", updateError)
		return
	}
	writeRepresentation(applicationContext, responseWriter, currentUser)
}

func writeRepresentation(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, organizer *models.User) {
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, representation{ID: organizer.ID, Timezone: organizer.Timezone}); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write organizer response: %v", encodeError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write organizer error response: %v", responseError)
	}
}
