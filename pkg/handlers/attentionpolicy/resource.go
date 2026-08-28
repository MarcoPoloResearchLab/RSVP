// Package attentionpolicy provides authenticated attention policy resource handlers.
package attentionpolicy

import (
	"encoding/json"
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

const policyBodyBytes = 4096

type requestBody struct {
	LaneID                    string          `json:"lane_id"`
	ReviewIntervalSeconds     *int64          `json:"review_interval_seconds"`
	NextProbeAt               *string         `json:"next_probe_at"`
	EscalationIntervalSeconds json.RawMessage `json:"escalation_interval_seconds"`
}

type representation struct {
	ID                        string `json:"id"`
	LaneID                    string `json:"lane_id"`
	ReviewIntervalSeconds     int64  `json:"review_interval_seconds"`
	NextProbeAt               string `json:"next_probe_at"`
	EscalationIntervalSeconds *int64 `json:"escalation_interval_seconds"`
}

// Handler returns the canonical attention policy collection and item handler.
func Handler(applicationContext *config.ApplicationContext, now func() time.Time) http.Handler {
	service, serviceError := services.NewAttentionService(applicationContext.Database, now)
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "private, no-store")
		if serviceError != nil {
			writeError(applicationContext, responseWriter, http.StatusInternalServerError, "attention_service_failed", "Attention management is unavailable.", serviceError)
			return
		}
		currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if !userFound || currentUser == nil || currentUser.ID == "" {
			writeError(applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path == config.WebAttentionPolicies {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
				return
			}
			handleCreate(applicationContext, service, currentUser.ID, responseWriter, request)
			return
		}
		policyID, pathValid := handlers.ResourceIDFromPath(config.WebAttentionPolicies, request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		switch request.Method {
		case http.MethodPatch:
			handleUpdate(applicationContext, service, currentUser.ID, policyID, responseWriter, request)
		case http.MethodDelete:
			if deleteError := service.DeletePolicy(request.Context(), currentUser.ID, policyID); deleteError != nil {
				writeServiceError(applicationContext, responseWriter, deleteError)
				return
			}
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			responseWriter.Header().Set("Allow", http.MethodPatch+", "+http.MethodDelete)
			writeError(applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
		}
	})
}

func handleCreate(applicationContext *config.ApplicationContext, service *services.AttentionService, organizerID string, responseWriter http.ResponseWriter, request *http.Request) {
	body, decodeError := decodeRequest(responseWriter, request)
	if decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	if strings.TrimSpace(body.LaneID) == "" || body.ReviewIntervalSeconds == nil || body.NextProbeAt == nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "Attention policy data is invalid.", nil)
		return
	}
	nextProbeAt, parseError := time.Parse(time.RFC3339, *body.NextProbeAt)
	if parseError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "The next probe time is invalid.", parseError)
		return
	}
	escalation, _, escalationError := parseEscalation(body.EscalationIntervalSeconds)
	if escalationError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "The escalation interval is invalid.", escalationError)
		return
	}
	createdPolicy, createError := service.CreatePolicy(request.Context(), organizerID, body.LaneID, time.Duration(*body.ReviewIntervalSeconds)*time.Second, nextProbeAt, escalation)
	if createError != nil {
		writeServiceError(applicationContext, responseWriter, createError)
		return
	}
	responseWriter.Header().Set("Location", config.WebAttentionPolicies+createdPolicy.ID)
	writeRepresentation(applicationContext, responseWriter, http.StatusCreated, createdPolicy)
}

func handleUpdate(applicationContext *config.ApplicationContext, service *services.AttentionService, organizerID string, policyID string, responseWriter http.ResponseWriter, request *http.Request) {
	body, decodeError := decodeRequest(responseWriter, request)
	if decodeError != nil {
		writeDecodeError(applicationContext, responseWriter, decodeError)
		return
	}
	patch := services.AttentionPolicyPatch{}
	if body.ReviewIntervalSeconds != nil {
		interval := time.Duration(*body.ReviewIntervalSeconds) * time.Second
		patch.ReviewInterval = &interval
	}
	if body.NextProbeAt != nil {
		parsedTime, parseError := time.Parse(time.RFC3339, *body.NextProbeAt)
		if parseError != nil {
			writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "The next probe time is invalid.", parseError)
			return
		}
		patch.NextProbeAt = &parsedTime
	}
	escalation, escalationPresent, escalationError := parseEscalation(body.EscalationIntervalSeconds)
	if escalationError != nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "The escalation interval is invalid.", escalationError)
		return
	}
	if escalationPresent {
		patch.EscalationInterval = &escalation
	}
	if patch.ReviewInterval == nil && patch.NextProbeAt == nil && patch.EscalationInterval == nil {
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "Attention policy patch is empty.", nil)
		return
	}
	updatedPolicy, updateError := service.UpdatePolicy(request.Context(), organizerID, policyID, patch)
	if updateError != nil {
		writeServiceError(applicationContext, responseWriter, updateError)
		return
	}
	writeRepresentation(applicationContext, responseWriter, http.StatusOK, updatedPolicy)
}

func decodeRequest(responseWriter http.ResponseWriter, request *http.Request) (requestBody, error) {
	var body requestBody
	decodeError := handlers.DecodeJSON(responseWriter, request, policyBodyBytes, &body)
	return body, decodeError
}

func parseEscalation(rawValue json.RawMessage) (*time.Duration, bool, error) {
	if len(rawValue) == 0 {
		return nil, false, nil
	}
	if string(rawValue) == "null" {
		return nil, true, nil
	}
	var seconds int64
	if unmarshalError := json.Unmarshal(rawValue, &seconds); unmarshalError != nil || seconds <= 0 {
		return nil, true, models.ErrEscalationIntervalInvalid
	}
	duration := time.Duration(seconds) * time.Second
	return &duration, true, nil
}

func writeRepresentation(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, policy *models.AttentionPolicy) {
	value := representation{ID: policy.ID, LaneID: policy.LaneID, ReviewIntervalSeconds: policy.ReviewIntervalSeconds, NextProbeAt: policy.NextProbeAt.UTC().Format(time.RFC3339Nano), EscalationIntervalSeconds: policy.EscalationIntervalSeconds}
	if encodeError := handlers.WriteJSON(responseWriter, statusCode, value); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write attention policy response: %v", encodeError)
	}
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
		writeError(applicationContext, responseWriter, http.StatusForbidden, "attention_policy_forbidden", "Attention policy access is forbidden.", serviceError)
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(applicationContext, responseWriter, http.StatusNotFound, "attention_policy_not_found", "Attention policy was not found.", serviceError)
	case errors.Is(serviceError, services.ErrAttentionLaneInvalid), errors.Is(serviceError, models.ErrReviewIntervalInvalid), errors.Is(serviceError, models.ErrNextProbeRequired), errors.Is(serviceError, models.ErrEscalationIntervalInvalid):
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_attention_policy", "Attention policy data is invalid.", serviceError)
	default:
		writeError(applicationContext, responseWriter, http.StatusInternalServerError, "attention_policy_operation_failed", "Attention policy operation failed.", serviceError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write attention policy error response: %v", responseError)
	}
}
