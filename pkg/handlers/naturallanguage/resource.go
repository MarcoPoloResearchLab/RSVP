// Package naturallanguage provides natural-language ingestion resources.
package naturallanguage

import (
	"errors"
	"net/http"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
)

const maximumBodyBytes = 8192

type Resource struct {
	applicationContext *config.ApplicationContext
	service            *services.NaturalLanguageService
}

func New(applicationContext *config.ApplicationContext, parser services.NaturalLanguageParser) (*Resource, error) {
	service, serviceError := services.NewNaturalLanguageService(applicationContext.Database, parser)
	if serviceError != nil {
		return nil, serviceError
	}
	return &Resource{applicationContext: applicationContext, service: service}, nil
}

func (resource *Resource) Handler() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "private, no-store")
		if request.Method != http.MethodPost || request.URL.Path != config.WebNaturalLanguageIngestion {
			responseWriter.Header().Set("Allow", http.MethodPost)
			_ = handlers.WriteTypedError(responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
			return
		}
		organizer, found := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !found || organizer == nil || organizer.ID == "" {
			_ = handlers.WriteTypedError(responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
			return
		}
		var body struct {
			InputText     string `json:"input_text"`
			CalendarID    string `json:"calendar_id"`
			ReferenceTime string `json:"reference_time"`
			Timezone      string `json:"timezone"`
		}
		if decodeError := handlers.DecodeJSON(responseWriter, request, maximumBodyBytes, &body); decodeError != nil {
			_ = handlers.WriteTypedError(responseWriter, http.StatusBadRequest, "invalid_request", decodeError.Error())
			return
		}
		referenceTime, referenceError := time.Parse(time.RFC3339Nano, body.ReferenceTime)
		timezone, timezoneError := models.NewTimezone(body.Timezone)
		if referenceError != nil || timezoneError != nil {
			_ = handlers.WriteTypedError(responseWriter, http.StatusUnprocessableEntity, "natural_language_input_invalid", "Reference time and organizer timezone must be valid.")
			return
		}
		draft, parseError := resource.service.Parse(request.Context(), organizer.ID, body.InputText, body.CalendarID, referenceTime, timezone)
		if parseError != nil {
			resource.writeError(responseWriter, parseError)
			return
		}
		responseWriter.Header().Set("Location", config.WebIngestionDrafts+draft.ID)
		_ = handlers.WriteJSON(responseWriter, http.StatusCreated, draft)
	})
}

func (resource *Resource) writeError(responseWriter http.ResponseWriter, parseError error) {
	switch {
	case errors.Is(parseError, services.ErrNaturalLanguageInputRequired), errors.Is(parseError, services.ErrNaturalLanguageProviderResponseInvalid), errors.Is(parseError, models.ErrIngestionDraftInvalid):
		_ = handlers.WriteTypedError(responseWriter, http.StatusUnprocessableEntity, "natural_language_input_invalid", parseError.Error())
	case errors.Is(parseError, services.ErrResourceForbidden):
		_ = handlers.WriteTypedError(responseWriter, http.StatusForbidden, "resource_forbidden", "The selected calendar or anchor belongs to another organizer.")
	default:
		resource.applicationContext.Logger.Printf("ERROR: Natural-language parser operation failed: %v", parseError)
		_ = handlers.WriteTypedError(responseWriter, http.StatusBadGateway, "natural_language_provider_failed", "Natural-language parsing failed.")
	}
}
