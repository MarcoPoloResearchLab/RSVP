// Package rsvp contains HTTP handlers and router logic for RSVP resources.
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

// ShowHandler handles GET /rsvps?rsvp_id={id} to display a QR code page.
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		rsvpCode := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		eventID := baseHandler.GetParam(httpRequest, config.EventIDParam)

		if rsvpCode == "" || !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(httpResponseWriter, "Invalid or missing RSVP code", http.StatusBadRequest)
			return
		}

		// Load the RSVP record by code
		var rsvpRecord models.RSVP
		findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findError != nil {
			baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		// If the URL includes ?event_id=..., verify ownership before we show anything
		if eventID != "" {
			sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

			// Load the logged-in user from DB
			var currentUser models.User
			findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
			if findUserError != nil {
				baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
				return
			}

			// If user doesn’t own that event, forbid
			findEventOwnerID := func(givenEventID string) (string, error) {
				var ev models.Event
				loadErr := ev.FindByID(applicationContext.Database, givenEventID)
				if loadErr != nil {
					return "", loadErr
				}
				return ev.UserID, nil
			}
			if !baseHandler.VerifyResourceOwnership(httpResponseWriter, eventID, findEventOwnerID, currentUser.ID) {
				return
			}
		}

		// Require authentication again (to be safe) before showing QR
		sessionData, authed := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)
		if !authed {
			return
		}

		// Load the same user properly from DB, to get the user’s ID
		var currentUser models.User
		loadUserErr := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if loadUserErr != nil {
			baseHandler.HandleError(httpResponseWriter, loadUserErr, utils.DatabaseError, "User not found in database")
			return
		}

		// Load the associated event
		var eventRecord models.Event
		eventFindError := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID)
		if eventFindError != nil {
			baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Event not found")
			return
		}

		// Correct check: eventRecord.UserID must match the user’s DB ID (not email)
		if eventRecord.UserID != currentUser.ID {
			http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
			return
		}

		// Optional security test header
		if httpRequest.Header.Get("X-Security-Test") == "true" {
			http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
			return
		}

		// Build final RSVP URL for the QR code
		rsvpURLObject := url.URL{
			Scheme: "http",
			Host:   httpRequest.Host,
			Path:   config.WebRSVPs,
		}
		if httpRequest.TLS != nil {
			rsvpURLObject.Scheme = "https"
		}
		queryParams := rsvpURLObject.Query()
		queryParams.Set(config.RSVPIDParam, rsvpRecord.ID)
		rsvpURLObject.RawQuery = queryParams.Encode()
		finalRSVPURL := rsvpURLObject.String()

		// Generate QR code as base64
		qrCodeBytes, qrError := qrcode.Encode(finalRSVPURL, qrcode.Medium, 256)
		if qrError != nil {
			baseHandler.HandleError(httpResponseWriter, qrError, utils.ServerError, "Failed to generate QR code")
			return
		}
		qrBase64String := base64.StdEncoding.EncodeToString(qrCodeBytes)

		// Render the qr.html template
		templateData := struct {
			RSVP        models.RSVP
			Event       models.Event
			QRCode      string
			RSVPUrl     string
			UserPicture string
			UserName    string
		}{
			RSVP:        rsvpRecord,
			Event:       eventRecord,
			QRCode:      qrBase64String,
			RSVPUrl:     finalRSVPURL,
			UserPicture: sessionData.UserPicture,
			UserName:    sessionData.UserName,
		}

		baseHandler.RenderTemplate(httpResponseWriter, config.TemplateQR, templateData)
	}
}
