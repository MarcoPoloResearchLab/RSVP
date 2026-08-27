// Package lane provides authenticated lane resource handlers.
package lane

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
	laneCacheControl = "private, no-store"
	laneBodyBytes    = 4096
	laneKindOpen     = "open"
	laneKindFinite   = "finite"
)

type createRequest struct {
	CalendarID string  `json:"calendar_id"`
	Title      string  `json:"title"`
	Kind       string  `json:"kind"`
	EndsAt     *string `json:"ends_at"`
	Timezone   string  `json:"timezone"`
}

type patchRequest struct {
	Title        *string `json:"title"`
	CalendarID   *string `json:"calendar_id"`
	DisplayOrder *int    `json:"display_order"`
	EndsAt       *string `json:"ends_at"`
	ResolvedAt   *string `json:"resolved_at"`
}

type representation struct {
	ID           string            `json:"id"`
	CalendarID   string            `json:"calendar_id"`
	Title        string            `json:"title"`
	Status       models.LaneStatus `json:"status"`
	StartsAt     string            `json:"starts_at"`
	EndsAt       *string           `json:"ends_at"`
	ResolvedAt   *string           `json:"resolved_at"`
	DisplayOrder int               `json:"display_order"`
	EventIDs     []string          `json:"event_ids"`
}

// Handler returns the canonical lane collection and item handler.
func Handler(applicationContext *config.ApplicationContext, now func() time.Time) http.Handler {
	service, serviceError := services.NewTemporalManagementService(applicationContext.Database, now)
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", laneCacheControl)
		if serviceError != nil {
			writeError(applicationContext, responseWriter, http.StatusInternalServerError, "lane_service_failed", "Lane management is unavailable.", serviceError)
			return
		}
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path == config.WebLanes {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
				return
			}
			handleCreate(applicationContext, service, currentUser, responseWriter, request)
			return
		}
		laneID, pathValid := handlers.ResourceIDFromPath(config.WebLanes, request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		switch request.Method {
		case http.MethodGet:
			handleRead(applicationContext, service, currentUser.ID, laneID, responseWriter, request)
		case http.MethodPatch:
			handleUpdate(applicationContext, service, currentUser.ID, laneID, responseWriter, request)
		case http.MethodDelete:
			handleDelete(applicationContext, service, currentUser.ID, laneID, responseWriter, request)
		default:
			responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch+", "+http.MethodDelete)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
		}
	})
}

func handleCreate(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, currentUser *models.User, responseWriter http.ResponseWriter, request *http.Request) {
	var body createRequest
	if decodeError := handlers.DecodeJSON(responseWriter, request, laneBodyBytes, &body); decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	if strings.TrimSpace(body.CalendarID) == "" || strings.TrimSpace(body.Title) == "" || (body.Kind != laneKindOpen && body.Kind != laneKindFinite) {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane", "Lane data is invalid.", nil)
		return
	}
	timezone, timezoneError := models.NewTimezone(body.Timezone)
	if timezoneError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_timezone", "Timezone is invalid.", timezoneError)
		return
	}
	var endsAt *time.Time
	if body.Kind == laneKindFinite {
		if body.EndsAt == nil {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane", "A finite lane requires an end time.", nil)
			return
		}
		parsedEnd, parseError := time.Parse(time.RFC3339, *body.EndsAt)
		if parseError != nil {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane", "Lane end time is invalid.", parseError)
			return
		}
		endsAt = &parsedEnd
	} else if body.EndsAt != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane", "An open lane has no end time.", nil)
		return
	}
	createdLane, createError := service.CreateLane(request.Context(), currentUser, timezone, body.CalendarID, body.Title, endsAt)
	if createError != nil {
		writeServiceError(applicationContext, responseWriter, createError)
		return
	}
	responseWriter.Header().Set("Location", config.WebLanes+createdLane.ID)
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusCreated, laneRepresentation(createdLane)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write lane creation response: %v", encodeError)
	}
}

func handleRead(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, laneID string, responseWriter http.ResponseWriter, request *http.Request) {
	storedLane, readError := service.ReadLane(request.Context(), organizerID, laneID)
	if readError != nil {
		writeServiceError(applicationContext, responseWriter, readError)
		return
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, laneRepresentation(storedLane)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write lane response: %v", encodeError)
	}
}

