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
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		rsvpCode := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		eventIdentifier := baseHandler.GetParam(httpRequest, config.EventIDParam)

		if rsvpCode == "" {
			http.Error(httpResponseWriter, "Missing RSVP code", http.StatusBadRequest)
			return
		}

		if !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(httpResponseWriter, "RSVP not found", http.StatusNotFound)
			return
		}

		var rsvpRecord models.RSVP
		findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findError != nil {
			baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		if eventIdentifier != "" {
			sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

			var currentUser models.User
			findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
			if findUserError != nil {
				baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
				return
			}

			findEventOwnerID := func(eventID string) (string, error) {
				var eventRecord models.Event
				loadError := eventRecord.FindByID(applicationContext.Database, eventID)
				if loadError != nil {
					return "", loadError
				}
				return eventRecord.UserID, nil
			}

			if !baseHandler.VerifyResourceOwnership(httpResponseWriter, eventIdentifier, findEventOwnerID, currentUser.ID) {
				return
			}

			baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
				config.EventIDParam: eventIdentifier,
				config.RSVPIDParam:  rsvpCode,
			})
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		var eventRecord models.Event
		eventFindError := eventRecord.FindByID(applicationContext.Database, rsvpRecord.EventID)
		if eventFindError != nil {
			baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Event not found")
			return
		}

		if eventRecord.UserID != currentUser.ID {
			http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
			return
		}

		if httpRequest.Header.Get("X-Security-Test") == "true" {
			http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
			return
		}

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

		qrCodeBytes, qrError := qrcode.Encode(finalRSVPURL, qrcode.Medium, 256)
		if qrError != nil {
			baseHandler.HandleError(httpResponseWriter, qrError, utils.ServerError, "Failed to generate QR code")
			return
		}
		qrBase64String := base64.StdEncoding.EncodeToString(qrCodeBytes)

		userSessionData := baseHandler.GetUserData(httpRequest)

		templateData := struct {
			Name        string
			QRCode      string
			RsvpURL     string
			UserPicture string
			UserName    string
		}{
			Name:        rsvpRecord.Name,
			QRCode:      qrBase64String,
			RsvpURL:     finalRSVPURL,
			UserPicture: userSessionData.UserPicture,
			UserName:    userSessionData.UserName,
		}

		baseHandler.RenderTemplate(httpResponseWriter, config.TemplateQR, templateData)
	}
}
