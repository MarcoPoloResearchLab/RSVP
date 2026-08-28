// Package probe provides authenticated probe resource handlers.
package probe

import (
	"errors"
	"net/http"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

type patchRequest struct {
	State models.ProbeState `json:"state"`
}

type representation struct {
	ID          string            `json:"id"`
	PolicyID    string            `json:"policy_id"`
	LaneID      string            `json:"lane_id"`
	DueAt       string            `json:"due_at"`
	EscalatesAt *string           `json:"escalates_at"`
	State       models.ProbeState `json:"state"`
	CompletedAt *string           `json:"completed_at"`
}

// Handler returns the canonical probe item handler.
func Handler(applicationContext *config.ApplicationContext, now func() time.Time) http.Handler {
	service, serviceError := services.NewAttentionService(applicationContext.Database, now)
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "private, no-store")
		if serviceError != nil {
			writeError(applicationContext, responseWriter, http.StatusInternalServerError, "probe_service_failed", "Probe management is unavailable.", serviceError)
			return
		}
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		probeID, pathValid := handlers.ResourceIDFromPath(config.WebProbes, request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodPatch {
			responseWriter.Header().Set("Allow", http.MethodPatch)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			return
		}
		var body patchRequest
		if decodeError := handlers.DecodeJSON(responseWriter, request, 1024, &body); decodeError != nil {
			statusCode := http.StatusBadRequest
			if errors.Is(decodeError, handlers.ErrUnsupportedJSONMediaType) {
				statusCode = http.StatusUnsupportedMediaType
			}
			writeError(applicationContext, responseWriter, statusCode, "invalid_probe_request", "Probe request data is invalid.", decodeError)
			return
		}
		if body.State != models.ProbeStateCompleted {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_probe_transition", "Probe transition is invalid.", nil)
			return
		}
		completedProbe, completeError := service.CompleteProbe(request.Context(), currentUser.ID, probeID)
		if completeError != nil {
			writeServiceError(applicationContext, responseWriter, completeError)
			return
		}
		value := probeRepresentation(completedProbe)
		if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, value); encodeError != nil {
			applicationContext.Logger.Printf("ERROR: Write probe response: %v", encodeError)
		}
	})
}

func probeRepresentation(probe *models.Probe) representation {
	value := representation{ID: probe.ID, PolicyID: probe.PolicyID, LaneID: probe.LaneID, DueAt: probe.DueAt.UTC().Format(time.RFC3339Nano), State: probe.State}
	if probe.EscalatesAt != nil {
		formatted := probe.EscalatesAt.UTC().Format(time.RFC3339Nano)
		value.EscalatesAt = &formatted
	}
	if probe.CompletedAt != nil {
		formatted := probe.CompletedAt.UTC().Format(time.RFC3339Nano)
		value.CompletedAt = &formatted
	}
	return value
}

func writeServiceError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, serviceError error) {
	switch {
	case errors.Is(serviceError, services.ErrResourceForbidden):
		writeError(applicationContext, responseWriter, http.StatusForbidden, "probe_forbidden", "Probe access is forbidden.", serviceError)
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(applicationContext, responseWriter, http.StatusNotFound, "probe_not_found", "Probe was not found.", serviceError)
	case errors.Is(serviceError, services.ErrProbeTransitionInvalid):
		writeError(applicationContext, responseWriter, http.StatusConflict, "probe_transition_conflict", "Probe transition conflicts with its current state.", serviceError)
	default:
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "probe_operation_failed", "Probe operation failed.", serviceError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write probe error response: %v", responseError)
	}
}