func handleUpdate(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, laneID string, responseWriter http.ResponseWriter, request *http.Request) {
	var body patchRequest
	if decodeError := handlers.DecodeJSON(responseWriter, request, laneBodyBytes, &body); decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	if body.Title == nil && body.CalendarID == nil && body.DisplayOrder == nil && body.EndsAt == nil && body.ResolvedAt == nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane_patch", "Lane patch is empty.", nil)
		return
	}
	if (body.Title != nil && strings.TrimSpace(*body.Title) == "") || (body.CalendarID != nil && strings.TrimSpace(*body.CalendarID) == "") ||
		(body.DisplayOrder != nil && *body.DisplayOrder < 0) || (body.EndsAt != nil && body.ResolvedAt != nil) {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane_patch", "Lane patch is invalid.", nil)
		return
	}
	var endsAt *time.Time
	if body.EndsAt != nil {
		parsedEnd, parseError := time.Parse(time.RFC3339, *body.EndsAt)
		if parseError != nil {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane_patch", "Lane end time is invalid.", parseError)
			return
		}
		endsAt = &parsedEnd
	}
	var resolutionTime *time.Time
	if body.ResolvedAt != nil {
		parsedResolution, parseError := time.Parse(time.RFC3339, *body.ResolvedAt)
		if parseError != nil {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane_patch", "Lane resolution time is invalid.", parseError)
			return
		}
		resolutionTime = &parsedResolution
	}
	updatedLane, updateError := service.UpdateLane(request.Context(), organizerID, laneID, services.LanePatch{
		Title: body.Title, CalendarID: body.CalendarID, DisplayOrder: body.DisplayOrder, EndsAt: endsAt, ResolutionTime: resolutionTime,
	})
	if updateError != nil {
		writeServiceError(applicationContext, responseWriter, updateError)
		return
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, laneRepresentation(updatedLane)); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write lane update response: %v", encodeError)
	}
}

func handleDelete(applicationContext *config.ApplicationContext, service *services.TemporalManagementService, organizerID string, laneID string, responseWriter http.ResponseWriter, request *http.Request) {
	if deleteError := service.DeleteLane(request.Context(), organizerID, laneID); deleteError != nil {
		writeServiceError(applicationContext, responseWriter, deleteError)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func laneRepresentation(laneRecord *models.Lane) representation {
	representationValue := representation{
		ID: laneRecord.ID, CalendarID: laneRecord.CalendarID, Title: laneRecord.Title, Status: laneRecord.Status,
		StartsAt: laneRecord.StartsAt.UTC().Format(time.RFC3339Nano), DisplayOrder: laneRecord.DisplayOrder, EventIDs: make([]string, 0, len(laneRecord.Events)),
	}
	if laneRecord.EndsAt != nil {
		formattedEnd := laneRecord.EndsAt.UTC().Format(time.RFC3339Nano)
		representationValue.EndsAt = &formattedEnd
	}
	if laneRecord.ResolvedAt != nil {
		formattedResolution := laneRecord.ResolvedAt.UTC().Format(time.RFC3339Nano)
		representationValue.ResolvedAt = &formattedResolution
	}
	for _, event := range laneRecord.Events {
		representationValue.EventIDs = append(representationValue.EventIDs, event.ID)
	}
	return representationValue
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
		writeError(applicationContext, responseWriter, http.StatusForbidden, "lane_forbidden", "Lane access is forbidden.", serviceError)
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(applicationContext, responseWriter, http.StatusNotFound, "lane_not_found", "Lane was not found.", serviceError)
	case errors.Is(serviceError, services.ErrLaneHasRSVPs):
		writeError(applicationContext, responseWriter, http.StatusConflict, "lane_has_rsvps", "An RSVP prevents lane deletion.", serviceError)
	case errors.Is(serviceError, services.ErrSourceOwnedLane):
		writeError(applicationContext, responseWriter, http.StatusConflict, "source_owned_lane", "Calendar synchronization owns this lane.", serviceError)
	case errors.Is(serviceError, services.ErrDisplayOrderOutOfRange), errors.Is(serviceError, services.ErrLaneResolutionInvalid),
		errors.Is(serviceError, services.ErrLaneEndUpdateInvalid), errors.Is(serviceError, models.ErrLaneTitleRequired),
		errors.Is(serviceError, models.ErrLaneEndInvalid), errors.Is(serviceError, models.ErrLaneStateInvalid),
		errors.Is(serviceError, models.ErrMarkerOutsideLane), errors.Is(serviceError, models.ErrDisplayOrderInvalid),
		errors.Is(serviceError, models.ErrTimezoneInvalid), errors.Is(serviceError, models.ErrTimezoneRequired):
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_lane", "Lane data is invalid.", serviceError)
	default:
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "lane_operation_failed", "Lane operation failed.", serviceError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write lane error response: %v", responseError)
	}
}
