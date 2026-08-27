// Package ingestiondraft provides quick ingestion draft resources.
package ingestiondraft

import (
	"errors"
	"fmt"
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

const maximumBodyBytes = 8192

type Resources struct {
	applicationContext *config.ApplicationContext
	service            *services.IngestionDraftService
}

func New(applicationContext *config.ApplicationContext, now func() time.Time) (*Resources, error) {
	service, serviceError := services.NewIngestionDraftService(applicationContext.Database, now)
	if serviceError != nil {
		return nil, serviceError
	}
	return &Resources{applicationContext: applicationContext, service: service}, nil
}

type draftRequest struct {
	Mode                      models.IngestionDraftMode `json:"mode"`
	CalendarID                string                    `json:"calendar_id"`
	Title                     string                    `json:"title"`
	AnchorEventID             *string                   `json:"anchor_event_id"`
	StartsAt                  *string                   `json:"starts_at"`
	EndsAt                    *string                   `json:"ends_at"`
	ReviewIntervalSeconds     *int64                    `json:"review_interval_seconds"`
	NextProbeAt               *string                   `json:"next_probe_at"`
	EscalationIntervalSeconds *int64                    `json:"escalation_interval_seconds"`
	ReferenceTime             string                    `json:"reference_time"`
	Timezone                  string                    `json:"timezone"`
}

func (body draftRequest) proposal() (models.IngestionDraftProposal, error) {
	timezone, timezoneError := models.NewTimezone(body.Timezone)
	if timezoneError != nil {
		return models.IngestionDraftProposal{}, timezoneError
	}
	startsAt, startsError := parseDraftTime(body.StartsAt, timezone)
	if startsError != nil {
		return models.IngestionDraftProposal{}, startsError
	}
	endsAt, endsError := parseDraftTime(body.EndsAt, timezone)
	if endsError != nil {
		return models.IngestionDraftProposal{}, endsError
	}
	nextProbeAt, probeError := parseDraftTime(body.NextProbeAt, timezone)
	if probeError != nil {
		return models.IngestionDraftProposal{}, probeError
	}
	referenceTime, referenceError := time.Parse(time.RFC3339Nano, body.ReferenceTime)
	if referenceError != nil {
		return models.IngestionDraftProposal{}, fmt.Errorf("%w: reference time: %v", models.ErrIngestionDraftInvalid, referenceError)
	}
	return models.IngestionDraftProposal{Mode: body.Mode, CalendarID: body.CalendarID, Title: body.Title, AnchorEventID: body.AnchorEventID, StartsAt: startsAt, EndsAt: endsAt, ReviewIntervalSeconds: body.ReviewIntervalSeconds, NextProbeAt: nextProbeAt, EscalationIntervalSeconds: body.EscalationIntervalSeconds, ReferenceTime: referenceTime, Timezone: body.Timezone}, nil
}

func parseDraftTime(value *string, timezone models.Timezone) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if instant, parseError := time.Parse(time.RFC3339Nano, *value); parseError == nil {
		canonical := instant.UTC()
		return &canonical, nil
	}
	instant, parseError := models.ParseLocalTime(*value, "2006-01-02T15:04", timezone)
	if parseError != nil {
		return nil, fmt.Errorf("%w: local time: %v", models.ErrIngestionDraftInvalid, parseError)
	}
	return &instant, nil
}

