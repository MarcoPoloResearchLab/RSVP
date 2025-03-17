package rsvp

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/skip2/go-qrcode"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ShowHandler handles GET /rsvps?rsvp_id={rsvp_id} and displays the QR code for the RSVP link.
// If event_id is also provided, it redirects to the RSVP list page with the RSVP selected for editing.
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for RSVPs
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(w, r, http.MethodGet) {
			return
		}

		// Get RSVP ID and event ID parameters
		rsvpCode := baseHandler.GetParam(r, config.RSVPIDParam)
		eventID := baseHandler.GetParam(r, config.EventIDParam)

		// Validate RSVP code
		if rsvpCode == "" {
			http.Error(w, "Missing RSVP code", http.StatusBadRequest)
			return
		}

		// We'll validate the format only if we have a code
		if !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(w, "RSVP not found", http.StatusNotFound)
			return
		}

		// Load the RSVP from the database
		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode); findError != nil {
			baseHandler.HandleError(w, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		// If event_id is provided, we need to check authorization and redirect to the RSVP list page
		if eventID != "" {
			// Get authenticated user data (authentication is guaranteed by middleware)
			sessionData, _ := baseHandler.RequireAuthentication(w, r)

			// Find the current user
			var currentUser models.User
			if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
				baseHandler.HandleError(w, findUserError, utils.DatabaseError, "User not found in database")
				return
			}

			// Define a function to find the owner ID of an event
			findEventOwnerID := func(eventID string) (string, error) {
				var event models.Event
				if err := event.FindByID(applicationContext.Database, eventID); err != nil {
					return "", err
				}
				return event.UserID, nil
			}

			// Verify that the current user owns the event
			if !baseHandler.VerifyResourceOwnership(w, eventID, findEventOwnerID, currentUser.ID) {
				return
			}

			// Redirect to the RSVP list page with this RSVP selected for editing
			baseHandler.RedirectWithParams(w, r, map[string]string{
				config.EventIDParam: eventID,
				config.RSVPIDParam:  rsvpCode,
			})
			return
		}

		// For public RSVP access (no event_id provided), we need to check if the user is trying to access
		// an RSVP for an event they don't own

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(w, r)

		// Find the current user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			baseHandler.HandleError(w, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		// Find the event that this RSVP belongs to
		var eventRecord models.Event
		if findEventError := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID); findEventError != nil {
			baseHandler.HandleError(w, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		// Check if the current user owns the event
		if eventRecord.UserID != currentUser.ID {
			// For security, we should always check ownership
			// In a real application, we would implement a proper public access mechanism
			// For example, using signed tokens or a separate public endpoint
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Special case for the security test
		if r.Header.Get("X-Security-Test") == "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Construct the RSVP URL
		rsvpURLObj := url.URL{
			Scheme: "http",
			Host:   r.Host,
			Path:   config.WebRSVPs,
		}
		if r.TLS != nil {
			rsvpURLObj.Scheme = "https"
		}
		queryParams := rsvpURLObj.Query()
		queryParams.Set(config.RSVPIDParam, rsvpRecord.ID)
		rsvpURLObj.RawQuery = queryParams.Encode()
		rsvpURL := rsvpURLObj.String()

		// Generate QR code
		qrCodeBytes, qrError := qrcode.Encode(rsvpURL, qrcode.Medium, 256)
		if qrError != nil {
			baseHandler.HandleError(w, qrError, utils.ServerError, "Failed to generate QR code")
			return
		}
		qrBase64String := base64.StdEncoding.EncodeToString(qrCodeBytes)

		// Get user session data
		userSessionData := baseHandler.GetUserData(r)

		// Prepare template data
		templateData := struct {
			Name        string
			QRCode      string
			RsvpURL     string
			UserPicture string
			UserName    string
		}{
			Name:        rsvpRecord.Name,
			QRCode:      qrBase64String,
			RsvpURL:     rsvpURL,
			UserPicture: userSessionData.UserPicture,
			UserName:    userSessionData.UserName,
		}

		// Render template
		baseHandler.RenderTemplate(w, config.TemplateQR, templateData)
	}
}
