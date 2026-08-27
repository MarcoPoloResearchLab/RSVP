// Package calendar provides authenticated calendar HTTP handlers.
package calendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"gorm.io/gorm"
)

const (
	calendarCacheControl        = "private, no-store"
	invalidVisibilityCode       = "invalid_calendar_visibility"
	invalidVisibilityMessage    = "Calendar visibility is invalid."
	calendarNotFoundCode        = "calendar_not_found"
	calendarNotFoundMessage     = "Calendar was not found."
	calendarUpdateErrorCode     = "calendar_update_failed"
	calendarUpdateErrorMessage  = "Calendar visibility could not be updated."
	calendarVisibilityBodyBytes = 1024
)

type visibilityPatch struct {
	Visible *bool `json:"visible"`
}

// VisibilityHandler updates one organizer-owned calendar visibility value.
func VisibilityHandler(applicationContext *config.ApplicationContext) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", calendarCacheControl)
		calendarID, pathValid := calendarIDFromPath(request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodPatch {
			responseWriter.Header().Set("Allow", http.MethodPatch)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			return
		}
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		visible, decodeError := decodeVisibility(responseWriter, request)
		if decodeError != nil {
			writeError(applicationContext, responseWriter, http.StatusBadRequest, invalidVisibilityCode, invalidVisibilityMessage, decodeError)
			return
		}
		updateError := applicationContext.Database.WithContext(request.Context()).Transaction(func(transaction *gorm.DB) error {
			var storedCalendar models.Calendar
			if findError := transaction.Where("id = ? AND organizer_id = ?", calendarID, currentUser.ID).First(&storedCalendar).Error; findError != nil {
				return findError
			}
			storedCalendar.Visible = visible
			return transaction.Save(&storedCalendar).Error
		})
		if errors.Is(updateError, gorm.ErrRecordNotFound) {
			writeError(applicationContext, responseWriter, http.StatusNotFound, calendarNotFoundCode, calendarNotFoundMessage, updateError)
			return
		}
		if updateError != nil {
			writeError(applicationContext, responseWriter, http.StatusInternalServerError, calendarUpdateErrorCode, calendarUpdateErrorMessage, updateError)
			return
		}
		responseWriter.WriteHeader(http.StatusNoContent)
	})
}

func calendarIDFromPath(requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, config.WebCalendars) {
		return "", false
	}
	calendarID := strings.TrimPrefix(requestPath, config.WebCalendars)
	return calendarID, calendarID != "" && !strings.Contains(calendarID, "/")
}

func decodeVisibility(responseWriter http.ResponseWriter, request *http.Request) (bool, error) {
	mediaType, _, contentTypeError := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if contentTypeError != nil || mediaType != handlers.JSONMediaType {
		return false, fmt.Errorf("content type must be %s", handlers.JSONMediaType)
	}
	request.Body = http.MaxBytesReader(responseWriter, request.Body, calendarVisibilityBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var patch visibilityPatch
	if decodeError := decoder.Decode(&patch); decodeError != nil {
		return false, decodeError
	}
	if patch.Visible == nil {
		return false, errors.New("visible is required")
	}
	if trailingError := decoder.Decode(&struct{}{}); !errors.Is(trailingError, io.EOF) {
		if trailingError == nil {
			return false, errors.New("request body has trailing content")
		}
		return false, trailingError
	}
	return *patch.Visible, nil
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar visibility response: %v", responseError)
	}
}