func (resources *Resources) Handler() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "private, no-store")
		organizer, found := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !found || organizer == nil || organizer.ID == "" {
			writeError(responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
			return
		}
		if request.URL.Path == config.WebIngestionDrafts {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
				return
			}
			var body draftRequest
			if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
				writeDecodeError(responseWriter, decodeError)
				return
			}
			proposal, proposalError := body.proposal()
			if proposalError != nil {
				resources.writeServiceError(responseWriter, proposalError)
				return
			}
			draft, createError := resources.service.Create(request.Context(), organizer.ID, proposal)
			if createError != nil {
				resources.writeServiceError(responseWriter, createError)
				return
			}
			responseWriter.Header().Set("Location", config.WebIngestionDrafts+draft.ID)
			_ = handlers.WriteJSON(responseWriter, http.StatusCreated, draft)
			return
		}
		remainder := strings.TrimPrefix(request.URL.Path, config.WebIngestionDrafts)
		confirmationCollection := strings.HasSuffix(remainder, "/confirmations/")
		draftID := strings.TrimSuffix(remainder, "/confirmations/")
		if !confirmationCollection {
			draftID = strings.Trim(remainder, "/")
		}
		if draftID == "" || strings.Contains(draftID, "/") {
			http.NotFound(responseWriter, request)
			return
		}
		if confirmationCollection {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
				return
			}
			var body struct{}
			if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
				writeDecodeError(responseWriter, decodeError)
				return
			}
			confirmation, reused, confirmError := resources.service.Confirm(request.Context(), organizer.ID, draftID, request.Header.Get(config.IdempotencyKeyHeader))
			if confirmError != nil {
				resources.writeServiceError(responseWriter, confirmError)
				return
			}
			status := http.StatusCreated
			if reused {
				status = http.StatusOK
			}
			responseWriter.Header().Set("Location", config.WebIngestionDrafts+draftID+"/confirmations/"+confirmation.ID)
			_ = handlers.WriteJSON(responseWriter, status, confirmation)
			return
		}
		switch request.Method {
		case http.MethodGet:
			draft, readError := resources.service.Read(request.Context(), organizer.ID, draftID)
			if readError != nil {
				resources.writeServiceError(responseWriter, readError)
				return
			}
			_ = handlers.WriteJSON(responseWriter, http.StatusOK, draft)
		case http.MethodPatch:
			var body draftRequest
			if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
				writeDecodeError(responseWriter, decodeError)
				return
			}
			proposal, proposalError := body.proposal()
			if proposalError != nil {
				resources.writeServiceError(responseWriter, proposalError)
				return
			}
			draft, updateError := resources.service.Update(request.Context(), organizer.ID, draftID, proposal)
			if updateError != nil {
				resources.writeServiceError(responseWriter, updateError)
				return
			}
			_ = handlers.WriteJSON(responseWriter, http.StatusOK, draft)
		case http.MethodDelete:
			if _, cancelError := resources.service.Cancel(request.Context(), organizer.ID, draftID); cancelError != nil {
				resources.writeServiceError(responseWriter, cancelError)
				return
			}
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch+", "+http.MethodDelete)
			writeError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		}
	})
}

func (resources *Resources) writeServiceError(responseWriter http.ResponseWriter, serviceError error) {
	switch {
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(responseWriter, http.StatusNotFound, "ingestion_draft_not_found", "Ingestion draft was not found.")
	case errors.Is(serviceError, services.ErrResourceForbidden):
		writeError(responseWriter, http.StatusForbidden, "resource_forbidden", "The ingestion draft belongs to another organizer.")
	case errors.Is(serviceError, services.ErrIdempotencyKeyRequired):
		writeError(responseWriter, http.StatusBadRequest, "idempotency_key_required", serviceError.Error())
	case errors.Is(serviceError, services.ErrIdempotencyConflict):
		writeError(responseWriter, http.StatusConflict, "idempotency_conflict", serviceError.Error())
	case errors.Is(serviceError, services.ErrIngestionDraftNotReady), errors.Is(serviceError, models.ErrIngestionDraftInvalid), errors.Is(serviceError, models.ErrEventMembershipInvalid), errors.Is(serviceError, models.ErrTimezoneInvalid):
		writeError(responseWriter, http.StatusUnprocessableEntity, "ingestion_draft_invalid", serviceError.Error())
	default:
		resources.applicationContext.Logger.Printf("ERROR: Ingestion draft operation: %v", serviceError)
		writeError(responseWriter, http.StatusInternalServerError, "ingestion_draft_failed", "The ingestion draft operation failed.")
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
