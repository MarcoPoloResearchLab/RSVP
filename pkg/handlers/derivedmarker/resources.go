// Package derivedmarker provides derived marker rule resources.
package derivedmarker

import (
	"errors"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

const maximumBodyBytes = 4096

type Resources struct {
	applicationContext *config.ApplicationContext
	service            *services.DerivedMarkerService
}

func New(applicationContext *config.ApplicationContext) (*Resources, error) {
	service, serviceError := services.NewDerivedMarkerService(applicationContext.Database)
	if serviceError != nil {
		return nil, serviceError
	}
	return &Resources{applicationContext: applicationContext, service: service}, nil
}

type ruleRequest struct {
	AnchorType    models.DerivedAnchorType `json:"anchor_type"`
	AnchorID      string                   `json:"anchor_id"`
	AnchorEdge    models.DerivedAnchorEdge `json:"anchor_edge"`
	OffsetSeconds int64                    `json:"offset_seconds"`
}

func (resources *Resources) Handler() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "private, no-store")
		organizer, found := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !found || organizer == nil || organizer.ID == "" {
			writeError(responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
			return
		}
		if request.URL.Path == config.WebDerivedMarkerRules {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
				return
			}
			var body ruleRequest
			if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
				writeDecodeError(responseWriter, decodeError)
				return
			}
			rule, marker, createError := resources.service.Create(request.Context(), organizer.ID, body.AnchorType, body.AnchorID, body.AnchorEdge, body.OffsetSeconds)
			if createError != nil {
				resources.writeServiceError(responseWriter, createError)
				return
			}
			responseWriter.Header().Set("Location", config.WebDerivedMarkerRules+rule.ID)
			_ = handlers.WriteJSON(responseWriter, http.StatusCreated, map[string]any{"rule": rule, "marker": marker})
			return
		}
		ruleID, valid := handlers.ResourceIDFromPath(config.WebDerivedMarkerRules, request.URL.Path)
		if !valid {
			http.NotFound(responseWriter, request)
			return
		}
		switch request.Method {
		case http.MethodPatch:
			var body ruleRequest
			if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
				writeDecodeError(responseWriter, decodeError)
				return
			}
			rule, marker, updateError := resources.service.Update(request.Context(), organizer.ID, ruleID, body.AnchorType, body.AnchorID, body.AnchorEdge, body.OffsetSeconds)
			if updateError != nil {
				resources.writeServiceError(responseWriter, updateError)
				return
			}
			_ = handlers.WriteJSON(responseWriter, http.StatusOK, map[string]any{"rule": rule, "marker": marker})
		case http.MethodDelete:
			if deleteError := resources.service.Delete(request.Context(), organizer.ID, ruleID); deleteError != nil {
				resources.writeServiceError(responseWriter, deleteError)
				return
			}
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			responseWriter.Header().Set("Allow", http.MethodPatch+", "+http.MethodDelete)
			writeError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		}
	})
}

func (resources *Resources) writeServiceError(responseWriter http.ResponseWriter, serviceError error) {
	switch {
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(responseWriter, http.StatusNotFound, "derived_marker_not_found", "Derived marker rule was not found.")
	case errors.Is(serviceError, services.ErrResourceForbidden):
		writeError(responseWriter, http.StatusForbidden, "resource_forbidden", "The derived marker belongs to another organizer.")
	case errors.Is(serviceError, services.ErrDerivedMarkerCycle), errors.Is(serviceError, services.ErrDerivedAnchorAllDay), errors.Is(serviceError, models.ErrDerivedMarkerRuleInvalid), errors.Is(serviceError, models.ErrEventMembershipInvalid):
		writeError(responseWriter, http.StatusUnprocessableEntity, "derived_marker_invalid", serviceError.Error())
	default:
		resources.applicationContext.Logger.Printf("ERROR: Derived marker operation: %v", serviceError)
		writeError(responseWriter, http.StatusInternalServerError, "derived_marker_failed", "The derived marker operation failed.")
	}
}

func writeDecodeError(responseWriter http.ResponseWriter, decodeError error) {
	status := http.StatusBadRequest
	if errors.Is(decodeError, handlers.ErrUnsupportedJSONMediaType) {
		status = http.StatusUnsupportedMediaType
	}
	writeError(responseWriter, status, "invalid_request", decodeError.Error())
}

func writeError(responseWriter http.ResponseWriter, status int, code string, message string) {
	_ = handlers.WriteTypedError(responseWriter, status, code, message)
}
