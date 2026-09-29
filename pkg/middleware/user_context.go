// Package middleware authorizes RSVP resources with the published TAuth validator.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/tauth/pkg/sessionvalidator"
)

type contextKey string

// ContextKeyUser identifies the authorized organizer in a resource request.
const ContextKeyUser contextKey = "user"

// ResourceAuthorization validates the exact TAuth cookie and tenant before an organizer lookup.
func ResourceAuthorization(application *config.ApplicationContext, validator *sessionvalidator.Validator, tenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			claims, err := validator.ValidateRequest(request)
			if err != nil || claims.GetTenantID() != tenantID || claims.GetUserID() == "" || claims.GetUserEmail() == "" || claims.GetExpiresAt().IsZero() {
				writeAuthorizationError(writer, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
				return
			}
			// The verified email preserves the existing organizer identity and resource ownership.
			organizer, err := models.UpsertUser(application.Database, claims.GetUserEmail(), claims.GetUserDisplayName(), claims.GetUserAvatarURL())
			if err != nil {
				application.Logger.Printf("Authorize organizer resource: database operation failed")
				writeAuthorizationError(writer, http.StatusInternalServerError, "organizer_unavailable", "The organizer is unavailable.")
				return
			}
			next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), ContextKeyUser, organizer)))
		})
	}
}

func writeAuthorizationError(writer http.ResponseWriter, status int, code string, message string) {
	identity := make([]byte, 16)
	if _, err := rand.Read(identity); err != nil {
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"error": map[string]any{"code": code, "message": message, "details": map[string]string{}, "request_id": hex.EncodeToString(identity)}})
}
