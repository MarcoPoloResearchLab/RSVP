// Package middleware provides HTTP middleware functions for the application.
package middleware

import (
	"context"
	"net/http"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

// contextKey is a custom type used for keys in context.Context to avoid collisions.
type contextKey string

// ContextKeyUser is the key used to store the authenticated *models.User in the request context.
const ContextKeyUser contextKey = "user"

// AddUserToContext is middleware that retrieves user information based on the session email,
// performs an Upsert operation (find or create) in the database, and adds the resulting
// *models.User object to the request's context. If the user cannot be determined or upserted
// after successful authentication (which implies a server issue), it stops the request chain
// and returns an error.
func AddUserToContext(applicationContext *config.ApplicationContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			sessionInstance, sessionError := session.Store().Get(request, gconstants.SessionName)
			if sessionError != nil {
				applicationContext.Logger.Printf("ERROR: Session retrieval failed in AddUserToContext for %s: %v", request.URL.Path, sessionError)
				utils.HandleError(responseWriter, sessionError, utils.ServerError, applicationContext.Logger, "Failed to process user session.")
				return
			}

			userEmail, emailOk := sessionInstance.Values[gconstants.SessionKeyUserEmail].(string)
			userName, _ := sessionInstance.Values[gconstants.SessionKeyUserName].(string)
			userPicture, _ := sessionInstance.Values[gconstants.SessionKeyUserPicture].(string)

			if !emailOk || userEmail == "" {
				applicationContext.Logger.Printf("ERROR: User email missing from session after authentication for %s", request.URL.Path)
				utils.HandleError(responseWriter, nil, utils.AuthenticationError, applicationContext.Logger, utils.ErrMsgUnauthorized)
				return
			}

			user, upsertErr := models.UpsertUser(applicationContext.Database, userEmail, userName, userPicture)
			if upsertErr != nil {
				applicationContext.Logger.Printf("ERROR: Failed to upsert user (%s) in AddUserToContext middleware for %s: %v", userEmail, request.URL.Path, upsertErr)
				utils.HandleError(responseWriter, upsertErr, utils.ServerError, applicationContext.Logger, "Failed to retrieve or create user profile.")
				return
			}

			ctx := context.WithValue(request.Context(), ContextKeyUser, user)
			requestWithUser := request.WithContext(ctx)

			next.ServeHTTP(responseWriter, requestWithUser)
		})
	}
}
