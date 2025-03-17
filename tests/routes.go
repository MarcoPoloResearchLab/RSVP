package tests

import (
	"net/http"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers/event"
	"github.com/temirov/RSVP/pkg/handlers/rsvp"
	// Use the constants from the tests package
)

// Routes holds the application context for integration routes
type Routes struct {
	ApplicationContext *config.ApplicationContext
}

// New creates a new Routes instance for integration testing
func New(applicationContext *config.ApplicationContext) *Routes {
	return &Routes{
		ApplicationContext: applicationContext,
	}
}

// TestAuthMiddleware is a middleware that sets up a test session for integration testing
func TestAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a test session using the shared test user data
		sess, _ := session.Store().Get(r, gconstants.SessionName)
		sess.Values[gconstants.SessionKeyUserEmail] = DefaultTestUser.Email
		sess.Values[gconstants.SessionKeyUserName] = DefaultTestUser.Name
		sess.Values[gconstants.SessionKeyUserPicture] = DefaultTestUser.Picture
		sess.Save(r, w)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes registers all routes without authentication middleware
func (routes *Routes) RegisterRoutes(mux *http.ServeMux) {
	// Register event routes with test auth middleware
	mux.Handle(config.WebEvents, TestAuthMiddleware(event.EventRouter(routes.ApplicationContext)))

	// Register RSVP routes with test auth middleware
	mux.Handle(config.WebRSVPs, TestAuthMiddleware(rsvp.RSVPRouter(routes.ApplicationContext)))

	// Add any other routes needed for testing
}
